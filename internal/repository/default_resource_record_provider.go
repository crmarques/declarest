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

	"declarest/internal/metadata"
	"declarest/internal/openapi"
	"declarest/internal/resource"
)

type DefaultResourceRecordProvider struct {
	BaseDir        string
	fileStore      FileStore
	resourceLoader ResourceLoader
	resourceFormat ResourceFormat
	remoteMu       sync.Mutex
	remoteInFlight map[string]int
	openapiSpec    *openapi.Spec
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

func NewDefaultResourceRecordProvider(baseDir string, loader ResourceLoader) *DefaultResourceRecordProvider {
	return &DefaultResourceRecordProvider{
		BaseDir:        baseDir,
		fileStore:      NewFileSystemStore(baseDir),
		resourceLoader: loader,
		resourceFormat: ResourceFormatJSON,
	}
}

func (p *DefaultResourceRecordProvider) SetFileStore(store FileStore) {
	if p == nil {
		return
	}
	p.fileStore = store
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
		p.fileStore = NewFileSystemStore(p.BaseDir)
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

	result := metadata.DefaultMetadata(collectionSegments)
	if p.openapiSpec != nil {
		result = openapi.ApplyDefaults(result, trimmed, isCollection, p.openapiSpec)
	}

	files := metadataRelPaths(segments, collectionSegments, isCollection)
	store := p.store()
	for _, relPath := range files {
		data, err := store.ReadFile(relPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return resource.ResourceMetadata{}, fmt.Errorf("failed to read metadata file %q: %w", relPath, err)
		}

		if len(data) == 0 {
			continue
		}

		var fileMetadata resource.ResourceMetadata
		if err := json.Unmarshal(data, &fileMetadata); err != nil {
			return resource.ResourceMetadata{}, fmt.Errorf("failed to parse metadata file %q: %w", relPath, err)
		}

		result = metadata.MergeMetadata(result, fileMetadata)
	}

	if renderTemplates {
		ctx := p.buildMetadataTemplateContext(trimmed)
		opts := metadata.RenderOptions{
			RelativePlaceholderResolver: p.resolveResourceAttributes,
		}
		result = metadata.RenderTemplates(result, trimmed, ctx, opts)
	}

	return result, nil
}

func metadataRelPaths(segments, collectionSegments []string, isCollection bool) []string {
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
		for _, variant := range expandWithWildcards(collectionSegments[:depth]) {
			wildcards := countWildcards(variant)
			addCandidate(filepath.Join(filepath.Join(variant...), "metadata.json"), len(variant), wildcards)
			addCandidate(filepath.Join(filepath.Join(variant...), "_", "metadata.json"), len(variant)+1, wildcards+1)
		}
	}

	if !isCollection && len(segments) > 0 {
		addCandidate(filepath.Join(filepath.Join(segments...), "metadata.json"), len(segments), countWildcards(segments))
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
		files []string
		seen  = make(map[string]struct{})
	)

	add := func(rel string) error {
		if strings.TrimSpace(rel) == "" {
			return nil
		}
		if _, ok := seen[rel]; ok {
			return nil
		}
		seen[rel] = struct{}{}
		files = append(files, rel)
		return nil
	}

	for _, cand := range candidates {
		_ = add(cand.rel)
	}

	return files
}

func expandWithWildcards(segments []string) [][]string {
	if len(segments) == 0 {
		return nil
	}

	results := [][]string{{}}
	for _, segment := range segments {
		var next [][]string
		for _, prefix := range results {
			next = append(next, append(append([]string{}, prefix...), segment))
			next = append(next, append(append([]string{}, prefix...), "_"))
		}
		results = next
	}

	return results
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
