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

package fsmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
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

	selector, kind, err := parseMetadataPath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs set invalid logical_path=%q error=%v", logicalPath, err)
		return err
	}
	if err := validateResourceMetadata(kind, metadata); err != nil {
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

	if err := s.writeMetadataFile(targetPath, kind, metadata); err != nil {
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

	candidates, err := s.metadataFileCandidates(selector, kind)
	if err != nil {
		return err
	}

	removedAny := false
	for _, candidate := range candidates {
		if err := os.Remove(candidate.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			debugctx.Printf(ctx, "metadata fs unset failed logical_path=%q file=%q error=%v", logicalPath, candidate.path, err)
			return internalError("failed to remove metadata file", err)
		}
		removedAny = true
	}
	if !removedAny {
		debugctx.Printf(ctx, "metadata fs unset no-op logical_path=%q file=%q", logicalPath, targetPath)
		return nil
	}

	_ = cleanupEmptyParents(filepath.Dir(targetPath), s.baseDir)
	debugctx.Printf(ctx, "metadata fs unset done logical_path=%q file=%q", logicalPath, targetPath)
	return nil
}

func (s *FSMetadataService) tryReadMetadata(selector string, kind metadataPathKind) (metadatadomain.ResourceMetadata, bool, error) {
	candidates, err := s.metadataFileCandidates(selector, kind)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, false, err
	}

	for _, candidate := range candidates {
		item, err := s.readMetadataFile(candidate.path, candidate.yaml, kind)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return metadatadomain.ResourceMetadata{}, false, err
		}
		return item, true, nil
	}
	return metadatadomain.ResourceMetadata{}, false, nil
}

func (s *FSMetadataService) readMetadataFile(targetPath string, yaml bool, kind metadataPathKind) (metadatadomain.ResourceMetadata, error) {
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	item, err := s.decodeMetadata(data, yaml, kind)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	return item, nil
}

func (s *FSMetadataService) writeMetadataFile(
	targetPath string,
	kind metadataPathKind,
	metadata metadatadomain.ResourceMetadata,
) error {
	yaml, err := metadataPathUsesYAML(targetPath)
	if err != nil {
		return err
	}

	encoded, err := s.encodeMetadata(metadata, yaml, kind)
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

	switch {
	case strings.HasSuffix(targetPath, ".yaml"):
		_ = os.Remove(strings.TrimSuffix(targetPath, ".yaml") + ".json")
	case strings.HasSuffix(targetPath, ".json"):
		_ = os.Remove(strings.TrimSuffix(targetPath, ".json") + ".yaml")
	}

	return nil
}

func (s *FSMetadataService) decodeMetadata(data []byte, yaml bool, kind metadataPathKind) (metadatadomain.ResourceMetadata, error) {
	var (
		decoded metadatadomain.ResourceMetadata
		err     error
	)

	switch {
	case yaml:
		decoded, err = metadatadomain.DecodeResourceMetadataYAML(data)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, faults.NewValidationError("invalid yaml metadata", err)
		}
	default:
		decoded, err = metadatadomain.DecodeResourceMetadataJSON(data)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, faults.NewValidationError("invalid json metadata", err)
		}
	}

	if err := validateResourceMetadata(kind, decoded); err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	return decoded, nil
}

func (s *FSMetadataService) encodeMetadata(metadata metadatadomain.ResourceMetadata, yaml bool, kind metadataPathKind) ([]byte, error) {
	if err := validateResourceMetadata(kind, metadata); err != nil {
		return nil, err
	}

	var (
		encoded []byte
		err     error
	)
	if yaml {
		encoded, err = metadatadomain.EncodeResourceMetadataYAML(metadata)
		if err != nil {
			return nil, internalError("failed to encode yaml metadata", err)
		}
	} else {
		encoded, err = metadatadomain.EncodeResourceMetadataJSON(metadata, true)
		if err != nil {
			return nil, internalError("failed to encode json metadata", err)
		}
	}
	return ensureTrailingNewline(encoded), nil
}

func metadataPathUsesYAML(targetPath string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(targetPath))) {
	case ".yaml":
		return true, nil
	case ".json":
		return false, nil
	default:
		return false, internalError("unsupported metadata file extension", nil)
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
