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

func (r *LocalResourceRepository) discoverPayloadFile(logicalPath string) (*payloadFileInfo, error) {
	resourceDir, err := r.collectionDirPath(logicalPath)
	if err != nil {
		return nil, err
	}
	return r.payloadFileInfoFromDir(logicalPath, resourceDir)
}

func (r *LocalResourceRepository) payloadFileInfoFromDir(logicalPath string, dirPath string) (*payloadFileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, internalError("failed to inspect resource directory", err)
	}

	candidates := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "resource.") {
			continue
		}
		candidates = append(candidates, entry.Name())
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Strings(candidates)
	if len(candidates) > 1 {
		return nil, faults.NewConflictError(
			fmt.Sprintf("resource %q has multiple payload files: %s", logicalPath, strings.Join(candidates, ", ")),
			nil,
		)
	}

	name := candidates[0]
	descriptor := resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		Extension: filepath.Ext(name),
	})
	return &payloadFileInfo{
		Path:       filepath.Join(dirPath, name),
		Name:       name,
		Descriptor: descriptor,
	}, nil
}

func (r *LocalResourceRepository) resolvePayloadTarget(
	logicalPath string,
	content resource.Content,
) (payloadFileInfo, *payloadFileInfo, error) {
	existing, err := r.discoverPayloadFile(logicalPath)
	if err != nil {
		return payloadFileInfo{}, nil, err
	}

	desired := desiredPayloadDescriptor(content, existing)
	canonicalPath, err := r.canonicalPayloadFilePath(logicalPath, desired.Extension)
	if err != nil {
		return payloadFileInfo{}, existing, err
	}

	target := payloadFileInfo{
		Path:       canonicalPath,
		Name:       "resource" + desired.Extension,
		Descriptor: desired,
	}
	return target, existing, nil
}

func desiredPayloadDescriptor(content resource.Content, existing *payloadFileInfo) resource.PayloadDescriptor {
	if contentDescriptorExplicit(content.Descriptor) {
		return resource.NormalizePayloadDescriptor(content.Descriptor)
	}
	if existing != nil {
		return existing.Descriptor
	}
	if resource.IsBinaryValue(content.Value) {
		return resource.DefaultOctetStreamDescriptor()
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
}

func contentDescriptorExplicit(descriptor resource.PayloadDescriptor) bool {
	return strings.TrimSpace(descriptor.PayloadType) != "" ||
		strings.TrimSpace(descriptor.MediaType) != "" ||
		strings.TrimSpace(descriptor.Extension) != ""
}
