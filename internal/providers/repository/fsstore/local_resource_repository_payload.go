package fsstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type payloadFileInfo struct {
	Path                 string
	Name                 string
	PayloadType          string
	Extension            string
	PreserveExistingName bool
}

func firstMetadataBaseDir(values []string) string {
	if len(values) == 0 {
		return ""
	}
	trimmed := strings.TrimSpace(values[0])
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func (r *LocalResourceRepository) defaultPayloadType() string {
	return resource.NormalizePayloadType(r.resourceFormat)
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
	extension := filepath.Ext(name)
	payloadType, found := resource.PayloadTypeForExtension(extension)
	info := &payloadFileInfo{
		Path:      filepath.Join(dirPath, name),
		Name:      name,
		Extension: extension,
	}
	if found {
		info.PayloadType = payloadType
		return info, nil
	}

	metadataPayloadType, found, err := r.metadataPayloadType(logicalPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, faults.NewValidationError(
			fmt.Sprintf("resource %q uses unsupported payload file extension %q without metadata payloadType", logicalPath, extension),
			nil,
		)
	}

	info.PayloadType = metadataPayloadType
	info.PreserveExistingName = true
	return info, nil
}

func (r *LocalResourceRepository) metadataPayloadType(logicalPath string) (string, bool, error) {
	if strings.TrimSpace(r.metadataBaseDir) == "" {
		return "", false, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", false, err
	}
	if normalizedPath == "/" {
		return "", false, nil
	}

	relative := strings.TrimPrefix(normalizedPath, "/")
	basePath := filepath.Join(r.metadataBaseDir, filepath.FromSlash(relative))
	candidates := []struct {
		path string
		yaml bool
	}{
		{path: filepath.Join(basePath, "metadata.yaml"), yaml: true},
		{path: filepath.Join(basePath, "metadata.json"), yaml: false},
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, internalError("failed to read metadata payload type", err)
		}

		var metadataValue metadatadomain.ResourceMetadata
		if candidate.yaml {
			metadataValue, err = metadatadomain.DecodeResourceMetadataYAML(data)
		} else {
			metadataValue, err = metadatadomain.DecodeResourceMetadataJSON(data)
		}
		if err != nil {
			return "", false, internalError("failed to decode metadata payload type", err)
		}
		if strings.TrimSpace(metadataValue.PayloadType) == "" {
			return "", false, nil
		}

		payloadType, err := metadatadomain.ValidateResourceFormat(metadataValue.PayloadType)
		if err != nil {
			return "", false, err
		}
		return payloadType, true, nil
	}

	return "", false, nil
}

func (r *LocalResourceRepository) resolvePayloadTarget(
	logicalPath string,
	value resource.Value,
) (payloadFileInfo, *payloadFileInfo, error) {
	existing, err := r.discoverPayloadFile(logicalPath)
	if err != nil {
		return payloadFileInfo{}, nil, err
	}

	desiredPayloadType, preserveExistingName, err := r.desiredPayloadType(logicalPath, value, existing)
	if err != nil {
		return payloadFileInfo{}, existing, err
	}

	if existing != nil && existing.PreserveExistingName && preserveExistingName {
		return *existing, existing, nil
	}

	canonicalPath, err := r.canonicalPayloadFilePath(logicalPath, desiredPayloadType)
	if err != nil {
		return payloadFileInfo{}, existing, err
	}
	extension, err := resource.PayloadExtension(desiredPayloadType)
	if err != nil {
		return payloadFileInfo{}, existing, err
	}

	target := payloadFileInfo{
		Path:        canonicalPath,
		Name:        "resource" + extension,
		PayloadType: desiredPayloadType,
		Extension:   extension,
	}

	if existing != nil && existing.Path == target.Path {
		target.PreserveExistingName = existing.PreserveExistingName
	}
	return target, existing, nil
}

func (r *LocalResourceRepository) desiredPayloadType(
	logicalPath string,
	value resource.Value,
	existing *payloadFileInfo,
) (string, bool, error) {
	if payloadType, found, err := r.metadataPayloadType(logicalPath); err != nil {
		return "", false, err
	} else if found {
		return payloadType, existing != nil && existing.PreserveExistingName, nil
	}

	if existing != nil {
		return existing.PayloadType, existing.PreserveExistingName, nil
	}

	if resource.IsBinaryValue(value) {
		return resource.PayloadTypeOctetStream, false, nil
	}

	payloadType, err := resource.ValidatePayloadType(r.defaultPayloadType())
	if err != nil {
		return "", false, err
	}
	return payloadType, false, nil
}
