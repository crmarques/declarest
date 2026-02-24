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
	"github.com/crmarques/declarest/resource"
	"go.yaml.in/yaml/v3"
)

func (r *LocalResourceRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return validationError("logical path must target a resource, not root", nil)
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

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return internalError("failed to create resource directory", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".declarest-tmp-*")
	if err != nil {
		return internalError("failed to create temporary file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write temporary payload", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to finalize temporary payload", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace payload file", err)
	}

	legacyPath, err := r.legacyPayloadFilePath(normalizedPath)
	if err == nil && legacyPath != targetPath {
		if _, statErr := os.Stat(legacyPath); statErr == nil {
			if removeErr := os.Remove(legacyPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return internalError("failed to remove legacy payload file", removeErr)
			}
			_ = r.cleanupEmptyParents(filepath.Dir(legacyPath))
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return internalError("failed to inspect legacy payload file", statErr)
		}
	}

	return nil
}

func (r *LocalResourceRepository) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}
	if normalizedPath == "/" {
		return nil, validationError("logical path must target a resource, not root", nil)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacyPayloadPath, pathErr := r.legacyPayloadFilePath(normalizedPath)
			if pathErr != nil {
				return nil, pathErr
			}
			data, err = os.ReadFile(legacyPayloadPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
				}
				return nil, internalError("failed to read resource payload", err)
			}
		} else {
			return nil, internalError("failed to read resource payload", err)
		}
	}

	decoded, err := r.decodePayload(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
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
			return nil, validationError("invalid yaml payload", err)
		}
		return resource.Normalize(decoded)
	case config.ResourceFormatJSON:
		fallthrough
	default:
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()

		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, validationError("invalid json payload", err)
		}
		return resource.Normalize(decoded)
	}
}
