package fsstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

	targetInfo, existingInfo, err := r.resolvePayloadTarget(normalizedPath, content)
	if err != nil {
		return err
	}

	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return err
	}
	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	if err := r.writeFileAtomically(targetInfo.Path, encoded, ".declarest-tmp-*", "resource"); err != nil {
		return err
	}
	if existingInfo != nil && existingInfo.Path != targetInfo.Path {
		if err := os.Remove(existingInfo.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return internalError("failed to remove superseded resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(existingInfo.Path))
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

	targetInfo, existingInfo, err := r.resolvePayloadTarget(normalizedPath, content)
	if err != nil {
		return err
	}

	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return err
	}
	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	for idx := range artifacts {
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
		if err := r.writeFileAtomically(artifactPath, artifacts[idx].Content, ".declarest-artifact-*", "resource artifact"); err != nil {
			return err
		}
	}

	if err := r.writeFileAtomically(targetInfo.Path, encoded, ".declarest-tmp-*", "resource"); err != nil {
		return err
	}
	if existingInfo != nil && existingInfo.Path != targetInfo.Path {
		if err := os.Remove(existingInfo.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return internalError("failed to remove superseded resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(existingInfo.Path))
	}
	return nil
}

func (r *LocalResourceRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, err
	}
	if normalizedPath == "/" {
		return resource.Content{}, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	info, err := r.discoverPayloadFile(normalizedPath)
	if err != nil {
		return resource.Content{}, err
	}
	if info == nil {
		return resource.Content{}, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
	}

	data, err := os.ReadFile(info.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resource.Content{}, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
		}
		return resource.Content{}, internalError("failed to read resource payload", err)
	}

	decoded, err := resource.DecodeContent(data, info.Descriptor)
	if err != nil {
		return resource.Content{}, err
	}
	return decoded, nil
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
