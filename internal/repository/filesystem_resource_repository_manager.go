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
	BaseDir string
}

func NewFileSystemResourceRepositoryManager(baseDir string) *FileSystemResourceRepositoryManager {
	return &FileSystemResourceRepositoryManager{BaseDir: baseDir}
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
	filePath, err := m.resourceFile(path)
	if err != nil {
		return resource.Resource{}, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return resource.Resource{}, err
	}
	if len(data) == 0 {
		return resource.NewResource(nil)
	}

	return resource.NewResourceFromJSON(data)
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
	filePath, err := m.resourceFile(path)
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

	var results []resource.Resource
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		filePath := filepath.Join(dirPath, entry.Name(), "resource.json")
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		res, err := resource.NewResourceFromJSON(data)
		if err != nil {
			continue
		}
		results = append(results, res)
	}

	return results, nil
}

func (m *FileSystemResourceRepositoryManager) ListResourcePaths() []string {
	base := strings.TrimSpace(m.BaseDir)
	if base == "" {
		base = "."
	}

	var paths []string
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "resource.json" {
			return nil
		}
		rel, err := filepath.Rel(base, filepath.Dir(path))
		if err != nil {
			return nil
		}
		if rel == "." {
			paths = append(paths, "/")
		} else {
			paths = append(paths, "/"+filepath.ToSlash(rel))
		}
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

	data, err := res.MarshalJSON()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("failed to format resource payload: %w", err)
	}
	buf.WriteByte('\n')

	return os.WriteFile(filePath, buf.Bytes(), 0o644)
}

func (m *FileSystemResourceRepositoryManager) resourceFile(path string) (string, error) {
	return m.safeJoin(ResourceFileRelPath(path))
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
