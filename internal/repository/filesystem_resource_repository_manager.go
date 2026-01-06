package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"declarest/internal/resource"
)

type FileSystemResourceRepositoryManager struct {
	BaseDir        string
	ResourceFormat ResourceFormat
}

func NewFileSystemResourceRepositoryManager(baseDir string) *FileSystemResourceRepositoryManager {
	return &FileSystemResourceRepositoryManager{
		BaseDir:        baseDir,
		ResourceFormat: ResourceFormatJSON,
	}
}

func (m *FileSystemResourceRepositoryManager) SetResourceFormat(format ResourceFormat) {
	if m == nil {
		return
	}
	m.ResourceFormat = normalizeResourceFormat(format)
}

func (m *FileSystemResourceRepositoryManager) resourceFormat() ResourceFormat {
	if m == nil {
		return ResourceFormatJSON
	}
	return normalizeResourceFormat(m.ResourceFormat)
}

func (m *FileSystemResourceRepositoryManager) Init() error {
	if m == nil {
		return errors.New("filesystem resource repository manager is nil")
	}
	dir := strings.TrimSpace(m.BaseDir)
	if dir == "" {
		dir = "."
	}
	m.BaseDir = dir
	return os.MkdirAll(dir, 0o755)
}

func (m *FileSystemResourceRepositoryManager) IsLocalRepositoryInitialized() (bool, error) {
	if m == nil {
		return false, errors.New("filesystem resource repository manager is nil")
	}
	baseDir, err := AbsBaseDir(m.BaseDir)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("repository path %s is not a directory", baseDir)
	}
	return true, nil
}

func (m *FileSystemResourceRepositoryManager) GetResource(path string) (resource.Resource, error) {
	candidates, err := m.resourceFileCandidates(path)
	if err != nil {
		return resource.Resource{}, err
	}

	var missing error
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				missing = err
				continue
			}
			return resource.Resource{}, err
		}
		if len(data) == 0 {
			return resource.NewResource(nil)
		}
		return decodeResourcePayload(data, candidate.format)
	}
	if missing != nil {
		return resource.Resource{}, missing
	}
	return resource.Resource{}, fs.ErrNotExist
}

func (m *FileSystemResourceRepositoryManager) CreateResource(path string, res resource.Resource) error {
	return m.writeResource(path, res)
}

func (m *FileSystemResourceRepositoryManager) UpdateResource(path string, res resource.Resource) error {
	return m.writeResource(path, res)
}

func (m *FileSystemResourceRepositoryManager) ApplyResource(path string, res resource.Resource) error {
	return m.writeResource(path, res)
}

func (m *FileSystemResourceRepositoryManager) DeleteResource(path string) error {
	paths, err := m.resourceFilesForDelete(path)
	if err != nil {
		return err
	}

	for _, filePath := range paths {
		if err := os.Remove(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	dir, err := m.resourceDir(path)
	if err == nil {
		_ = os.Remove(dir)
	}

	return nil
}

func (m *FileSystemResourceRepositoryManager) ReadMetadata(path string) (map[string]any, error) {
	filePath, err := m.metadataFile(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}

	var meta map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata file %s: %w", filePath, err)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

func (m *FileSystemResourceRepositoryManager) WriteMetadata(path string, metadata map[string]any) error {
	filePath, err := m.metadataFile(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create directories for metadata %s: %w", path, err)
	}

	if metadata == nil {
		metadata = map[string]any{}
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("failed to format metadata payload: %w", err)
	}
	buf.WriteByte('\n')

	return os.WriteFile(filePath, buf.Bytes(), 0o644)
}

func (m *FileSystemResourceRepositoryManager) DeleteMetadata(path string) error {
	filePath, err := m.metadataFile(path)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	dir := filepath.Dir(filePath)
	_ = os.Remove(dir)

	return nil
}

func (m *FileSystemResourceRepositoryManager) MoveResourceTree(fromPath, toPath string) error {
	if m == nil {
		return errors.New("filesystem resource repository manager is nil")
	}
	fromPath = resource.NormalizePath(fromPath)
	toPath = resource.NormalizePath(toPath)
	if fromPath == toPath {
		return nil
	}
	if resource.IsCollectionPath(fromPath) || resource.IsCollectionPath(toPath) {
		return fmt.Errorf("resource move requires resource paths, not collections")
	}
	if fromPath == "/" || toPath == "/" {
		return fmt.Errorf("cannot move the root resource path")
	}

	fromDir, err := m.resourceDir(fromPath)
	if err != nil {
		return err
	}
	toDir, err := m.resourceDir(toPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(fromDir); err != nil {
		return err
	}
	if _, err := os.Stat(toDir); err == nil {
		return fmt.Errorf("target resource path %s already exists", toPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(toDir), 0o755); err != nil {
		return fmt.Errorf("failed to create target directory for %s: %w", toPath, err)
	}
	return os.Rename(fromDir, toDir)
}

func (m *FileSystemResourceRepositoryManager) GetResourceCollection(path string) ([]resource.Resource, error) {
	dirPath, err := m.collectionDir(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []resource.Resource{}, nil
		}
		return nil, err
	}

	basePath := strings.TrimSuffix(resource.NormalizePath(path), "/")
	var results []resource.Resource
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childPath := resource.NormalizePath(basePath + "/" + entry.Name())
		candidates, err := m.resourceFileCandidates(childPath)
		if err != nil {
			continue
		}
		for _, candidate := range candidates {
			data, err := os.ReadFile(candidate.path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				break
			}
			if len(data) == 0 {
				if res, err := resource.NewResource(nil); err == nil {
					results = append(results, res)
				}
				break
			}
			res, err := decodeResourcePayload(data, candidate.format)
			if err != nil {
				break
			}
			results = append(results, res)
			break
		}
	}

	return results, nil
}

func (m *FileSystemResourceRepositoryManager) ListResourcePaths() []string {
	base := strings.TrimSpace(m.BaseDir)
	if base == "" {
		base = "."
	}

	seen := make(map[string]struct{})
	var paths []string
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !isResourceFileName(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(base, filepath.Dir(path))
		if err != nil {
			return nil
		}
		var logical string
		if rel == "." {
			logical = "/"
		} else {
			logical = "/" + filepath.ToSlash(rel)
		}
		if _, ok := seen[logical]; ok {
			return nil
		}
		seen[logical] = struct{}{}
		paths = append(paths, logical)
		return nil
	})

	sort.Slice(paths, func(i, j int) bool {
		di := strings.Count(strings.Trim(paths[i], "/"), "/")
		dj := strings.Count(strings.Trim(paths[j], "/"), "/")
		if di == dj {
			return paths[i] < paths[j]
		}
		return di < dj
	})

	return paths
}

func (FileSystemResourceRepositoryManager) Close() error { return nil }

func (m *FileSystemResourceRepositoryManager) writeResource(path string, res resource.Resource) error {
	filePath, err := m.resourceFile(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create directories for %s: %w", path, err)
	}

	data, err := encodeResourcePayload(res, m.resourceFormat())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return err
	}

	return m.cleanupAlternativeResourceFiles(path, filePath)
}

func (m *FileSystemResourceRepositoryManager) resourceFile(path string) (string, error) {
	return m.safeJoin(ResourceFileRelPathForFormat(path, m.resourceFormat()))
}

func (m *FileSystemResourceRepositoryManager) resourceDir(path string) (string, error) {
	return m.safeJoin(ResourceDirRelPath(path))
}

func (m *FileSystemResourceRepositoryManager) metadataFile(path string) (string, error) {
	return m.safeJoin(MetadataFileRelPath(path))
}

func (m *FileSystemResourceRepositoryManager) collectionDir(path string) (string, error) {
	return m.safeJoin(ResourceDirRelPath(path))
}

func (m *FileSystemResourceRepositoryManager) safeJoin(rel string) (string, error) {
	return SafeJoin(m.BaseDir, rel)
}

type resourceFileCandidatePath struct {
	path   string
	format ResourceFormat
}

func (m *FileSystemResourceRepositoryManager) resourceFileCandidates(path string) ([]resourceFileCandidatePath, error) {
	candidates := resourceFileRelPathCandidates(path, m.resourceFormat())
	out := make([]resourceFileCandidatePath, 0, len(candidates))
	for _, candidate := range candidates {
		full, err := m.safeJoin(candidate.relPath)
		if err != nil {
			return nil, err
		}
		out = append(out, resourceFileCandidatePath{
			path:   full,
			format: candidate.format,
		})
	}
	return out, nil
}

func (m *FileSystemResourceRepositoryManager) resourceFilesForDelete(path string) ([]string, error) {
	relPaths := resourceFileRelPathsAll(path)
	out := make([]string, 0, len(relPaths))
	for _, rel := range relPaths {
		full, err := m.safeJoin(rel)
		if err != nil {
			return nil, err
		}
		out = append(out, full)
	}
	return out, nil
}

func (m *FileSystemResourceRepositoryManager) cleanupAlternativeResourceFiles(path, keep string) error {
	relPaths := resourceFileRelPathsAll(path)
	for _, rel := range relPaths {
		full, err := m.safeJoin(rel)
		if err != nil {
			return err
		}
		if full == keep {
			continue
		}
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
