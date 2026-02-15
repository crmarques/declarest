package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

var _ config.ContextService = (*FileContextService)(nil)

type FileContextService struct {
	catalogPath string
}

func NewFileContextService(path string) *FileContextService {
	return &FileContextService{catalogPath: path}
}

func (m *FileContextService) Create(_ context.Context, cfg config.Context) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	catalog, err := m.loadCatalogAllowMissing()
	if err != nil {
		return err
	}

	if idx := findContextIndex(catalog.Contexts, cfg.Name); idx >= 0 {
		return validationError(fmt.Sprintf("context %q already exists", cfg.Name), nil)
	}

	catalog.Contexts = append(catalog.Contexts, cfg)
	if catalog.CurrentCtx == "" {
		catalog.CurrentCtx = cfg.Name
	}

	return m.saveCatalog(catalog)
}

func (m *FileContextService) Update(_ context.Context, cfg config.Context) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	catalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(catalog.Contexts, cfg.Name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", cfg.Name))
	}

	catalog.Contexts[idx] = cfg
	return m.saveCatalog(catalog)
}

func (m *FileContextService) Delete(_ context.Context, name string) error {
	catalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(catalog.Contexts, name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	catalog.Contexts = append(catalog.Contexts[:idx], catalog.Contexts[idx+1:]...)

	if catalog.CurrentCtx == name {
		if len(catalog.Contexts) == 0 {
			catalog.CurrentCtx = ""
		} else {
			catalog.CurrentCtx = catalog.Contexts[0].Name
		}
	}

	return m.saveCatalog(catalog)
}

func (m *FileContextService) Rename(_ context.Context, fromName string, toName string) error {
	if toName == "" {
		return validationError("context name must not be empty", nil)
	}

	catalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	fromIdx := findContextIndex(catalog.Contexts, fromName)
	if fromIdx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", fromName))
	}
	if findContextIndex(catalog.Contexts, toName) >= 0 {
		return validationError(fmt.Sprintf("context %q already exists", toName), nil)
	}

	catalog.Contexts[fromIdx].Name = toName
	if catalog.CurrentCtx == fromName {
		catalog.CurrentCtx = toName
	}

	return m.saveCatalog(catalog)
}

func (m *FileContextService) List(_ context.Context) ([]config.Context, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}

	contexts := make([]config.Context, len(catalog.Contexts))
	copy(contexts, catalog.Contexts)
	return contexts, nil
}

func (m *FileContextService) SetCurrent(_ context.Context, name string) error {
	catalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	if findContextIndex(catalog.Contexts, name) < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	catalog.CurrentCtx = name
	return m.saveCatalog(catalog)
}

func (m *FileContextService) GetCurrent(_ context.Context) (config.Context, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}
	if catalog.CurrentCtx == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(catalog.Contexts, catalog.CurrentCtx)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("current context %q not found", catalog.CurrentCtx))
	}

	return catalog.Contexts[idx], nil
}

func (m *FileContextService) ResolveContext(_ context.Context, selection config.ContextSelection) (config.Context, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}

	effectiveName := selection.Name
	if effectiveName == "" {
		effectiveName = catalog.CurrentCtx
	}
	if effectiveName == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(catalog.Contexts, effectiveName)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("context %q not found", effectiveName))
	}

	resolved, err := applyOverrides(catalog.Contexts[idx], selection.Overrides)
	if err != nil {
		return config.Context{}, err
	}
	if err := validateConfig(resolved); err != nil {
		return config.Context{}, err
	}

	return resolved, nil
}

func (m *FileContextService) Validate(_ context.Context, cfg config.Context) error {
	return validateConfig(cfg)
}

func (m *FileContextService) saveCatalog(catalog config.ContextCatalog) error {
	if err := validateCatalog(catalog); err != nil {
		return err
	}

	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return err
	}

	encoded, err := encodeCatalog(catalog)
	if err != nil {
		return internalError("failed to encode context catalog", err)
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return internalError("failed to create context config directory", err)
	}

	tempPath := resolvedPath + ".tmp"
	if err := os.WriteFile(tempPath, encoded, 0o600); err != nil {
		return internalError("failed to write context catalog", err)
	}

	if err := os.Rename(tempPath, resolvedPath); err != nil {
		return internalError("failed to replace context catalog", err)
	}

	return nil
}

func (m *FileContextService) loadCatalog() (config.ContextCatalog, error) {
	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return config.ContextCatalog{}, err
	}

	catalog, err := decodeCatalogFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.ContextCatalog{}, notFoundError(fmt.Sprintf("context catalog not found: %s", resolvedPath))
		}
		return config.ContextCatalog{}, err
	}

	if err := validateCatalog(catalog); err != nil {
		return config.ContextCatalog{}, err
	}

	return catalog, nil
}

func (m *FileContextService) loadCatalogAllowMissing() (config.ContextCatalog, error) {
	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return config.ContextCatalog{}, err
	}

	catalog, err := decodeCatalogFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.ContextCatalog{}, nil
		}
		return config.ContextCatalog{}, err
	}

	if err := validateCatalog(catalog); err != nil {
		return config.ContextCatalog{}, err
	}

	return catalog, nil
}

func (m *FileContextService) resolveCatalogPath() (string, error) {
	return resolveCatalogPath(m.catalogPath)
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

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
