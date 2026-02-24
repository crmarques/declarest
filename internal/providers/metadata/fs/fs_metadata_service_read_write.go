package fsmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/debugctx"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func (s *FSMetadataService) Get(ctx context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	debugctx.Printf(ctx, "metadata fs get start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs get invalid logical_path=%q error=%v", logicalPath, err)
		return metadatadomain.ResourceMetadata{}, err
	}

	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs get resolve-path failed logical_path=%q selector=%q kind=%q error=%v",
			logicalPath,
			selector,
			metadataPathKindName(kind),
			err,
		)
		return metadatadomain.ResourceMetadata{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs get lookup logical_path=%q selector=%q kind=%q file=%q",
		logicalPath,
		selector,
		metadataPathKindName(kind),
		targetPath,
	)

	metadata, found, err := s.tryReadMetadata(selector, kind)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs get failed logical_path=%q file=%q error=%v", logicalPath, targetPath, err)
		return metadatadomain.ResourceMetadata{}, err
	}
	if !found {
		debugctx.Printf(ctx, "metadata fs get miss logical_path=%q file=%q", logicalPath, targetPath)
		return metadatadomain.ResourceMetadata{}, notFoundError(fmt.Sprintf("metadata %q not found", logicalPath))
	}
	debugctx.Printf(ctx, "metadata fs get hit logical_path=%q file=%q", logicalPath, targetPath)
	return metadata, nil
}

func (s *FSMetadataService) Set(ctx context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	debugctx.Printf(ctx, "metadata fs set start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	if err := validateResourceMetadata(metadata); err != nil {
		debugctx.Printf(ctx, "metadata fs set invalid logical_path=%q error=%v", logicalPath, err)
		return err
	}

	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs set invalid logical_path=%q error=%v", logicalPath, err)
		return err
	}

	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs set resolve-path failed logical_path=%q selector=%q kind=%q error=%v",
			logicalPath,
			selector,
			metadataPathKindName(kind),
			err,
		)
		return err
	}
	debugctx.Printf(
		ctx,
		"metadata fs set write logical_path=%q selector=%q kind=%q file=%q",
		logicalPath,
		selector,
		metadataPathKindName(kind),
		targetPath,
	)

	if err := s.writeMetadataFile(targetPath, metadata); err != nil {
		debugctx.Printf(ctx, "metadata fs set failed logical_path=%q file=%q error=%v", logicalPath, targetPath, err)
		return err
	}
	debugctx.Printf(ctx, "metadata fs set done logical_path=%q file=%q", logicalPath, targetPath)
	return nil
}

func (s *FSMetadataService) Unset(ctx context.Context, logicalPath string) error {
	debugctx.Printf(ctx, "metadata fs unset start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs unset invalid logical_path=%q error=%v", logicalPath, err)
		return err
	}

	targetPath, err := s.metadataFilePath(selector, kind)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs unset resolve-path failed logical_path=%q selector=%q kind=%q error=%v",
			logicalPath,
			selector,
			metadataPathKindName(kind),
			err,
		)
		return err
	}
	debugctx.Printf(
		ctx,
		"metadata fs unset delete logical_path=%q selector=%q kind=%q file=%q",
		logicalPath,
		selector,
		metadataPathKindName(kind),
		targetPath,
	)

	if err := os.Remove(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			debugctx.Printf(ctx, "metadata fs unset no-op logical_path=%q file=%q", logicalPath, targetPath)
			return nil
		}
		debugctx.Printf(ctx, "metadata fs unset failed logical_path=%q file=%q error=%v", logicalPath, targetPath, err)
		return internalError("failed to remove metadata file", err)
	}

	_ = cleanupEmptyParents(filepath.Dir(targetPath), s.baseDir)
	debugctx.Printf(ctx, "metadata fs unset done logical_path=%q file=%q", logicalPath, targetPath)
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
	var (
		decoded metadatadomain.ResourceMetadata
		err     error
	)

	switch s.resourceFormat {
	case config.ResourceFormatYAML:
		decoded, err = metadatadomain.DecodeResourceMetadataYAML(data)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, validationError("invalid yaml metadata", err)
		}
	default:
		decoded, err = metadatadomain.DecodeResourceMetadataJSON(data)
		if err != nil {
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
		encoded, err := metadatadomain.EncodeResourceMetadataYAML(metadata)
		if err != nil {
			return nil, internalError("failed to encode yaml metadata", err)
		}
		return encoded, nil
	default:
		encoded, err := metadatadomain.EncodeResourceMetadataJSON(metadata, true)
		if err != nil {
			return nil, internalError("failed to encode json metadata", err)
		}
		return ensureTrailingNewline(encoded), nil
	}
}

func ensureTrailingNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}

	result := make([]byte, len(data)+1)
	copy(result, data)
	result[len(data)] = '\n'
	return result
}
