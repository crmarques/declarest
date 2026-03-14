package fsmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var _ metadatadomain.DefaultsArtifactStore = (*FSMetadataService)(nil)

func (s *FSMetadataService) ReadDefaultsArtifact(
	_ context.Context,
	logicalPath string,
	file string,
) (resource.Content, error) {
	targetPath, descriptor, err := s.defaultsArtifactPath(logicalPath, file)
	if err != nil {
		return resource.Content{}, err
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resource.Content{}, notFoundError(fmt.Sprintf("resource defaults artifact %q not found", file))
		}
		return resource.Content{}, internalError("failed to read resource defaults artifact", err)
	}

	decoded, err := resource.DecodeContent(data, descriptor)
	if err != nil {
		return resource.Content{}, err
	}
	if err := resource.ValidateDefaultsSidecarValue(decoded.Value); err != nil {
		return resource.Content{}, err
	}
	return decoded, nil
}

func (s *FSMetadataService) WriteDefaultsArtifact(
	_ context.Context,
	logicalPath string,
	file string,
	content resource.Content,
) error {
	targetPath, descriptor, err := s.defaultsArtifactPath(logicalPath, file)
	if err != nil {
		return err
	}

	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return err
	}
	if err := resource.ValidateDefaultsSidecarValue(normalizedValue); err != nil {
		return err
	}

	encoded, err := resource.EncodeContentPretty(resource.Content{
		Value:      normalizedValue,
		Descriptor: descriptor,
	})
	if err != nil {
		return internalError("failed to encode resource defaults artifact", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return internalError("failed to create defaults artifact directory", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".declarest-defaults-*")
	if err != nil {
		return internalError("failed to create temporary defaults artifact", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write temporary defaults artifact", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to close temporary defaults artifact", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace defaults artifact", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(targetPath))
	for _, extension := range metadatadomain.SupportedDefaultsArtifactExtensions() {
		candidate := filepath.Join(filepath.Dir(targetPath), baseName+extension)
		if candidate == targetPath {
			continue
		}
		_ = os.Remove(candidate)
	}

	return nil
}

func (s *FSMetadataService) DeleteDefaultsArtifact(
	_ context.Context,
	logicalPath string,
	file string,
) error {
	targetPath, _, err := s.defaultsArtifactPath(logicalPath, file)
	if err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return internalError("failed to delete resource defaults artifact", err)
	}
	_ = cleanupEmptyParents(filepath.Dir(targetPath), s.baseDir)
	return nil
}

func (s *FSMetadataService) defaultsArtifactPath(
	logicalPath string,
	file string,
) (string, resource.PayloadDescriptor, error) {
	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		return "", resource.PayloadDescriptor{}, err
	}
	dirPath, err := s.metadataSelectorDirPath(selector, kind)
	if err != nil {
		return "", resource.PayloadDescriptor{}, err
	}

	trimmedFile := strings.TrimSpace(file)
	if trimmedFile == "" || filepath.Base(trimmedFile) != trimmedFile {
		return "", resource.PayloadDescriptor{}, faults.NewValidationError("resource defaults artifact file name is invalid", nil)
	}
	includeRef := metadatadomain.DefaultsIncludePlaceholder(trimmedFile)
	if strings.HasPrefix(trimmedFile, "defaults-") {
		profile := strings.TrimPrefix(strings.TrimSuffix(trimmedFile, filepath.Ext(trimmedFile)), "defaults-")
		if err := metadatadomain.ValidateDefaultsProfileName(profile); err != nil {
			return "", resource.PayloadDescriptor{}, err
		}
		if err := metadatadomain.ValidateDefaultsSpec(&metadatadomain.DefaultsSpec{
			Profiles: map[string]any{profile: includeRef},
		}); err != nil {
			return "", resource.PayloadDescriptor{}, err
		}
	} else {
		if err := metadatadomain.ValidateDefaultsSpec(&metadatadomain.DefaultsSpec{Value: includeRef}); err != nil {
			return "", resource.PayloadDescriptor{}, err
		}
	}

	descriptor, ok := resource.PayloadDescriptorForFileName(trimmedFile)
	if !ok || !metadatadomain.DefaultsSupportsFileBackedDescriptor(descriptor) {
		return "", resource.PayloadDescriptor{}, faults.NewValidationError(
			fmt.Sprintf("resource defaults artifact %q is not supported", trimmedFile),
			nil,
		)
	}

	targetPath := filepath.Join(dirPath, trimmedFile)
	if !isPathUnderRoot(s.baseDir, targetPath) {
		return "", resource.PayloadDescriptor{}, faults.NewValidationError("defaults artifact path escapes metadata base directory", nil)
	}
	return targetPath, descriptor, nil
}
