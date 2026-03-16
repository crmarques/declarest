// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/crmarques/declarest/internal/envref"
)

var _ config.ContextService = (*Service)(nil)
var _ config.ContextCatalogEditor = (*Service)(nil)

type Service struct {
	contextCatalogPath string
}

func NewService(path string) *Service {
	return &Service{contextCatalogPath: path}
}

func (m *Service) Create(_ context.Context, cfg config.Context) error {
	cfg = normalizeConfig(cfg)
	if err := validateResolvedConfig(cfg); err != nil {
		return err
	}

	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	if idx := findContextIndex(contextCatalog.Contexts, cfg.Name); idx >= 0 {
		return faults.NewValidationError(fmt.Sprintf("context %q already exists", cfg.Name), nil)
	}

	contextCatalog, err = mergeContextCredentials(contextCatalog, cfg.Credentials)
	if err != nil {
		return err
	}
	contextCatalog.Contexts = append(contextCatalog.Contexts, cfg)
	if contextCatalog.CurrentContext == "" {
		contextCatalog.CurrentContext = cfg.Name
	}

	return m.saveCatalog(contextCatalog)
}

func (m *Service) Update(_ context.Context, cfg config.Context) error {
	cfg = normalizeConfig(cfg)

	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(contextCatalog.Contexts, cfg.Name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", cfg.Name))
	}

	cfg = preserveProxyOmissions(cfg, normalizeConfig(contextCatalog.Contexts[idx]))
	if err := validateResolvedConfig(cfg); err != nil {
		return err
	}

	contextCatalog, err = mergeContextCredentials(contextCatalog, cfg.Credentials)
	if err != nil {
		return err
	}
	contextCatalog.Contexts[idx] = cfg
	return m.saveCatalog(contextCatalog)
}

func (m *Service) Delete(_ context.Context, name string) error {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	idx := findContextIndex(contextCatalog.Contexts, name)
	if idx < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	contextCatalog.Contexts = append(contextCatalog.Contexts[:idx], contextCatalog.Contexts[idx+1:]...)

	if contextCatalog.CurrentContext == name {
		if len(contextCatalog.Contexts) == 0 {
			contextCatalog.CurrentContext = ""
		} else {
			contextCatalog.CurrentContext = contextCatalog.Contexts[0].Name
		}
	}

	return m.saveCatalog(contextCatalog)
}

func (m *Service) Rename(_ context.Context, fromName string, toName string) error {
	if toName == "" {
		return faults.NewValidationError("context name must not be empty", nil)
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
		return faults.NewValidationError(fmt.Sprintf("context %q already exists", toName), nil)
	}

	contextCatalog.Contexts[fromIdx].Name = toName
	if contextCatalog.CurrentContext == fromName {
		contextCatalog.CurrentContext = toName
	}

	return m.saveCatalog(contextCatalog)
}

func (m *Service) List(_ context.Context) ([]config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}

	contexts := make([]config.Context, len(contextCatalog.Contexts))
	copy(contexts, contextCatalog.Contexts)
	return contexts, nil
}

func (m *Service) SetCurrent(_ context.Context, name string) error {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return err
	}

	if findContextIndex(contextCatalog.Contexts, name) < 0 {
		return notFoundError(fmt.Sprintf("context %q not found", name))
	}

	contextCatalog.CurrentContext = name
	return m.saveCatalog(contextCatalog)
}

func (m *Service) GetCurrent(_ context.Context) (config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}
	if contextCatalog.CurrentContext == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(contextCatalog.Contexts, contextCatalog.CurrentContext)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("current context %q not found", contextCatalog.CurrentContext))
	}

	return contextCatalog.Contexts[idx], nil
}

func (m *Service) ResolveContext(_ context.Context, selection config.ContextSelection) (config.Context, error) {
	contextCatalog, err := m.loadCatalog()
	if err != nil {
		return config.Context{}, err
	}
	resolvedCatalog := envref.ExpandExactEnvPlaceholders(contextCatalog)
	credentials, err := validateCredentials(resolvedCatalog.Credentials)
	if err != nil {
		return config.Context{}, err
	}

	effectiveName := selection.Name
	if effectiveName == "" {
		effectiveName = resolvedCatalog.CurrentContext
	}
	if effectiveName == "" {
		return config.Context{}, notFoundError("current context not set")
	}

	idx := findContextIndex(resolvedCatalog.Contexts, effectiveName)
	if idx < 0 {
		return config.Context{}, notFoundError(fmt.Sprintf("context %q not found", effectiveName))
	}

	resolved, err := applyOverrides(normalizeConfig(resolvedCatalog.Contexts[idx]), selection.Overrides)
	if err != nil {
		return config.Context{}, err
	}
	resolved = applyConfigDefaults(resolved)
	if err := validateConfig(resolved, credentials, true); err != nil {
		return config.Context{}, err
	}
	resolved, err = injectContextCredentials(resolved, credentials)
	if err != nil {
		return config.Context{}, err
	}

	return resolved, nil
}

func (m *Service) Validate(_ context.Context, cfg config.Context) error {
	return validateResolvedConfig(normalizeConfig(cfg))
}

func (m *Service) GetCatalog(_ context.Context) (config.ContextCatalog, error) {
	return m.loadCatalog()
}

func (m *Service) ReplaceCatalog(_ context.Context, catalog config.ContextCatalog) error {
	return m.saveCatalog(catalog)
}

func (m *Service) saveCatalog(contextCatalog config.ContextCatalog) error {
	contextCatalog = compactContextCatalogForPersistence(contextCatalog)

	if err := validateResolvedCatalog(contextCatalog); err != nil {
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

func (m *Service) loadCatalog() (config.ContextCatalog, error) {
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

	// Validate structural constraints (names, duplicates, one-of rules) without
	// expanding env placeholders. Full expansion happens in ResolveContext and
	// saveCatalog, avoiding redundant deep-clone + reflection walks.
	if err := validateCatalog(contextCatalog); err != nil {
		return config.ContextCatalog{}, err
	}

	return contextCatalog, nil
}

func (m *Service) resolveCatalogPath() (string, error) {
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
