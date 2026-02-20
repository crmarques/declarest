package fsmetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

type storageResourceMetadata struct {
	ResourceInfo  *storageResourceInfo  `json:"resourceInfo,omitempty" yaml:"resourceInfo,omitempty"`
	OperationInfo *storageOperationInfo `json:"operationInfo,omitempty" yaml:"operationInfo,omitempty"`

	// Backward-compatible flat fields.
	IDFromAttribute       string                           `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string                           `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath        string                           `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	SecretsFromAttributes *[]string                        `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
	SecretInAttributes    *[]string                        `json:"secretInAttributes,omitempty" yaml:"secretInAttributes,omitempty"`
	Operations            *map[string]storageOperationSpec `json:"operations,omitempty" yaml:"operations,omitempty"`
	Filter                *[]string                        `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress              *[]string                        `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ                    string                           `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type storageResourceInfo struct {
	IDFromAttribute       string    `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string    `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath        string    `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	SecretInAttributes    *[]string `json:"secretInAttributes,omitempty" yaml:"secretInAttributes,omitempty"`
	SecretsFromAttributes *[]string `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
}

type storageOperationInfo struct {
	Defaults         *storageOperationDefaults `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	GetResource      *storageOperationSpec     `json:"getResource,omitempty" yaml:"getResource,omitempty"`
	CreateResource   *storageOperationSpec     `json:"createResource,omitempty" yaml:"createResource,omitempty"`
	UpdateResource   *storageOperationSpec     `json:"updateResource,omitempty" yaml:"updateResource,omitempty"`
	DeleteResource   *storageOperationSpec     `json:"deleteResource,omitempty" yaml:"deleteResource,omitempty"`
	ListCollection   *storageOperationSpec     `json:"listCollection,omitempty" yaml:"listCollection,omitempty"`
	CompareResources *storageOperationSpec     `json:"compareResources,omitempty" yaml:"compareResources,omitempty"`

	// Backward-compatible operation names.
	Get     *storageOperationSpec `json:"get,omitempty" yaml:"get,omitempty"`
	Create  *storageOperationSpec `json:"create,omitempty" yaml:"create,omitempty"`
	Update  *storageOperationSpec `json:"update,omitempty" yaml:"update,omitempty"`
	Delete  *storageOperationSpec `json:"delete,omitempty" yaml:"delete,omitempty"`
	List    *storageOperationSpec `json:"list,omitempty" yaml:"list,omitempty"`
	Compare *storageOperationSpec `json:"compare,omitempty" yaml:"compare,omitempty"`
}

type storageOperationDefaults struct {
	Filter   *[]string `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress *[]string `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ       string    `json:"jq,omitempty" yaml:"jq,omitempty"`
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
	storage := storageResourceMetadata{}

	switch s.resourceFormat {
	case config.ResourceFormatYAML:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&storage); err != nil {
			return metadatadomain.ResourceMetadata{}, validationError("invalid yaml metadata", err)
		}
	default:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&storage); err != nil {
			return metadatadomain.ResourceMetadata{}, validationError("invalid json metadata", err)
		}
	}

	decoded := resourceMetadataFromStorage(storage)

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
	item := storageResourceMetadata{}

	resourceInfo := storageResourceInfo{
		IDFromAttribute:    metadata.IDFromAttribute,
		AliasFromAttribute: metadata.AliasFromAttribute,
		CollectionPath:     metadata.CollectionPath,
	}
	if metadata.SecretsFromAttributes != nil {
		resourceInfo.SecretInAttributes = stringSliceForStorage(metadata.SecretsFromAttributes)
	}
	if hasStorageResourceInfo(resourceInfo) {
		item.ResourceInfo = &resourceInfo
	}

	operationInfo := storageOperationInfo{}
	if metadata.Filter != nil || metadata.Suppress != nil || metadata.JQ != "" {
		operationInfo.Defaults = &storageOperationDefaults{
			Filter:   stringSliceForStorage(metadata.Filter),
			Suppress: stringSliceForStorage(metadata.Suppress),
			JQ:       metadata.JQ,
		}
	}
	if metadata.Operations != nil {
		if spec, exists := metadata.Operations[string(metadatadomain.OperationGet)]; exists {
			operationInfo.GetResource = operationSpecForStorage(spec)
		}
		if spec, exists := metadata.Operations[string(metadatadomain.OperationCreate)]; exists {
			operationInfo.CreateResource = operationSpecForStorage(spec)
		}
		if spec, exists := metadata.Operations[string(metadatadomain.OperationUpdate)]; exists {
			operationInfo.UpdateResource = operationSpecForStorage(spec)
		}
		if spec, exists := metadata.Operations[string(metadatadomain.OperationDelete)]; exists {
			operationInfo.DeleteResource = operationSpecForStorage(spec)
		}
		if spec, exists := metadata.Operations[string(metadatadomain.OperationList)]; exists {
			operationInfo.ListCollection = operationSpecForStorage(spec)
		}
		if spec, exists := metadata.Operations[string(metadatadomain.OperationCompare)]; exists {
			operationInfo.CompareResources = operationSpecForStorage(spec)
		}
	}
	if metadata.Operations != nil || hasStorageOperationInfo(operationInfo) {
		item.OperationInfo = &operationInfo
	}
	return item
}

func resourceMetadataFromStorage(storage storageResourceMetadata) metadatadomain.ResourceMetadata {
	metadata := metadatadomain.ResourceMetadata{
		IDFromAttribute:    storage.IDFromAttribute,
		AliasFromAttribute: storage.AliasFromAttribute,
		CollectionPath:     storage.CollectionPath,
		JQ:                 storage.JQ,
	}

	if storage.SecretInAttributes != nil {
		metadata.SecretsFromAttributes = cloneStringSlice(*storage.SecretInAttributes)
	} else if storage.SecretsFromAttributes != nil {
		metadata.SecretsFromAttributes = cloneStringSlice(*storage.SecretsFromAttributes)
	}
	if storage.Operations != nil {
		metadata.Operations = make(map[string]metadatadomain.OperationSpec, len(*storage.Operations))
		for key, spec := range *storage.Operations {
			metadata.Operations[key] = operationSpecFromStorage(spec)
		}
	}
	if storage.Filter != nil {
		metadata.Filter = cloneStringSlice(*storage.Filter)
	}
	if storage.Suppress != nil {
		metadata.Suppress = cloneStringSlice(*storage.Suppress)
	}

	if storage.ResourceInfo != nil {
		info := storage.ResourceInfo
		if strings.TrimSpace(info.IDFromAttribute) != "" {
			metadata.IDFromAttribute = info.IDFromAttribute
		}
		if strings.TrimSpace(info.AliasFromAttribute) != "" {
			metadata.AliasFromAttribute = info.AliasFromAttribute
		}
		if strings.TrimSpace(info.CollectionPath) != "" {
			metadata.CollectionPath = info.CollectionPath
		}
		if info.SecretInAttributes != nil {
			metadata.SecretsFromAttributes = cloneStringSlice(*info.SecretInAttributes)
		} else if info.SecretsFromAttributes != nil {
			metadata.SecretsFromAttributes = cloneStringSlice(*info.SecretsFromAttributes)
		}
	}

	if storage.OperationInfo != nil {
		if operations := operationMapFromStorageInfo(storage.OperationInfo); operations != nil {
			if metadata.Operations == nil {
				metadata.Operations = map[string]metadatadomain.OperationSpec{}
			}
			for key, spec := range operations {
				metadata.Operations[key] = spec
			}
		}
		if storage.OperationInfo.Defaults != nil {
			defaults := storage.OperationInfo.Defaults
			if defaults.Filter != nil {
				metadata.Filter = cloneStringSlice(*defaults.Filter)
			}
			if defaults.Suppress != nil {
				metadata.Suppress = cloneStringSlice(*defaults.Suppress)
			}
			if defaults.JQ != "" {
				metadata.JQ = defaults.JQ
			}
		}
		if metadata.Operations == nil &&
			storage.OperationInfo.Defaults == nil &&
			storageOperationInfoExplicitEmpty(storage.OperationInfo) {
			metadata.Operations = map[string]metadatadomain.OperationSpec{}
		}
	}

	return metadata
}

func operationSpecForStorage(spec metadatadomain.OperationSpec) *storageOperationSpec {
	item := &storageOperationSpec{
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

func operationSpecFromStorage(spec storageOperationSpec) metadatadomain.OperationSpec {
	item := metadatadomain.OperationSpec{
		Method:      spec.Method,
		Path:        spec.Path,
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        spec.Body,
		JQ:          spec.JQ,
	}

	if spec.Query != nil {
		item.Query = cloneStringMap(*spec.Query)
	}
	if spec.Headers != nil {
		item.Headers = cloneStringMap(*spec.Headers)
	}
	if spec.Filter != nil {
		item.Filter = cloneStringSlice(*spec.Filter)
	}
	if spec.Suppress != nil {
		item.Suppress = cloneStringSlice(*spec.Suppress)
	}
	return item
}

func stringSliceForStorage(values []string) *[]string {
	if values == nil {
		return nil
	}

	copied := make([]string, len(values))
	copy(copied, values)
	return &copied
}

func stringMapForStorage(values map[string]string) *map[string]string {
	if values == nil {
		return nil
	}

	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return &copied
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func hasStorageResourceInfo(info storageResourceInfo) bool {
	return strings.TrimSpace(info.IDFromAttribute) != "" ||
		strings.TrimSpace(info.AliasFromAttribute) != "" ||
		strings.TrimSpace(info.CollectionPath) != "" ||
		info.SecretInAttributes != nil
}

func hasStorageOperationInfo(info storageOperationInfo) bool {
	return info.Defaults != nil ||
		info.GetResource != nil ||
		info.CreateResource != nil ||
		info.UpdateResource != nil ||
		info.DeleteResource != nil ||
		info.ListCollection != nil ||
		info.CompareResources != nil
}

func storageOperationInfoExplicitEmpty(info *storageOperationInfo) bool {
	if info == nil {
		return false
	}

	return info.GetResource == nil &&
		info.CreateResource == nil &&
		info.UpdateResource == nil &&
		info.DeleteResource == nil &&
		info.ListCollection == nil &&
		info.CompareResources == nil &&
		info.Get == nil &&
		info.Create == nil &&
		info.Update == nil &&
		info.Delete == nil &&
		info.List == nil &&
		info.Compare == nil
}

func operationMapFromStorageInfo(info *storageOperationInfo) map[string]metadatadomain.OperationSpec {
	if info == nil {
		return nil
	}

	operations := map[string]metadatadomain.OperationSpec{}
	set := func(operation metadatadomain.Operation, spec *storageOperationSpec) {
		if spec == nil {
			return
		}
		operations[string(operation)] = operationSpecFromStorage(*spec)
	}

	// Legacy names are loaded first so canonical names win when both are present.
	set(metadatadomain.OperationGet, info.Get)
	set(metadatadomain.OperationCreate, info.Create)
	set(metadatadomain.OperationUpdate, info.Update)
	set(metadatadomain.OperationDelete, info.Delete)
	set(metadatadomain.OperationList, info.List)
	set(metadatadomain.OperationCompare, info.Compare)

	set(metadatadomain.OperationGet, info.GetResource)
	set(metadatadomain.OperationCreate, info.CreateResource)
	set(metadatadomain.OperationUpdate, info.UpdateResource)
	set(metadatadomain.OperationDelete, info.DeleteResource)
	set(metadatadomain.OperationList, info.ListCollection)
	set(metadatadomain.OperationCompare, info.CompareResources)

	if len(operations) == 0 {
		return nil
	}
	return operations
}
