// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/providers/fsutil"
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
	content.Value = normalizedValue
	content.Descriptor = targetInfo.Descriptor

	if content.Value == nil {
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
		if err := r.writeFileAtomically(artifactPath, artifacts[idx].Content, ".declarest-artifact-*", "resource artifact"); err != nil {
			return err
		}
	}

	if content.Value == nil {
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

func validateReservedSidecarArtifactName(file string) error {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(file)))
	switch {
	case strings.HasPrefix(base, "resource."):
		return faults.NewValidationError("resource artifacts cannot use the reserved prefix \"resource.\"", nil)
	case strings.HasPrefix(base, "defaults"):
		return faults.NewValidationError("resource artifacts cannot use the reserved prefix \"defaults\"", nil)
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
	if files.Resource == nil {
		return resource.Content{}, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
	}

	overrideValue, err := r.readPayloadFile(files.Resource)
	if err != nil {
		return resource.Content{}, err
	}

	descriptor := resource.PayloadDescriptor{}
	if primary := files.primary(); primary != nil {
		descriptor = primary.Descriptor
	}
	return resource.Content{
		Value:      overrideValue.Value,
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
	rootDir := r.baseDir
	if info.Shared {
		rootDir = r.defaultsBaseDir()
	}
	_ = fsutil.CleanupEmptyParents(filepath.Dir(info.Path), rootDir)
	return nil
}
