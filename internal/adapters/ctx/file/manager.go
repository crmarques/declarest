package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/ctx"
	metadatanoop "github.com/crmarques/declarest/internal/adapters/noop/metadata"
	repositorynoop "github.com/crmarques/declarest/internal/adapters/noop/repository"
	secretsnoop "github.com/crmarques/declarest/internal/adapters/noop/secrets"
	servernoop "github.com/crmarques/declarest/internal/adapters/noop/server"
)

var _ ctx.Manager = (*Manager)(nil)

type Manager struct {
	catalogPath string
}

func NewManager(path string) *Manager {
	return &Manager{catalogPath: path}
}

func (m *Manager) Create(_ context.Context, cfg ctx.Config) error {
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

func (m *Manager) Update(_ context.Context, cfg ctx.Config) error {
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

func (m *Manager) Delete(_ context.Context, name string) error {
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

func (m *Manager) Rename(_ context.Context, fromName string, toName string) error {
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

func (m *Manager) List(_ context.Context) ([]ctx.Config, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}

	contexts := make([]ctx.Config, len(catalog.Contexts))
	copy(contexts, catalog.Contexts)
	return contexts, nil
}

func (m *Manager) SetCurrent(_ context.Context, name string) error {
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

func (m *Manager) GetCurrent(_ context.Context) (ctx.Config, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return ctx.Config{}, err
	}
	if catalog.CurrentCtx == "" {
		return ctx.Config{}, notFoundError("current context not set")
	}

	idx := findContextIndex(catalog.Contexts, catalog.CurrentCtx)
	if idx < 0 {
		return ctx.Config{}, notFoundError(fmt.Sprintf("current context %q not found", catalog.CurrentCtx))
	}

	return catalog.Contexts[idx], nil
}

func (m *Manager) LoadResolvedConfig(_ context.Context, name string, overrides map[string]string) (ctx.Runtime, error) {
	catalog, err := m.loadCatalog()
	if err != nil {
		return ctx.Runtime{}, err
	}

	effectiveName := name
	if effectiveName == "" {
		effectiveName = catalog.CurrentCtx
	}
	if effectiveName == "" {
		return ctx.Runtime{}, notFoundError("current context not set")
	}

	idx := findContextIndex(catalog.Contexts, effectiveName)
	if idx < 0 {
		return ctx.Runtime{}, notFoundError(fmt.Sprintf("context %q not found", effectiveName))
	}

	resolved, err := applyOverrides(catalog.Contexts[idx], overrides)
	if err != nil {
		return ctx.Runtime{}, err
	}
	if err := validateConfig(resolved); err != nil {
		return ctx.Runtime{}, err
	}

	runtime := ctx.Runtime{
		Name:        resolved.Name,
		Environment: copyMap(overrides),
		Repository:  &repositorynoop.Manager{},
		Metadata:    &metadatanoop.Manager{},
	}

	if resolved.ManagedServer != nil {
		runtime.Server = &servernoop.Manager{}
	}
	if resolved.SecretStore != nil {
		runtime.Secrets = &secretsnoop.Manager{}
	}

	return runtime, nil
}

func (m *Manager) Validate(_ context.Context, cfg ctx.Config) error {
	return validateConfig(cfg)
}

func (m *Manager) saveCatalog(catalog ctx.Catalog) error {
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

func (m *Manager) loadCatalog() (ctx.Catalog, error) {
	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return ctx.Catalog{}, err
	}

	catalog, err := decodeCatalogFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ctx.Catalog{}, notFoundError(fmt.Sprintf("context catalog not found: %s", resolvedPath))
		}
		return ctx.Catalog{}, err
	}

	if err := validateCatalog(catalog); err != nil {
		return ctx.Catalog{}, err
	}

	return catalog, nil
}

func (m *Manager) loadCatalogAllowMissing() (ctx.Catalog, error) {
	resolvedPath, err := m.resolveCatalogPath()
	if err != nil {
		return ctx.Catalog{}, err
	}

	catalog, err := decodeCatalogFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ctx.Catalog{}, nil
		}
		return ctx.Catalog{}, err
	}

	if err := validateCatalog(catalog); err != nil {
		return ctx.Catalog{}, err
	}

	return catalog, nil
}

func (m *Manager) resolveCatalogPath() (string, error) {
	return resolveCatalogPath(m.catalogPath)
}

func findContextIndex(contexts []ctx.Config, name string) int {
	for idx, item := range contexts {
		if item.Name == name {
			return idx
		}
	}
	return -1
}

func copyMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func validationError(message string, cause error) error {
	return core.NewTypedError(core.ValidationError, message, cause)
}

func notFoundError(message string) error {
	return core.NewTypedError(core.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return core.NewTypedError(core.InternalError, message, cause)
}
