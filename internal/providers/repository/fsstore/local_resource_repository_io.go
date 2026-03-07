package fsstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"go.yaml.in/yaml/v3"
)

func (r *LocalResourceRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return err
	}

	encoded, err := r.encodePayload(normalizedValue)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return err
	}

	return r.writeFileAtomically(targetPath, encoded, ".declarest-tmp-*", "resource")
}

func (r *LocalResourceRepository) SaveResourceWithArtifacts(
	_ context.Context,
	logicalPath string,
	value resource.Value,
	artifacts []repository.ResourceArtifact,
) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return err
	}

	encoded, err := r.encodePayload(normalizedValue)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return err
	}

	for idx := range artifacts {
		artifactPath, err := r.resourceArtifactFilePath(normalizedPath, artifacts[idx].File)
		if err != nil {
			return err
		}
		if artifactPath == targetPath {
			return faults.NewValidationError(
				fmt.Sprintf("resource artifact %q conflicts with the canonical resource payload file", artifacts[idx].File),
				nil,
			)
		}
		if err := r.writeFileAtomically(artifactPath, artifacts[idx].Content, ".declarest-artifact-*", "resource artifact"); err != nil {
			return err
		}
	}

	return r.writeFileAtomically(targetPath, encoded, ".declarest-tmp-*", "resource")
}

func (r *LocalResourceRepository) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}
	if normalizedPath == "/" {
		return nil, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
		}
		return nil, internalError("failed to read resource payload", err)
	}

	decoded, err := r.decodePayload(data)
	if err != nil {
		return nil, err
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

func (r *LocalResourceRepository) encodePayload(value resource.Value) ([]byte, error) {
	switch r.resourceFormat {
	case config.ResourceFormatYAML:
		return yaml.Marshal(value)
	case config.ResourceFormatJSON:
		fallthrough
	default:
		return json.MarshalIndent(value, "", "  ")
	}
}

func (r *LocalResourceRepository) decodePayload(data []byte) (resource.Value, error) {
	switch r.resourceFormat {
	case config.ResourceFormatYAML:
		var decoded any
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			return nil, faults.NewValidationError("invalid yaml payload", err)
		}
		return resource.Normalize(decoded)
	case config.ResourceFormatJSON:
		fallthrough
	default:
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()

		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, faults.NewValidationError("invalid json payload", err)
		}
		return resource.Normalize(decoded)
	}
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
