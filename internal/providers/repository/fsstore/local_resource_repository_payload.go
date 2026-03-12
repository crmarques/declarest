package fsstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

type payloadFileInfo struct {
	Path       string
	Name       string
	Descriptor resource.PayloadDescriptor
}

type resourcePayloadFiles struct {
	Resource *payloadFileInfo
	Defaults *payloadFileInfo
}

func (f resourcePayloadFiles) primary() *payloadFileInfo {
	if f.Resource != nil {
		return f.Resource
	}
	return f.Defaults
}

func firstMetadataBaseDir(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		return filepath.Clean(trimmed)
	}
	return ""
}

func (r *LocalResourceRepository) discoverPayloadFiles(logicalPath string) (resourcePayloadFiles, error) {
	resourceDir, err := r.collectionDirPath(logicalPath)
	if err != nil {
		return resourcePayloadFiles{}, err
	}
	return r.payloadFilesInfoFromDir(logicalPath, resourceDir)
}

func (r *LocalResourceRepository) payloadFilesInfoFromDir(logicalPath string, dirPath string) (resourcePayloadFiles, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resourcePayloadFiles{}, nil
		}
		return resourcePayloadFiles{}, internalError("failed to inspect resource directory", err)
	}

	resourceCandidates := make([]string, 0, 1)
	defaultCandidates := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasPrefix(entry.Name(), "resource."):
			resourceCandidates = append(resourceCandidates, entry.Name())
		case strings.HasPrefix(entry.Name(), "defaults."):
			defaultCandidates = append(defaultCandidates, entry.Name())
		}
	}

	resourceInfo, err := payloadFileInfoFromCandidates(logicalPath, dirPath, "resource", resourceCandidates)
	if err != nil {
		return resourcePayloadFiles{}, err
	}
	defaultsInfo, err := payloadFileInfoFromCandidates(logicalPath, dirPath, "defaults", defaultCandidates)
	if err != nil {
		return resourcePayloadFiles{}, err
	}

	files := resourcePayloadFiles{
		Resource: resourceInfo,
		Defaults: defaultsInfo,
	}
	if err := validateDefaultsPayloadFiles(logicalPath, files); err != nil {
		return resourcePayloadFiles{}, err
	}
	return files, nil
}

func (r *LocalResourceRepository) resolvePayloadTarget(
	logicalPath string,
	content resource.Content,
) (payloadFileInfo, resourcePayloadFiles, error) {
	files, err := r.discoverPayloadFiles(logicalPath)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	desired := desiredPayloadDescriptor(content, files)
	if err := validateDesiredDescriptorWithDefaults(files, desired); err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}
	canonicalPath, err := r.canonicalPayloadFilePath(logicalPath, desired.Extension)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	target := payloadFileInfo{
		Path:       canonicalPath,
		Name:       "resource" + desired.Extension,
		Descriptor: desired,
	}
	return target, files, nil
}

func (r *LocalResourceRepository) resolveDefaultsTarget(
	logicalPath string,
	content resource.Content,
) (payloadFileInfo, resourcePayloadFiles, error) {
	files, err := r.discoverPayloadFiles(logicalPath)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	desired := desiredDefaultsPayloadDescriptor(content, files)
	if !resource.SupportsDefaultsOverlayPayloadType(desired.PayloadType) {
		return payloadFileInfo{}, resourcePayloadFiles{}, faults.NewValidationError(
			fmt.Sprintf(
				"defaults sidecar requires merge-capable payload type (json, yaml, ini, properties); got %q",
				desired.PayloadType,
			),
			nil,
		)
	}
	if err := validateDesiredDescriptorWithDefaults(files, desired); err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	canonicalPath, err := r.collectionDirPath(logicalPath)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	target := payloadFileInfo{
		Path:       filepath.Join(canonicalPath, "defaults"+desired.Extension),
		Name:       "defaults" + desired.Extension,
		Descriptor: desired,
	}
	return target, files, nil
}

func desiredPayloadDescriptor(content resource.Content, existing resourcePayloadFiles) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return resource.NormalizePayloadDescriptor(content.Descriptor)
	}
	if existing.Resource != nil {
		return existing.Resource.Descriptor
	}
	if existing.Defaults != nil {
		return existing.Defaults.Descriptor
	}
	if resource.IsBinaryValue(content.Value) {
		return resource.DefaultOctetStreamDescriptor()
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
}

func desiredDefaultsPayloadDescriptor(content resource.Content, existing resourcePayloadFiles) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return resource.NormalizePayloadDescriptor(content.Descriptor)
	}
	if existing.Defaults != nil {
		return existing.Defaults.Descriptor
	}
	if existing.Resource != nil {
		return existing.Resource.Descriptor
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
}

func payloadFileInfoFromCandidates(
	logicalPath string,
	dirPath string,
	baseName string,
	candidates []string,
) (*payloadFileInfo, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Strings(candidates)
	if len(candidates) > 1 {
		label := "payload"
		if baseName == "defaults" {
			label = "defaults"
		}
		return nil, faults.NewConflictError(
			fmt.Sprintf("resource %q has multiple %s files: %s", logicalPath, label, strings.Join(candidates, ", ")),
			nil,
		)
	}

	name := candidates[0]
	return &payloadFileInfo{
		Path: filepath.Join(dirPath, name),
		Name: name,
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			Extension: filepath.Ext(name),
		}),
	}, nil
}

func validateDefaultsPayloadFiles(logicalPath string, files resourcePayloadFiles) error {
	if files.Defaults == nil {
		return nil
	}

	overrides := resource.PayloadDescriptor{}
	if files.Resource != nil {
		overrides = files.Resource.Descriptor
	}
	if err := resource.ValidateDefaultsSidecarDescriptor(files.Defaults.Descriptor, overrides); err != nil {
		return faults.NewValidationError(fmt.Sprintf("resource %q defaults sidecar is invalid", logicalPath), err)
	}

	return nil
}

func validateDesiredDescriptorWithDefaults(files resourcePayloadFiles, desired resource.PayloadDescriptor) error {
	if files.Defaults == nil {
		if files.Resource != nil {
			return resource.ValidateDefaultsSidecarDescriptor(desired, files.Resource.Descriptor)
		}
		return nil
	}
	if err := resource.ValidateDefaultsSidecarDescriptor(files.Defaults.Descriptor, desired); err != nil {
		return err
	}
	return nil
}
