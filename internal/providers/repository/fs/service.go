package fs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"go.yaml.in/yaml/v3"
)

var _ repository.ResourceRepositoryManager = (*FSResourceRepository)(nil)

type FSResourceRepository struct {
	baseDir        string
	resourceFormat string
	extension      string
}

func NewFSResourceRepository(baseDir string, resourceFormat string) *FSResourceRepository {
	format := resourceFormat
	if format == "" {
		format = config.ResourceFormatJSON
	}

	extension := ".json"
	if format == config.ResourceFormatYAML {
		extension = ".yaml"
	}

	return &FSResourceRepository{
		baseDir:        filepath.Clean(baseDir),
		resourceFormat: format,
		extension:      extension,
	}
}

func (r *FSResourceRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if normalizedPath == "/" {
		return validationError("logical path must target a resource, not root", nil)
	}

	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return err
	}

	encoded, err := r.encodePayload(normalizedValue)
	if err != nil {
		return internalError("failed to encode payload", err)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return internalError("failed to create resource directory", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".declarest-tmp-*")
	if err != nil {
		return internalError("failed to create temporary file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write temporary payload", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to finalize temporary payload", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace payload file", err)
	}

	return nil
}

func (r *FSResourceRepository) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}
	if normalizedPath == "/" {
		return nil, validationError("logical path must target a resource, not root", nil)
	}

	targetPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, notFoundError(fmt.Sprintf("resource %q not found", normalizedPath))
		}
		return nil, internalError("failed to read resource payload", err)
	}

	decoded, err := r.decodePayload(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (r *FSResourceRepository) Delete(_ context.Context, logicalPath string, policy repository.DeletePolicy) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return err
	}

	if stat, statErr := os.Stat(payloadPath); statErr == nil && !stat.IsDir() {
		if err := os.Remove(payloadPath); err != nil {
			return internalError("failed to remove resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(payloadPath))
		return nil
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return err
	}

	if policy.Recursive {
		return r.deleteCollectionRecursive(collectionPath)
	}
	return r.deleteCollectionDirect(collectionPath)
}

func (r *FSResourceRepository) List(_ context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return nil, err
	}
	if stat, statErr := os.Stat(payloadPath); statErr == nil && !stat.IsDir() {
		return []resource.Resource{buildListedResource(normalizedPath)}, nil
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return nil, err
	}

	if policy.Recursive {
		return r.listRecursive(normalizedPath, collectionPath)
	}
	return r.listDirect(normalizedPath, collectionPath)
}

func (r *FSResourceRepository) Exists(_ context.Context, logicalPath string) (bool, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return false, err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(payloadPath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, internalError("failed to check resource payload", err)
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(collectionPath); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, internalError("failed to check collection path", err)
	}
}

func (r *FSResourceRepository) Move(_ context.Context, fromPath string, toPath string) error {
	fromNormalized, err := resource.NormalizeLogicalPath(fromPath)
	if err != nil {
		return err
	}
	toNormalized, err := resource.NormalizeLogicalPath(toPath)
	if err != nil {
		return err
	}
	if fromNormalized == "/" || toNormalized == "/" {
		return validationError("move requires resource paths", nil)
	}

	fromFile, err := r.payloadFilePath(fromNormalized)
	if err != nil {
		return err
	}
	toFile, err := r.payloadFilePath(toNormalized)
	if err != nil {
		return err
	}

	if _, err := os.Stat(fromFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return notFoundError(fmt.Sprintf("resource %q not found", fromNormalized))
		}
		return internalError("failed to access source resource", err)
	}

	if err := os.MkdirAll(filepath.Dir(toFile), 0o755); err != nil {
		return internalError("failed to create destination directory", err)
	}

	if err := os.Rename(fromFile, toFile); err != nil {
		return internalError("failed to move resource", err)
	}

	_ = r.cleanupEmptyParents(filepath.Dir(fromFile))
	return nil
}

func (r *FSResourceRepository) Init(_ context.Context) error {
	if r.baseDir == "" {
		return validationError("repository base directory must not be empty", nil)
	}
	if err := os.MkdirAll(r.baseDir, 0o755); err != nil {
		return internalError("failed to initialize repository directory", err)
	}
	return nil
}

func (r *FSResourceRepository) Refresh(context.Context) error {
	return nil
}

func (r *FSResourceRepository) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}

func (r *FSResourceRepository) Check(_ context.Context) error {
	info, err := os.Stat(r.baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return notFoundError("repository base directory does not exist")
		}
		return internalError("failed to inspect repository base directory", err)
	}
	if !info.IsDir() {
		return validationError("repository base directory is not a directory", nil)
	}
	return nil
}

func (r *FSResourceRepository) Push(context.Context, repository.PushPolicy) error {
	return validationError("push requires git repository with remote configuration", nil)
}

func (r *FSResourceRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
}

func (r *FSResourceRepository) listDirect(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, internalError("failed to list collection", err)
	}

	items := make([]resource.Resource, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), r.extension)
		logicalPath := path.Join(baseLogicalPath, name)
		if !strings.HasPrefix(logicalPath, "/") {
			logicalPath = "/" + logicalPath
		}
		items = append(items, buildListedResource(logicalPath))
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func (r *FSResourceRepository) listRecursive(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0)

	err := filepath.WalkDir(collectionPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "_" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			return nil
		}

		relPath, relErr := filepath.Rel(collectionPath, filePath)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)
		noExt := strings.TrimSuffix(relPath, r.extension)
		if hasReservedSegment(noExt) {
			return nil
		}

		logicalPath := path.Join(baseLogicalPath, noExt)
		if !strings.HasPrefix(logicalPath, "/") {
			logicalPath = "/" + logicalPath
		}
		items = append(items, buildListedResource(logicalPath))
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, internalError("failed to walk collection", err)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func (r *FSResourceRepository) deleteCollectionDirect(collectionPath string) error {
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to list collection for delete", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			continue
		}
		target := filepath.Join(collectionPath, entry.Name())
		if err := os.Remove(target); err != nil {
			return internalError("failed to delete resource from collection", err)
		}
	}
	return nil
}

func (r *FSResourceRepository) deleteCollectionRecursive(collectionPath string) error {
	err := filepath.WalkDir(collectionPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "_" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			return nil
		}
		if err := os.Remove(filePath); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to recursively delete collection resources", err)
	}
	return nil
}

func (r *FSResourceRepository) payloadFilePath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", validationError("repository base directory must not be empty", nil)
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	filePath := filepath.Join(r.baseDir, filepath.FromSlash(relative+r.extension))
	if !isPathUnderRoot(r.baseDir, filePath) {
		return "", validationError("logical path escapes repository base directory", nil)
	}
	return filePath, nil
}

func (r *FSResourceRepository) collectionDirPath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", validationError("repository base directory must not be empty", nil)
	}
	if logicalPath == "/" {
		return r.baseDir, nil
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	dirPath := filepath.Join(r.baseDir, filepath.FromSlash(relative))
	if !isPathUnderRoot(r.baseDir, dirPath) {
		return "", validationError("logical path escapes repository base directory", nil)
	}
	return dirPath, nil
}

func (r *FSResourceRepository) encodePayload(value resource.Value) ([]byte, error) {
	switch r.resourceFormat {
	case config.ResourceFormatYAML:
		return yaml.Marshal(value)
	case config.ResourceFormatJSON:
		fallthrough
	default:
		return json.MarshalIndent(value, "", "  ")
	}
}

func (r *FSResourceRepository) decodePayload(data []byte) (resource.Value, error) {
	switch r.resourceFormat {
	case config.ResourceFormatYAML:
		var decoded any
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			return nil, validationError("invalid yaml payload", err)
		}
		return resource.Normalize(decoded)
	case config.ResourceFormatJSON:
		fallthrough
	default:
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()

		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, validationError("invalid json payload", err)
		}
		return resource.Normalize(decoded)
	}
}

func (r *FSResourceRepository) cleanupEmptyParents(startDir string) error {
	current := startDir
	root := filepath.Clean(r.baseDir)

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
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && errors.Is(pathErr.Err, fs.ErrInvalid) {
				return nil
			}
			if errors.Is(err, fs.ErrExist) || strings.Contains(err.Error(), "not empty") {
				return nil
			}
			return err
		}

		current = filepath.Dir(current)
	}
}

func buildListedResource(logicalPath string) resource.Resource {
	collectionPath := path.Dir(logicalPath)
	if collectionPath == "." {
		collectionPath = "/"
	}
	if collectionPath == "" {
		collectionPath = "/"
	}
	return resource.Resource{
		LogicalPath:    logicalPath,
		CollectionPath: collectionPath,
		LocalAlias:     path.Base(logicalPath),
	}
}

func hasReservedSegment(value string) bool {
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if segment == "_" {
			return true
		}
	}
	return false
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
