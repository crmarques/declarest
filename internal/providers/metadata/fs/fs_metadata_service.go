package fsmetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var _ metadatadomain.MetadataService = (*FSMetadataService)(nil)

type metadataPathKind int

const (
	metadataPathResource metadataPathKind = iota
	metadataPathCollection
)

type FSMetadataService struct {
	baseDir        string
	resourceFormat string
	extension      string
}

func NewFSMetadataService(baseDir string, resourceFormat string) *FSMetadataService {
	format := resourceFormat
	if format == "" {
		format = config.ResourceFormatJSON
	}

	extension := ".json"
	if format == config.ResourceFormatYAML {
		extension = ".yaml"
	}

	return &FSMetadataService{
		baseDir:        filepath.Clean(baseDir),
		resourceFormat: format,
		extension:      extension,
	}
}

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

func (s *FSMetadataService) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	merged := metadatadomain.ResourceMetadata{}

	apply := func(selector string, kind metadataPathKind) error {
		item, found, err := s.tryReadMetadata(selector, kind)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		merged = mergeResourceMetadata(merged, item)
		return nil
	}

	if err := apply("/", metadataPathCollection); err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	segments := splitPathSegments(targetPath)
	parentSelector := "/"
	for _, segment := range segments {
		wildcards, literals, err := s.matchingCollectionCandidates(parentSelector, segment)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}

		for _, selector := range wildcards {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}
		for _, selector := range literals {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}

		parentSelector = joinSelector(parentSelector, segment)
	}

	if targetPath != "/" {
		if err := apply(targetPath, metadataPathResource); err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
	}

	return merged, nil
}

func (s *FSMetadataService) RenderOperationSpec(
	ctx context.Context,
	logicalPath string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	metadata, err := s.ResolveForPath(ctx, targetPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	templateValue, err := buildTemplateValue(targetPath, metadata, value)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	spec, err := metadatadomain.ResolveOperationSpec(ctx, metadata, operation, templateValue)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}
	return spec, nil
}

func (s *FSMetadataService) Infer(
	ctx context.Context,
	logicalPath string,
	request metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	inferred, err := metadatadomain.InferFromOpenAPI(ctx, targetPath, request)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	existing, found, err := s.tryReadMetadata(targetPath, metadataPathResource)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	if found {
		inferred = mergeResourceMetadata(inferred, existing)
	}

	if request.Apply {
		if err := s.Set(ctx, targetPath, inferred); err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
	}

	return inferred, nil
}

func (s *FSMetadataService) matchingCollectionCandidates(parentSelector string, segment string) ([]string, []string, error) {
	parentDir, err := s.selectorDirPath(parentSelector)
	if err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, internalError("failed to list metadata selectors", err)
	}

	wildcards := make([]string, 0)
	literals := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_" {
			continue
		}

		childName := entry.Name()
		childSelector := joinSelector(parentSelector, childName)

		collectionMetadataPath, pathErr := s.metadataFilePath(childSelector, metadataPathCollection)
		if pathErr != nil {
			return nil, nil, pathErr
		}
		if _, statErr := os.Stat(collectionMetadataPath); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return nil, nil, internalError("failed to inspect metadata selector file", statErr)
		}

		if hasWildcardPattern(childName) {
			matched, matchErr := path.Match(childName, segment)
			if matchErr != nil {
				return nil, nil, validationError(
					fmt.Sprintf("invalid wildcard selector %q", childSelector),
					matchErr,
				)
			}
			if matched {
				wildcards = append(wildcards, childSelector)
			}
			continue
		}

		if childName == segment {
			literals = append(literals, childSelector)
		}
	}

	sort.Strings(wildcards)
	sort.Strings(literals)
	return wildcards, literals, nil
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

func (s *FSMetadataService) metadataFilePath(selector string, kind metadataPathKind) (string, error) {
	if strings.TrimSpace(s.baseDir) == "" {
		return "", validationError("metadata base directory must not be empty", nil)
	}

	relativeSelector := strings.TrimPrefix(selector, "/")

	var targetPath string
	switch kind {
	case metadataPathCollection:
		if relativeSelector == "" {
			targetPath = filepath.Join(s.baseDir, "_", "metadata"+s.extension)
		} else {
			targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector), "_", "metadata"+s.extension)
		}
	case metadataPathResource:
		if relativeSelector == "" {
			return "", validationError("resource metadata path must not target root", nil)
		}
		targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector), "metadata"+s.extension)
	default:
		return "", internalError("unsupported metadata path kind", nil)
	}

	if !isPathUnderRoot(s.baseDir, targetPath) {
		return "", validationError("metadata path escapes metadata base directory", nil)
	}
	return targetPath, nil
}

func (s *FSMetadataService) selectorDirPath(selector string) (string, error) {
	if strings.TrimSpace(s.baseDir) == "" {
		return "", validationError("metadata base directory must not be empty", nil)
	}

	relativeSelector := strings.TrimPrefix(selector, "/")
	targetPath := s.baseDir
	if relativeSelector != "" {
		targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector))
	}
	if !isPathUnderRoot(s.baseDir, targetPath) {
		return "", validationError("metadata path escapes metadata base directory", nil)
	}
	return targetPath, nil
}

func parseMetadataPath(logicalPath string) (string, metadataPathKind, error) {
	value := strings.TrimSpace(logicalPath)
	if value == "" {
		return "", metadataPathResource, validationError("metadata path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", metadataPathResource, validationError("metadata path must be absolute", nil)
	}

	rawSegments := strings.Split(normalizedInput, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return "", metadataPathResource, validationError("metadata path must not contain traversal segments", nil)
		}
		segments = append(segments, segment)
	}

	kind := metadataPathResource
	if len(segments) > 0 && segments[len(segments)-1] == "_" {
		kind = metadataPathCollection
		segments = segments[:len(segments)-1]
	}

	for _, segment := range segments {
		if segment == "_" {
			return "", metadataPathResource, validationError(
				"metadata path must not contain reserved segment \"_\" except as terminal collection marker",
				nil,
			)
		}
		if hasWildcardPattern(segment) {
			if _, err := path.Match(segment, "sample"); err != nil {
				return "", metadataPathResource, validationError("metadata path contains invalid wildcard expression", err)
			}
			kind = metadataPathCollection
		}
	}

	selector := "/"
	if len(segments) > 0 {
		selector = "/" + strings.Join(segments, "/")
	}
	selector = path.Clean(selector)
	if !strings.HasPrefix(selector, "/") {
		return "", metadataPathResource, validationError("metadata path must be absolute", nil)
	}
	if selector != "/" {
		selector = strings.TrimSuffix(selector, "/")
	}

	if kind == metadataPathResource && selector == "/" {
		return "", metadataPathResource, validationError("resource metadata path must not target root", nil)
	}

	return selector, kind, nil
}

func normalizeResolvePath(logicalPath string) (string, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}

	for _, segment := range splitPathSegments(normalizedPath) {
		if hasWildcardPattern(segment) {
			return "", validationError("resolve path must not contain wildcard segments", nil)
		}
	}

	return normalizedPath, nil
}

func buildTemplateValue(
	logicalPath string,
	metadata metadatadomain.ResourceMetadata,
	value any,
) (map[string]any, error) {
	normalizedPayload, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	alias := aliasForLogicalPath(logicalPath)
	collectionPath := collectionPathForLogicalPath(logicalPath)
	remoteID := alias

	scope := map[string]any{
		"logicalPath":    logicalPath,
		"collectionPath": collectionPath,
		"alias":          alias,
		"remoteID":       remoteID,
		"payload":        normalizedPayload,
		"value":          normalizedPayload,
	}

	if payloadMap, ok := normalizedPayload.(map[string]any); ok {
		scope["payload"] = payloadMap
		scope["value"] = payloadMap
		for key, item := range payloadMap {
			scope[key] = item
		}

		if metadata.AliasFromAttribute != "" {
			if aliasValue, exists := payloadMap[metadata.AliasFromAttribute]; exists {
				scope["alias"] = fmt.Sprint(aliasValue)
			}
		}
		if metadata.IDFromAttribute != "" {
			if idValue, exists := payloadMap[metadata.IDFromAttribute]; exists {
				scope["remoteID"] = fmt.Sprint(idValue)
			}
		} else {
			scope["remoteID"] = scope["alias"]
		}
	}

	return scope, nil
}

func validateResourceMetadata(metadata metadatadomain.ResourceMetadata) error {
	keys := sortedOperationKeys(metadata.Operations)
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return validationError(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}
	}
	return nil
}

func mergeResourceMetadata(base metadatadomain.ResourceMetadata, overlay metadatadomain.ResourceMetadata) metadatadomain.ResourceMetadata {
	merged := metadatadomain.ResourceMetadata{
		IDFromAttribute:    base.IDFromAttribute,
		AliasFromAttribute: base.AliasFromAttribute,
		Operations:         cloneOperationMap(base.Operations),
		Filter:             cloneStringSlice(base.Filter),
		Suppress:           cloneStringSlice(base.Suppress),
		JQ:                 base.JQ,
	}

	if overlay.IDFromAttribute != "" {
		merged.IDFromAttribute = overlay.IDFromAttribute
	}
	if overlay.AliasFromAttribute != "" {
		merged.AliasFromAttribute = overlay.AliasFromAttribute
	}
	if overlay.Operations != nil {
		if merged.Operations == nil {
			merged.Operations = map[string]metadatadomain.OperationSpec{}
		}
		keys := sortedOperationKeys(overlay.Operations)
		for _, key := range keys {
			merged.Operations[key] = mergeOperationSpec(merged.Operations[key], overlay.Operations[key])
		}
	}
	if overlay.Filter != nil {
		merged.Filter = cloneStringSlice(overlay.Filter)
	}
	if overlay.Suppress != nil {
		merged.Suppress = cloneStringSlice(overlay.Suppress)
	}
	if overlay.JQ != "" {
		merged.JQ = overlay.JQ
	}

	return merged
}

func mergeOperationSpec(base metadatadomain.OperationSpec, overlay metadatadomain.OperationSpec) metadatadomain.OperationSpec {
	merged := metadatadomain.OperationSpec{
		Method:      base.Method,
		Path:        base.Path,
		Query:       cloneStringMap(base.Query),
		Headers:     cloneStringMap(base.Headers),
		Accept:      base.Accept,
		ContentType: base.ContentType,
		Body:        base.Body,
		Filter:      cloneStringSlice(base.Filter),
		Suppress:    cloneStringSlice(base.Suppress),
		JQ:          base.JQ,
	}

	if overlay.Method != "" {
		merged.Method = overlay.Method
	}
	if overlay.Path != "" {
		merged.Path = overlay.Path
	}
	if overlay.Query != nil {
		if len(overlay.Query) == 0 {
			merged.Query = map[string]string{}
		} else {
			if merged.Query == nil {
				merged.Query = make(map[string]string, len(overlay.Query))
			}
			for _, key := range sortedMapKeys(overlay.Query) {
				merged.Query[key] = overlay.Query[key]
			}
		}
	}
	if overlay.Headers != nil {
		if len(overlay.Headers) == 0 {
			merged.Headers = map[string]string{}
		} else {
			if merged.Headers == nil {
				merged.Headers = make(map[string]string, len(overlay.Headers))
			}
			for _, key := range sortedMapKeys(overlay.Headers) {
				merged.Headers[key] = overlay.Headers[key]
			}
		}
	}
	if overlay.Accept != "" {
		merged.Accept = overlay.Accept
	}
	if overlay.ContentType != "" {
		merged.ContentType = overlay.ContentType
	}
	if overlay.Body != nil {
		merged.Body = overlay.Body
	}
	if overlay.Filter != nil {
		merged.Filter = cloneStringSlice(overlay.Filter)
	}
	if overlay.Suppress != nil {
		merged.Suppress = cloneStringSlice(overlay.Suppress)
	}
	if overlay.JQ != "" {
		merged.JQ = overlay.JQ
	}

	return merged
}

func cleanupEmptyParents(startDir string, rootDir string) error {
	current := startDir
	root := filepath.Clean(rootDir)

	for {
		if current == root {
			return nil
		}
		if current == "." || current == string(filepath.Separator) {
			return nil
		}

		err := os.Remove(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if strings.Contains(err.Error(), "not empty") {
				return nil
			}
			return err
		}

		current = filepath.Dir(current)
	}
}

func splitPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func joinSelector(parent string, child string) string {
	if parent == "/" {
		return "/" + child
	}
	return parent + "/" + child
}

func hasWildcardPattern(segment string) bool {
	return strings.ContainsAny(segment, "*?[")
}

func aliasForLogicalPath(logicalPath string) string {
	if logicalPath == "/" {
		return "/"
	}
	return path.Base(logicalPath)
}

func collectionPathForLogicalPath(logicalPath string) string {
	if logicalPath == "/" {
		return "/"
	}
	collectionPath := path.Dir(logicalPath)
	if collectionPath == "." || collectionPath == "" {
		return "/"
	}
	return collectionPath
}

func sortedOperationKeys(values map[string]metadatadomain.OperationSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneOperationMap(values map[string]metadatadomain.OperationSpec) map[string]metadatadomain.OperationSpec {
	if values == nil {
		return nil
	}
	cloned := make(map[string]metadatadomain.OperationSpec, len(values))
	for _, key := range sortedOperationKeys(values) {
		cloned[key] = mergeOperationSpec(metadatadomain.OperationSpec{}, values[key])
	}
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

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func isPathUnderRoot(root string, candidate string) bool {
	rootClean := filepath.Clean(root)
	candidateClean := filepath.Clean(candidate)

	relPath, err := filepath.Rel(rootClean, candidateClean)
	if err != nil {
		return false
	}
	if relPath == ".." {
		return false
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
