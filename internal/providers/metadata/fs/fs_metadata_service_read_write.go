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
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

type storageResourceMetadata struct {
	IDFromAttribute       string                           `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string                           `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	SecretsFromAttributes *[]string                        `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
	Operations            *map[string]storageOperationSpec `json:"operations,omitempty" yaml:"operations,omitempty"`
	Filter                *[]string                        `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress              *[]string                        `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ                    string                           `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type storageOperationSpec struct {
	Method      string             `json:"method,omitempty" yaml:"method,omitempty"`
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	Query       *map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Accept      string             `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType string             `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Body        any                `json:"body,omitempty" yaml:"body,omitempty"`
	Filter      *[]string          `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress    *[]string          `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ          string             `json:"jq,omitempty" yaml:"jq,omitempty"`
}

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

	storage := resourceMetadataForStorage(metadata)

	switch s.resourceFormat {
	case config.ResourceFormatYAML:
		encoded, err := yaml.Marshal(storage)
		if err != nil {
			return nil, internalError("failed to encode yaml metadata", err)
		}
		return encoded, nil
	default:
		encoded, err := json.MarshalIndent(storage, "", "  ")
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

func resourceMetadataForStorage(metadata metadatadomain.ResourceMetadata) storageResourceMetadata {
	item := storageResourceMetadata{
		IDFromAttribute:    metadata.IDFromAttribute,
		AliasFromAttribute: metadata.AliasFromAttribute,
		JQ:                 metadata.JQ,
	}

	if metadata.SecretsFromAttributes != nil {
		item.SecretsFromAttributes = stringSliceForStorage(metadata.SecretsFromAttributes)
	}
	if metadata.Operations != nil {
		operations := make(map[string]storageOperationSpec, len(metadata.Operations))
		for _, key := range sortedOperationKeys(metadata.Operations) {
			operations[key] = operationSpecForStorage(metadata.Operations[key])
		}
		item.Operations = &operations
	}
	if metadata.Filter != nil {
		item.Filter = stringSliceForStorage(metadata.Filter)
	}
	if metadata.Suppress != nil {
		item.Suppress = stringSliceForStorage(metadata.Suppress)
	}

	return item
}

func operationSpecForStorage(spec metadatadomain.OperationSpec) storageOperationSpec {
	item := storageOperationSpec{
		Method:      spec.Method,
		Path:        spec.Path,
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        spec.Body,
		JQ:          spec.JQ,
	}

	if spec.Query != nil {
		item.Query = stringMapForStorage(spec.Query)
	}
	if spec.Headers != nil {
		item.Headers = stringMapForStorage(spec.Headers)
	}
	if spec.Filter != nil {
		item.Filter = stringSliceForStorage(spec.Filter)
	}
	if spec.Suppress != nil {
		item.Suppress = stringSliceForStorage(spec.Suppress)
	}

	return item
}

func stringSliceForStorage(values []string) *[]string {
	copied := make([]string, len(values))
	copy(copied, values)
	return &copied
}

func stringMapForStorage(values map[string]string) *map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return &copied
}
