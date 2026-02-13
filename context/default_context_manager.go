package context

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/yamlutil"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir  = ".declarest"
	defaultConfigFile = "config"
)

type DefaultContextManager struct {
	ConfigFilePath string

	mu sync.Mutex
}

type storedContext struct {
	Name    string         `yaml:"name"`
	Context *ContextConfig `yaml:"context"`
}

type contextStore struct {
	Contexts       []storedContext `yaml:"contexts"`
	CurrentContext string          `yaml:"currentContext"`
	DefaultEditor  string          `yaml:"defaultEditor,omitempty"`
}

func (m *DefaultContextManager) AddContext(name string, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	if _, ok := store.lookup(name); ok {
		return fmt.Errorf("context %q already exists", name)
	}

	cfg, err := m.readContextConfig(file)
	if err != nil {
		return err
	}

	store.add(name, cfg)
	if store.CurrentContext == "" {
		store.CurrentContext = name
	}

	return m.saveStore(store)
}

func (m *DefaultContextManager) AddContextConfig(name string, cfg *ContextConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("context name is required")
	}
	if cfg == nil {
		return errors.New("context configuration is required")
	}
	cfg = NormalizeContextConfig(cfg)

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	if _, ok := store.lookup(name); ok {
		return fmt.Errorf("context %q already exists", name)
	}

	store.add(name, cfg)
	if store.CurrentContext == "" {
		store.CurrentContext = name
	}

	return m.saveStore(store)
}

func (m *DefaultContextManager) ReplaceContextConfig(name string, cfg *ContextConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("context name is required")
	}
	if cfg == nil {
		return errors.New("context configuration is required")
	}
	cfg = NormalizeContextConfig(cfg)

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	store.replace(name, cfg)
	if store.CurrentContext == "" {
		store.CurrentContext = name
	}

	return m.saveStore(store)
}

func (m *DefaultContextManager) UpdateContext(name string, file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	if _, ok := store.lookup(name); !ok {
		return fmt.Errorf("context %q not found", name)
	}

	cfg, err := m.readContextConfig(file)
	if err != nil {
		return err
	}

	store.replace(name, cfg)
	return m.saveStore(store)
}

func (m *DefaultContextManager) DeleteContext(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	if !store.remove(name) {
		return fmt.Errorf("context %q not found", name)
	}

	if store.CurrentContext == name {
		store.CurrentContext = ""
	}

	return m.saveStore(store)
}

func (m *DefaultContextManager) RenameContext(currentName string, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentName = strings.TrimSpace(currentName)
	newName = strings.TrimSpace(newName)
	if currentName == "" || newName == "" {
		return errors.New("both current and new context names are required")
	}
	if currentName == newName {
		return errors.New("new context name must be different")
	}

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	var targetIdx int = -1
	for idx := range store.Contexts {
		if store.Contexts[idx].Name == currentName {
			targetIdx = idx
			break
		}
	}
	if targetIdx == -1 {
		return fmt.Errorf("context %q not found", currentName)
	}

	for _, entry := range store.Contexts {
		if entry.Name == newName {
			return fmt.Errorf("context %q already exists", newName)
		}
	}

	store.Contexts[targetIdx].Name = newName
	if store.CurrentContext == currentName {
		store.CurrentContext = newName
	}

	return m.saveStore(store)
}

func (m *DefaultContextManager) SetDefaultContext(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	if _, ok := store.lookup(name); !ok {
		return fmt.Errorf("context %q not found", name)
	}

	store.CurrentContext = name
	return m.saveStore(store)
}

func (m *DefaultContextManager) SetDefaultEditor(editor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return err
	}

	store.DefaultEditor = strings.TrimSpace(editor)
	return m.saveStore(store)
}

func (m *DefaultContextManager) GetDefaultContext() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return "", err
	}

	if store.CurrentContext == "" {
		return "", errors.New("no default context configured")
	}

	return store.CurrentContext, nil
}

func (m *DefaultContextManager) GetDefaultEditor() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return "", err
	}

	return store.DefaultEditor, nil
}

func (m *DefaultContextManager) ListContexts() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(store.Contexts))
	for _, entry := range store.Contexts {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (m *DefaultContextManager) GetContextConfig(name string) (*ContextConfig, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false, errors.New("context name is required")
	}

	store, err := m.loadStore()
	if err != nil {
		return nil, false, err
	}

	cfg, ok := store.lookup(name)
	if !ok {
		return nil, false, nil
	}
	if cfg == nil {
		return &ContextConfig{}, true, nil
	}
	return cfg, true, nil
}

func (m *DefaultContextManager) LoadDefaultContext() (Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		return Context{}, err
	}

	if store.CurrentContext == "" {
		return Context{}, errors.New("no default context configured")
	}

	cfg, ok := store.lookup(store.CurrentContext)
	if !ok {
		return Context{}, fmt.Errorf("context %q is missing", store.CurrentContext)
	}

	cfgCopy, err := cloneContextConfig(cfg)
	if err != nil {
		return Context{}, err
	}
	if err := resolveContextEnvPlaceholders(cfgCopy); err != nil {
		return Context{}, fmt.Errorf("failed to resolve environment references for context %q: %w", store.CurrentContext, err)
	}

	recon, err := m.buildReconciler(cfgCopy)
	if err != nil {
		return Context{}, err
	}

	return Context{
		Name:       store.CurrentContext,
		Reconciler: recon,
	}, nil
}

func (m *DefaultContextManager) InitConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, err := m.configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to inspect config file %q: %w", path, err)
	}

	empty := &contextStore{Contexts: []storedContext{}}
	return m.saveStore(empty)
}

func (m *DefaultContextManager) configFilePath() (string, error) {
	if strings.TrimSpace(m.ConfigFilePath) != "" {
		return m.ConfigFilePath, nil
	}

	info, err := ConfigFilePathInfo()
	if err != nil {
		return "", err
	}
	return info.Path, nil
}

func (m *DefaultContextManager) loadStore() (*contextStore, error) {
	path, err := m.configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return &contextStore{Contexts: []storedContext{}}, nil
	default:
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return &contextStore{Contexts: []storedContext{}}, nil
	}

	var store contextStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	if store.Contexts == nil {
		store.Contexts = []storedContext{}
	}

	return &store, nil
}

func (m *DefaultContextManager) saveStore(store *contextStore) error {
	path, err := m.configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yamlutil.MarshalWithIndent(store, 2)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

func (m *DefaultContextManager) readContextConfig(file string) (*ContextConfig, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read context config %q: %w", file, err)
	}

	var cfg ContextConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse context config %q: %w", file, err)
	}

	return NormalizeContextConfig(&cfg), nil
}

func (m *DefaultContextManager) buildReconciler(cfg *ContextConfig) (reconciler.AppReconciler, error) {
	return buildReconcilerFromConfig(cfg)
}

func (s *contextStore) lookup(name string) (*ContextConfig, bool) {
	for idx := range s.Contexts {
		if s.Contexts[idx].Name == name {
			return s.Contexts[idx].Context, true
		}
	}
	return nil, false
}

func (s *contextStore) add(name string, cfg *ContextConfig) {
	s.Contexts = append(s.Contexts, storedContext{
		Name:    name,
		Context: cfg,
	})
}

func (s *contextStore) replace(name string, cfg *ContextConfig) {
	for idx := range s.Contexts {
		if s.Contexts[idx].Name == name {
			s.Contexts[idx].Context = cfg
			return
		}
	}
	s.add(name, cfg)
}

func (s *contextStore) remove(name string) bool {
	for idx := range s.Contexts {
		if s.Contexts[idx].Name == name {
			s.Contexts = append(s.Contexts[:idx], s.Contexts[idx+1:]...)
			return true
		}
	}
	return false
}
