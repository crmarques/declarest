package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"
)

type DefaultResourceRecordProvider struct {
	MetadataBaseDir string
	fileStore       FileStore
	metadataManager MetadataRepositoryManager
	resourceLoader  ResourceLoader
	resourceFormat  ResourceFormat
	remoteMu        sync.Mutex
	remoteInFlight  map[string]int
	openapiSpec     *openapi.Spec
}

type ResourceLoader interface {
	GetLocalResource(path string) (resource.Resource, error)
}

type RemoteResourceLoader interface {
	GetRemoteResource(path string) (resource.Resource, error)
}

type FileStore interface {
	ReadFile(path string) ([]byte, error)
}

type FileSystemStore struct {
	BaseDir string
}

func NewFileSystemStore(baseDir string) *FileSystemStore {
	return &FileSystemStore{BaseDir: baseDir}
}

func (s *FileSystemStore) ReadFile(relPath string) ([]byte, error) {
	full, err := SafeJoin(s.BaseDir, relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func NewDefaultResourceRecordProvider(metadataBaseDir string, loader ResourceLoader) *DefaultResourceRecordProvider {
	return &DefaultResourceRecordProvider{
		MetadataBaseDir: metadataBaseDir,
		fileStore:       NewFileSystemStore(metadataBaseDir),
		resourceLoader:  loader,
		resourceFormat:  ResourceFormatJSON,
	}
}

func (p *DefaultResourceRecordProvider) SetFileStore(store FileStore) {
	if p == nil {
		return
	}
	p.fileStore = store
}

func (p *DefaultResourceRecordProvider) SetMetadataManager(manager MetadataRepositoryManager) {
	if p == nil {
		return
	}
	p.metadataManager = manager
}

func (p *DefaultResourceRecordProvider) SetResourceFormat(format ResourceFormat) {
	if p == nil {
		return
	}
	p.resourceFormat = normalizeResourceFormat(format)
}

func (p *DefaultResourceRecordProvider) SetOpenAPISpec(spec *openapi.Spec) {
	if p == nil {
		return
	}
	p.openapiSpec = spec
}

func (p *DefaultResourceRecordProvider) OpenAPISpec() *openapi.Spec {
	if p == nil {
		return nil
	}
	return p.openapiSpec
}

func (p *DefaultResourceRecordProvider) store() FileStore {
	if p == nil {
		return nil
	}
	if p.fileStore == nil {
		p.fileStore = NewFileSystemStore(p.MetadataBaseDir)
	}
	return p.fileStore
}

func (p *DefaultResourceRecordProvider) format() ResourceFormat {
	if p == nil {
		return ResourceFormatJSON
	}
	return normalizeResourceFormat(p.resourceFormat)
}

func (p *DefaultResourceRecordProvider) GetResourceRecord(resourcePath string) (resource.ResourceRecord, error) {
	meta, err := p.resolveMetadata(resourcePath)
	if err != nil {
		return resource.ResourceRecord{}, err
	}

	record := resource.ResourceRecord{
		Path: resourcePath,
		Meta: meta,
	}

	if p.resourceLoader != nil {
		if data, err := p.resourceLoader.GetLocalResource(resourcePath); err == nil {
			record.Data = data
		}
	}

	return record, nil
}

func (p *DefaultResourceRecordProvider) GetMergedMetadata(resourcePath string) (resource.ResourceMetadata, error) {
	return p.resolveMetadataInternal(resourcePath, false)
}

func (p *DefaultResourceRecordProvider) resolveMetadata(resourcePath string) (resource.ResourceMetadata, error) {
	return p.resolveMetadataInternal(resourcePath, true)
}

func (p *DefaultResourceRecordProvider) resolveMetadataInternal(resourcePath string, renderTemplates bool) (resource.ResourceMetadata, error) {
	trimmed := strings.TrimSpace(resourcePath)
	isCollection := strings.HasSuffix(trimmed, "/")
	clean := strings.Trim(trimmed, " /")
	segments := resource.SplitPathSegments(clean)
	collectionSegments := segments
	if !isCollection && len(collectionSegments) > 0 {
		collectionSegments = collectionSegments[:len(collectionSegments)-1]
	}
	collectionDepth := len(collectionSegments)

	result := metadata.DefaultMetadata(collectionSegments)

	files := metadataRelPaths(segments, collectionSegments, isCollection)
	for _, candidate := range files {
		fileMetadata, ok, err := p.readMetadataCandidate(candidate)
		if err != nil {
			return resource.ResourceMetadata{}, err
		}
		if !ok {
			continue
		}

		if candidate.depth < collectionDepth && fileMetadata.ResourceInfo != nil {
			fileMetadata.ResourceInfo.IDFromAttribute = ""
			fileMetadata.ResourceInfo.AliasFromAttribute = ""
		}

		result = metadata.MergeMetadata(result, fileMetadata)
	}

	ctx := p.buildMetadataTemplateContext(trimmed)
	opts := metadata.RenderOptions{
		RelativePlaceholderResolver: p.resolveResourceAttributes,
	}
	if renderTemplates {
		result = metadata.RenderTemplates(result, trimmed, ctx, opts)
	}

	if p.openapiSpec != nil {
		resourcePathForDefaults := trimmed
		if isCollection {
			if coll := strings.TrimSpace(result.ResourceInfo.CollectionPath); coll != "" {
				resourcePathForDefaults = coll
			}
		} else if remotePath := p.resourcePathForDefaults(trimmed, result); remotePath != "" {
			resourcePathForDefaults = remotePath
		}
		result = openapi.ApplyDefaults(result, resourcePathForDefaults, isCollection, p.openapiSpec)
		if renderTemplates {
			result = metadata.RenderTemplates(result, trimmed, ctx, opts)
		}
	}

	return result, nil
}

func (p *DefaultResourceRecordProvider) readMetadataCandidate(candidate metadataCandidate) (resource.ResourceMetadata, bool, error) {
	if manager := p.metadataManager; manager != nil {
		if path := metadataPathFromRel(candidate.rel); path != "" {
			meta, err := manager.ReadMetadata(path)
			if err != nil {
				return resource.ResourceMetadata{}, false, fmt.Errorf("failed to read metadata %q: %w", path, err)
			}
			if len(meta) == 0 {
				return resource.ResourceMetadata{}, false, nil
			}
			fileMetadata, err := metadataFromMap(meta)
			if err != nil {
				return resource.ResourceMetadata{}, false, fmt.Errorf("failed to parse metadata %q: %w", path, err)
			}
			return fileMetadata, true, nil
		}
	}

	store := p.store()
	data, err := store.ReadFile(candidate.rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resource.ResourceMetadata{}, false, nil
		}
		return resource.ResourceMetadata{}, false, fmt.Errorf("failed to read metadata file %q: %w", candidate.rel, err)
	}
	if len(data) == 0 {
		return resource.ResourceMetadata{}, false, nil
	}

	var fileMetadata resource.ResourceMetadata
	if err := json.Unmarshal(data, &fileMetadata); err != nil {
		return resource.ResourceMetadata{}, false, fmt.Errorf("failed to parse metadata file %q: %w", candidate.rel, err)
	}
	return fileMetadata, true, nil
}

func metadataFromMap(meta map[string]any) (resource.ResourceMetadata, error) {
	if len(meta) == 0 {
		return resource.ResourceMetadata{}, nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return resource.ResourceMetadata{}, err
	}
	var parsed resource.ResourceMetadata
	if err := json.Unmarshal(data, &parsed); err != nil {
		return resource.ResourceMetadata{}, err
	}
	return parsed, nil
}

func (p *DefaultResourceRecordProvider) resourcePathForDefaults(resourcePath string, meta resource.ResourceMetadata) string {
	record := resource.ResourceRecord{
		Path: resourcePath,
		Meta: meta,
	}
	if p.resourceLoader != nil {
		if data, err := p.resourceLoader.GetLocalResource(resourcePath); err == nil {
			record.Data = data
		}
	}
	return record.RemoteResourcePath(record.Data)
}

type metadataCandidate struct {
	rel   string
	depth int
}

func metadataRelPaths(segments, collectionSegments []string, isCollection bool) []metadataCandidate {
	type candidate struct {
		rel       string
		depth     int
		wildcards int
	}

	var (
		candidates []candidate
		seenRel    = make(map[string]struct{})
	)

	addCandidate := func(rel string, depth, wildcards int) {
		if strings.TrimSpace(rel) == "" {
			return
		}
		if _, ok := seenRel[rel]; ok {
			return
		}
		seenRel[rel] = struct{}{}
		candidates = append(candidates, candidate{rel: rel, depth: depth, wildcards: wildcards})
	}

	addCandidate(filepath.Join("metadata.json"), 0, 0)
	addCandidate(filepath.Join("_", "metadata.json"), 1, 1)

	for depth := 1; depth <= len(collectionSegments); depth++ {
		for _, variant := range resource.PathWildcardVariants(collectionSegments[:depth]) {
			wildcards := countWildcards(variant)
			addCandidate(filepath.Join(filepath.Join(variant...), "metadata.json"), len(variant), wildcards)
			addCandidate(filepath.Join(filepath.Join(variant...), "_", "metadata.json"), len(variant)+1, wildcards+1)
		}
	}

	if !isCollection && len(segments) > 0 {
		for _, variant := range resource.PathWildcardVariants(segments) {
			wildcards := countWildcards(variant)
			addCandidate(filepath.Join(filepath.Join(variant...), "metadata.json"), len(variant), wildcards)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth < candidates[j].depth
		}
		if candidates[i].wildcards != candidates[j].wildcards {
			return candidates[i].wildcards > candidates[j].wildcards
		}
		return candidates[i].rel < candidates[j].rel
	})

	var (
		files []metadataCandidate
		seen  = make(map[string]struct{})
	)

	add := func(rel string, depth int) error {
		if strings.TrimSpace(rel) == "" {
			return nil
		}
		if _, ok := seen[rel]; ok {
			return nil
		}
		seen[rel] = struct{}{}
		files = append(files, metadataCandidate{rel: rel, depth: depth})
		return nil
	}

	for _, cand := range candidates {
		_ = add(cand.rel, cand.depth)
	}

	return files
}

func countWildcards(segments []string) int {
	count := 0
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "_" {
			count++
		}
	}
	return count
}

func metadataPathFromRel(rel string) string {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	switch rel {
	case "", "metadata.json":
		return ""
	case "_/metadata.json":
		return "/"
	}
	if strings.HasSuffix(rel, "/_/metadata.json") {
		dir := strings.TrimSuffix(rel, "/_/metadata.json")
		if dir == "" {
			return "/"
		}
		return "/" + dir + "/"
	}
	if strings.HasSuffix(rel, "/metadata.json") {
		dir := strings.TrimSuffix(rel, "/metadata.json")
		if dir == "" {
			return ""
		}
		return "/" + dir
	}
	return ""
}

func (p *DefaultResourceRecordProvider) buildMetadataTemplateContext(resourcePath string) map[string]any {
	ctx := make(map[string]any)
	trimmed := strings.Trim(resourcePath, " /")
	if trimmed == "" {
		return ctx
	}

	segments := resource.SplitPathSegments(trimmed)
	for i := 1; i <= len(segments); i++ {
		prefix := "/" + strings.Join(segments[:i], "/")
		if attrs, ok := p.resolveResourceAttributes(prefix); ok {
			mergeContext(ctx, attrs)
		}
	}

	return ctx
}

func mergeContext(dst map[string]any, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

func (p *DefaultResourceRecordProvider) resolveResourceAttributes(path string) (map[string]any, bool) {
	if p.resourceLoader != nil {
		if res, err := p.resourceLoader.GetLocalResource(path); err == nil {
			if obj, ok := res.AsObject(); ok {
				return obj, true
			}
		}
	}

	data, format, err := p.readResourcePayload(path)
	if err != nil {
		return p.loadRemoteResourceAttributes(path)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return p.loadRemoteResourceAttributes(path)
	}

	res, err := decodeResourcePayload(data, format)
	if err != nil {
		return p.loadRemoteResourceAttributes(path)
	}

	obj, ok := res.AsObject()
	if !ok {
		return p.loadRemoteResourceAttributes(path)
	}

	return obj, true
}

func (p *DefaultResourceRecordProvider) readResourcePayload(path string) ([]byte, ResourceFormat, error) {
	store := p.store()
	candidates := resourceFileRelPathCandidates(path, p.format())
	var missing error
	for _, candidate := range candidates {
		data, err := store.ReadFile(candidate.relPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = err
				continue
			}
			return nil, candidate.format, err
		}
		return data, candidate.format, nil
	}
	if missing != nil {
		return nil, p.format(), missing
	}
	return nil, p.format(), os.ErrNotExist
}

func (p *DefaultResourceRecordProvider) loadRemoteResourceAttributes(path string) (map[string]any, bool) {
	loader, ok := p.resourceLoader.(RemoteResourceLoader)
	if !ok || loader == nil {
		return nil, false
	}
	if !p.enterRemote(path) {
		return nil, false
	}
	defer p.exitRemote(path)

	res, err := loader.GetRemoteResource(path)
	if err != nil {
		return nil, false
	}
	obj, ok := res.AsObject()
	if !ok {
		return nil, false
	}
	return obj, true
}

func (p *DefaultResourceRecordProvider) enterRemote(path string) bool {
	p.remoteMu.Lock()
	defer p.remoteMu.Unlock()

	if p.remoteInFlight == nil {
		p.remoteInFlight = make(map[string]int)
	}
	if p.remoteInFlight[path] > 0 {
		return false
	}
	p.remoteInFlight[path] = 1
	return true
}

func (p *DefaultResourceRecordProvider) exitRemote(path string) {
	p.remoteMu.Lock()
	defer p.remoteMu.Unlock()

	if p.remoteInFlight == nil {
		return
	}
	if p.remoteInFlight[path] <= 1 {
		delete(p.remoteInFlight, path)
		return
	}
	p.remoteInFlight[path]--
}

func (p *DefaultResourceRecordProvider) MetadataChildCollections(baseSegments []string) ([]string, error) {
	metadataDir := strings.TrimSpace(p.MetadataBaseDir)
	if metadataDir == "" {
		return nil, nil
	}

	node, ok := metadataCompletionNode(metadataDir, baseSegments)
	if !ok {
		return nil, nil
	}

	children, err := os.ReadDir(node)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, child := range children {
		if !child.IsDir() {
			continue
		}
		name := child.Name()
		if name == "_" || strings.HasPrefix(name, ".") {
			continue
		}
		if !dirExists(filepath.Join(node, name, "_")) {
			continue
		}
		results = append(results, name)
	}
	sort.Strings(results)
	return results, nil
}

func metadataCompletionNode(baseDir string, segments []string) (string, bool) {
	current := strings.TrimSpace(baseDir)
	if current == "" {
		return "", false
	}

	if len(segments) == 0 {
		if dirExists(current) {
			return current, true
		}
		return "", false
	}

	for _, candidate := range resource.PathWildcardVariants(segments) {
		path := filepath.Join(current, filepath.Join(candidate...))
		if !dirExists(path) {
			continue
		}
		if filepath.Base(path) == "_" || dirExists(filepath.Join(path, "_")) {
			return path, true
		}
	}

	return "", false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
