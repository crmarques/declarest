package fsstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) Save(_ context.Context, logicalPath string, content resource.Content) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	targetInfo, existingFiles, err := r.resolvePayloadTarget(normalizedPath, content)
	if err != nil {
		return err
	}

	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return err
	}
	if existingFiles.Defaults != nil {
		defaultsContent, defaultsErr := r.readPayloadFile(existingFiles.Defaults)
		if defaultsErr != nil {
			return defaultsErr
		}
		if err := resource.ValidateDefaultsSidecarValue(defaultsContent.Value); err != nil {
			return err
		}
		normalizedValue, err = resource.CompactAgainstDefaults(normalizedValue, defaultsContent.Value)
		if err != nil {
			return err
		}
	}
	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	if content.Value == nil && existingFiles.Defaults != nil {
		return r.removePayloadFile(existingFiles.Resource)
	}

	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	if err := r.writeFileAtomically(targetInfo.Path, encoded, ".declarest-tmp-*", "resource"); err != nil {
		return err
	}
	if existingFiles.Resource != nil && existingFiles.Resource.Path != targetInfo.Path {
		if err := r.removePayloadFile(existingFiles.Resource); err != nil {
			return err
		}
	}
	return nil
}

func (r *LocalResourceRepository) GetDefaults(_ context.Context, logicalPath string) (resource.Content, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, err
	}
	if normalizedPath == "/" {
		return resource.Content{}, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	files, err := r.discoverPayloadFiles(normalizedPath)
	if err != nil {
		return resource.Content{}, err
	}
	if files.Defaults == nil {
		return resource.Content{}, notFoundError(fmt.Sprintf("resource defaults %q not found", normalizedPath))
	}

	content, err := r.readPayloadFile(files.Defaults)
	if err != nil {
		return resource.Content{}, err
	}
	if err := resource.ValidateDefaultsSidecarValue(content.Value); err != nil {
		return resource.Content{}, err
	}
	return content, nil
}

func (r *LocalResourceRepository) SaveDefaults(_ context.Context, logicalPath string, content resource.Content) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	targetInfo, existingFiles, err := r.resolveDefaultsTarget(normalizedPath, content)
	if err != nil {
		return err
	}

	normalizedValue := content.Value
	if normalizedValue != nil {
		normalizedValue, err = resource.Normalize(content.Value)
		if err != nil {
			return err
		}
		if err := resource.ValidateDefaultsSidecarValue(normalizedValue); err != nil {
			return err
		}
	}

	if defaultsPayloadIsEmpty(normalizedValue) {
		return r.removePayloadFile(existingFiles.Defaults)
	}

	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return internalError("failed to encode defaults payload", err)
	}

	if err := r.writeFileAtomically(targetInfo.Path, encoded, ".declarest-defaults-*", "defaults"); err != nil {
		return err
	}
	if existingFiles.Defaults != nil && existingFiles.Defaults.Path != targetInfo.Path {
		if err := r.removePayloadFile(existingFiles.Defaults); err != nil {
			return err
		}
	}
	return nil
}

func (r *LocalResourceRepository) SaveResourceWithArtifacts(
	_ context.Context,
	logicalPath string,
	content resource.Content,
	artifacts []repository.ResourceArtifact,
) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	targetInfo, existingFiles, err := r.resolvePayloadTarget(normalizedPath, content)
	if err != nil {
		return err
	}

	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return err
	}
	if existingFiles.Defaults != nil {
		defaultsContent, defaultsErr := r.readPayloadFile(existingFiles.Defaults)
		if defaultsErr != nil {
			return defaultsErr
		}
		if err := resource.ValidateDefaultsSidecarValue(defaultsContent.Value); err != nil {
			return err
		}
		normalizedValue, err = resource.CompactAgainstDefaults(normalizedValue, defaultsContent.Value)
		if err != nil {
			return err
		}
	}
	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	for idx := range artifacts {
		if err := validateReservedSidecarArtifactName(artifacts[idx].File); err != nil {
			return err
		}
		artifactPath, err := r.resourceArtifactFilePath(normalizedPath, artifacts[idx].File)
		if err != nil {
			return err
		}
		if artifactPath == targetInfo.Path {
			return faults.NewValidationError(
				fmt.Sprintf("resource artifact %q conflicts with the canonical resource payload file", artifacts[idx].File),
				nil,
			)
		}
		if existingFiles.Defaults != nil && artifactPath == existingFiles.Defaults.Path {
			return faults.NewValidationError(
				fmt.Sprintf("resource artifact %q conflicts with the defaults sidecar file", artifacts[idx].File),
				nil,
			)
		}
		if err := r.writeFileAtomically(artifactPath, artifacts[idx].Content, ".declarest-artifact-*", "resource artifact"); err != nil {
			return err
		}
	}

	if content.Value == nil && existingFiles.Defaults != nil {
		return r.removePayloadFile(existingFiles.Resource)
	}

	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	if err := r.writeFileAtomically(targetInfo.Path, encoded, ".declarest-tmp-*", "resource"); err != nil {
		return err
	}
	if existingFiles.Resource != nil && existingFiles.Resource.Path != targetInfo.Path {
		if err := r.removePayloadFile(existingFiles.Resource); err != nil {
			return err
		}
	}
	return nil
}

func defaultsPayloadIsEmpty(value resource.Value) bool {
	if value == nil {
		return true
	}
	objectValue, ok := value.(map[string]any)
	return ok && len(objectValue) == 0
}

func validateReservedSidecarArtifactName(file string) error {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(file)))
	switch {
	case strings.HasPrefix(base, "resource."):
		return faults.NewValidationError("resource artifacts cannot use the reserved prefix \"resource.\"", nil)
	case strings.HasPrefix(base, "defaults."):
		return faults.NewValidationError("resource artifacts cannot use the reserved prefix \"defaults.\"", nil)
	default:
		return nil
	}
}

func (r *LocalResourceRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, err
	}
	if normalizedPath == "/" {
		return resource.Content{}, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	files, err := r.discoverPayloadFiles(normalizedPath)
	if err != nil {
		return resource.Content{}, err
	}
	if files.Resource == nil && files.Defaults == nil {
		return resource.Content{}, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
	}

	var defaultsValue resource.Content
	if files.Defaults != nil {
		defaultsValue, err = r.readPayloadFile(files.Defaults)
		if err != nil {
			return resource.Content{}, err
		}
		if err := resource.ValidateDefaultsSidecarValue(defaultsValue.Value); err != nil {
			return resource.Content{}, err
		}
	}

	var overrideValue resource.Content
	if files.Resource != nil {
		overrideValue, err = r.readPayloadFile(files.Resource)
		if err != nil {
			return resource.Content{}, err
		}
	}

	mergedValue, err := resource.MergeWithDefaults(defaultsValue.Value, overrideValue.Value)
	if err != nil {
		return resource.Content{}, err
	}

	descriptor := resource.PayloadDescriptor{}
	if primary := files.primary(); primary != nil {
		descriptor = primary.Descriptor
	}
	return resource.Content{
		Value:      mergedValue,
		Descriptor: descriptor,
	}, nil
}

func (r *LocalResourceRepository) ReadResourceArtifact(_ context.Context, logicalPath string, file string) ([]byte, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}
	if normalizedPath == "/" {
		return nil, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	targetPath, err := r.resourceArtifactFilePath(normalizedPath, file)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, notFoundError(fmt.Sprintf("resource artifact %q not found for %q", file, normalizedPath))
		}
		return nil, internalError("failed to read resource artifact", err)
	}

	return data, nil
}

func (r *LocalResourceRepository) writeFileAtomically(
	targetPath string,
	data []byte,
	tempPattern string,
	kind string,
) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return internalError(fmt.Sprintf("failed to create %s directory", kind), err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), tempPattern)
	if err != nil {
		return internalError(fmt.Sprintf("failed to create temporary %s file", kind), err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError(fmt.Sprintf("failed to write temporary %s", kind), err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError(fmt.Sprintf("failed to finalize temporary %s", kind), err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError(fmt.Sprintf("failed to replace %s file", kind), err)
	}

	return nil
}

func (r *LocalResourceRepository) readPayloadFile(info *payloadFileInfo) (resource.Content, error) {
	if info == nil {
		return resource.Content{}, nil
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resource.Content{}, notFoundError(fmt.Sprintf("resource payload %q not found", info.Path))
		}
		return resource.Content{}, internalError("failed to read resource payload", err)
	}

	decoded, err := resource.DecodeContent(data, info.Descriptor)
	if err != nil {
		return resource.Content{}, err
	}
	return decoded, nil
}

func (r *LocalResourceRepository) removePayloadFile(info *payloadFileInfo) error {
	if info == nil {
		return nil
	}
	if err := os.Remove(info.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return internalError("failed to remove resource payload", err)
	}
	_ = r.cleanupEmptyParents(filepath.Dir(info.Path))
	return nil
}
