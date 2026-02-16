package fsmetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/crmarques/declarest/config"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func (s *FSMetadataService) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	metadata, found, err := s.tryReadMetadata(selector, kind)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	if !found {
		return metadatadomain.ResourceMetadata{}, notFoundError(fmt.Sprintf("metadata %q not found", logicalPath))
	}
	return metadata, nil
}

func (s *FSMetadataService) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	if err := validateResourceMetadata(metadata); err != nil {
		return err
	}

	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		return err
	}

	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		return err
	}

	return s.writeMetadataFile(targetPath, metadata)
}

func (s *FSMetadataService) Unset(_ context.Context, logicalPath string) error {
	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		return err
	}

	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		return err
	}

	if err := os.Remove(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to remove metadata file", err)
	}

	_ = cleanupEmptyParents(filepath.Dir(targetPath), s.baseDir)
	return nil
}

func (s *FSMetadataService) tryReadMetadata(selector string, kind metadataPathKind) (metadatadomain.ResourceMetadata, bool, error) {
	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, false, err
	}

	item, err := s.readMetadataFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return metadatadomain.ResourceMetadata{}, false, nil
		}
		return metadatadomain.ResourceMetadata{}, false, err
	}
	return item, true, nil
}

func (s *FSMetadataService) readMetadataFile(targetPath string) (metadatadomain.ResourceMetadata, error) {
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	item, err := s.decodeMetadata(data)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	return item, nil
}

func (s *FSMetadataService) writeMetadataFile(targetPath string, metadata metadatadomain.ResourceMetadata) error {
	encoded, err := s.encodeMetadata(metadata)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return internalError("failed to create metadata directory", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".declarest-meta-*")
	if err != nil {
		return internalError("failed to create temporary metadata file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write temporary metadata file", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to close temporary metadata file", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace metadata file", err)
	}

	return nil
}

func (s *FSMetadataService) decodeMetadata(data []byte) (metadatadomain.ResourceMetadata, error) {
	decoded := metadatadomain.ResourceMetadata{}

	switch s.resourceFormat {
	case config.ResourceFormatYAML:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&decoded); err != nil {
			return metadatadomain.ResourceMetadata{}, validationError("invalid yaml metadata", err)
		}
	default:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil {
			return metadatadomain.ResourceMetadata{}, validationError("invalid json metadata", err)
		}
	}

	if err := validateResourceMetadata(decoded); err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	return decoded, nil
}

func (s *FSMetadataService) encodeMetadata(metadata metadatadomain.ResourceMetadata) ([]byte, error) {
	if err := validateResourceMetadata(metadata); err != nil {
		return nil, err
	}

	switch s.resourceFormat {
	case config.ResourceFormatYAML:
		encoded, err := yaml.Marshal(metadata)
		if err != nil {
			return nil, internalError("failed to encode yaml metadata", err)
		}
		return encoded, nil
	default:
		encoded, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return nil, internalError("failed to encode json metadata", err)
		}
		return encoded, nil
	}
}
