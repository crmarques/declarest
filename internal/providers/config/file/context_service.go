package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

var _ config.ContextService = (*FileContextService)(nil)
var _ config.ContextCatalogEditor = (*FileContextService)(nil)

type FileContextService struct {
	contextCatalogPath string
}

func NewFileContextService(path string) *FileContextService {
	return &FileContextService{contextCatalogPath: path}
}

func (m *FileContextService) Create(_ context.Context, cfg config.Context) error {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return err
	}

	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	if idx := findContextIndex(contextCatalog.Contexts, cfg.Name); idx >= 0 {
		return validationError(fmt.Sprintf("context %q already exists", cfg.Name), nil)
	}

	contextCatalog.Contexts = append(contextCatalog.Contexts, cfg)
	if contextCatalog.CurrentCtx == "" {
		contextCatalog.CurrentCtx = cfg.Name
	}

	return m.saveCatalog(contextCatalog)
}

func (m *FileContextService) Update(_ context.Context, cfg config.Context) error {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return err
	}

	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(contextCatalog.Contexts, cfg.Name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", cfg.Name))
	}

	contextCatalog.Contexts[idx] = cfg
	return m.saveCatalog(contextCatalog)
}

func (m *FileContextService) Delete(_ context.Context, name string) error {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(contextCatalog.Contexts, name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	contextCatalog.Contexts = append(contextCatalog.Contexts[:idx], contextCatalog.Contexts[idx+1:]...)

	if contextCatalog.CurrentCtx == name {
		if len(contextCatalog.Contexts) == 0 {
			contextCatalog.CurrentCtx = ""
		} else {
			contextCatalog.CurrentCtx = contextCatalog.Contexts[0].Name
		}
	}

	return m.saveCatalog(contextCatalog)
}

func (m *FileContextService) Rename(_ context.Context, fromName string, toName string) error {
	if toName == "" {
		return validationError("context name must not be empty", nil)
	}

	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	fromIdx := findContextIndex(contextCatalog.Contexts, fromName)
	if fromIdx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", fromName))
	}
	if findContextIndex(contextCatalog.Contexts, toName) >= 0 {
		return validationError(fmt.Sprintf("context %q already exists", toName), nil)
	}

	contextCatalog.Contexts[fromIdx].Name = toName
	if contextCatalog.CurrentCtx == fromName {
		contextCatalog.CurrentCtx = toName
	}

	return m.saveCatalog(contextCatalog)
}

func (m *FileContextService) List(_ context.Context) ([]config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}

	contexts := make([]config.Context, len(contextCatalog.Contexts))
	copy(contexts, contextCatalog.Contexts)
	return contexts, nil
}

func (m *FileContextService) SetCurrent(_ context.Context, name string) error {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	if findContextIndex(contextCatalog.Contexts, name) < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	contextCatalog.CurrentCtx = name
	return m.saveCatalog(contextCatalog)
}

func (m *FileContextService) GetCurrent(_ context.Context) (config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}
	if contextCatalog.CurrentCtx == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(contextCatalog.Contexts, contextCatalog.CurrentCtx)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("current context %q not found", contextCatalog.CurrentCtx))
	}

	return contextCatalog.Contexts[idx], nil
}

func (m *FileContextService) ResolveContext(_ context.Context, selection config.ContextSelection) (config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}

	effectiveName := selection.Name
	if effectiveName == "" {
		effectiveName = contextCatalog.CurrentCtx
	}
	if effectiveName == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(contextCatalog.Contexts, effectiveName)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("context %q not found", effectiveName))
	}

	resolved, err := applyOverrides(normalizeConfig(contextCatalog.Contexts[idx]), selection.Overrides)
	if err != nil {
		return config.Context{}, err
	}
	resolved = applyConfigDefaults(resolved)
	if err := validateConfig(resolved); err != nil {
		return config.Context{}, err
	}

	return resolved, nil
}

func (m *FileContextService) Validate(_ context.Context, cfg config.Context) error {
	return validateConfig(normalizeConfig(cfg))
}

func (m *FileContextService) GetCatalog(_ context.Context) (config.ContextCatalog, error) {
	return m.loadCatalog()
}

func (m *FileContextService) ReplaceCatalog(_ context.Context, catalog config.ContextCatalog) error {
	return m.saveCatalog(catalog)
}

func (m *FileContextService) saveCatalog(contextCatalog config.ContextCatalog) error {
	contextCatalog = compactContextCatalogForPersistence(contextCatalog)

	if err := validateCatalog(contextCatalog); err != nil {
		return err
	}

	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return err
	}

	encoded, err := encodeCatalog(contextCatalog)
	if err != nil {
		return internalError("failed to encode context catalog", err)
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return internalError("failed to create context config directory", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(resolvedPath), ".declarest-contexts-*")
	if err != nil {
		return internalError("failed to create temporary context catalog file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write context catalog", err)
	}
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to set context catalog permissions", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to finalize context catalog", err)
	}

	if err := os.Rename(tempPath, resolvedPath); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace context catalog", err)
	}

	if err := ensureUserOnlyReadWriteFile(resolvedPath); err != nil {
		return err
	}

	return nil
}

func (m *FileContextService) loadCatalog() (config.ContextCatalog, error) {
	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return config.ContextCatalog{}, err
	}

	contextCatalog, err := decodeCatalogFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.ContextCatalog{}, nil
		}
		return config.ContextCatalog{}, err
	}
	if err := ensureUserOnlyReadWriteFile(resolvedPath); err != nil {
		return config.ContextCatalog{}, err
	}

	if err := validateCatalog(contextCatalog); err != nil {
		return config.ContextCatalog{}, err
	}

	return contextCatalog, nil
}

func (m *FileContextService) resolveCatalogPath() (string, error) {
	return resolveCatalogPath(m.contextCatalogPath)
}

func findContextIndex(contexts []config.Context, name string) int {
	for idx, item := range contexts {
		if item.Name == name {
			return idx
		}
	}
	return -1
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func compactContextCatalogForPersistence(contextCatalog config.ContextCatalog) config.ContextCatalog {
	contextCatalog.DefaultEditor = strings.TrimSpace(contextCatalog.DefaultEditor)

	if len(contextCatalog.Contexts) == 0 {
		return contextCatalog
	}

	compacted := contextCatalog
	compacted.Contexts = make([]config.Context, len(contextCatalog.Contexts))
	for idx, item := range contextCatalog.Contexts {
		compacted.Contexts[idx] = compactConfigForPersistence(item)
	}

	return compacted
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}

func ensureUserOnlyReadWriteFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to inspect context catalog permissions", err)
	}

	if info.Mode().Perm() == 0o600 {
		return nil
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return internalError("failed to update context catalog permissions", err)
	}
	return nil
}
