package secrets

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"declarest/internal/resource"
)

type memorySecretsManager struct {
	store map[string]map[string]string
}

func (m *memorySecretsManager) Init() error  { return nil }
func (m *memorySecretsManager) Close() error { return nil }

func (m *memorySecretsManager) GetSecret(resourcePath string, key string) (string, error) {
	if m == nil || m.store == nil {
		return "", errors.New("secret not found")
	}
	if entry, ok := m.store[resourcePath]; ok {
		if value, ok := entry[key]; ok {
			return value, nil
		}
	}
	return "", errors.New("secret not found")
}

func (m *memorySecretsManager) CreateSecret(resourcePath string, key string, value string) error {
	return m.UpdateSecret(resourcePath, key, value)
}

func (m *memorySecretsManager) UpdateSecret(resourcePath string, key string, value string) error {
	if m.store == nil {
		m.store = make(map[string]map[string]string)
	}
	if _, ok := m.store[resourcePath]; !ok {
		m.store[resourcePath] = make(map[string]string)
	}
	m.store[resourcePath][key] = value
	return nil
}

func (m *memorySecretsManager) DeleteSecret(resourcePath string, key string, value string) error {
	if m.store == nil {
		return nil
	}
	if entry, ok := m.store[resourcePath]; ok {
		delete(entry, key)
	}
	return nil
}

func (m *memorySecretsManager) ListKeys(resourcePath string) []string {
	if m.store == nil {
		return nil
	}
	entry, ok := m.store[resourcePath]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(entry))
	for key := range entry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *memorySecretsManager) ListResources() ([]string, error) {
	if m.store == nil {
		return nil, nil
	}
	paths := make([]string, 0, len(m.store))
	for path := range m.store {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func TestMaskResourceSecretsStoresAndMasks(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"password": "p",
		"nested": map[string]any{
			"token": "t",
		},
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	manager := &memorySecretsManager{}
	updated, err := MaskResourceSecrets(res, "/items/foo", []string{"password", "nested.token"}, manager, true)
	if err != nil {
		t.Fatalf("MaskResourceSecrets: %v", err)
	}

	if val, err := manager.GetSecret("/items/foo", "password"); err != nil || val != "p" {
		t.Fatalf("expected stored password, got %q (err=%v)", val, err)
	}
	if val, err := manager.GetSecret("/items/foo", "nested.token"); err != nil || val != "t" {
		t.Fatalf("expected stored nested token, got %q (err=%v)", val, err)
	}

	obj, ok := updated.AsObject()
	if !ok {
		t.Fatalf("expected object payload, got %#v", updated.V)
	}
	if obj["password"] != secretTemplatePath {
		t.Fatalf("expected password to be masked, got %#v", obj["password"])
	}
	nested, ok := obj["nested"].(map[string]any)
	if !ok || nested["token"] != secretTemplatePath {
		t.Fatalf("expected nested token to be masked, got %#v", obj["nested"])
	}
}

func TestResolveResourceSecretsReplacesPlaceholders(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"password": "{{secret .}}",
		"nested": map[string]any{
			"token": "{{secret \"custom\"}}",
		},
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	manager := &memorySecretsManager{
		store: map[string]map[string]string{
			"/items/foo": {
				"password": "p",
				"custom":   "t",
			},
		},
	}

	updated, err := ResolveResourceSecrets(res, "/items/foo", []string{"password", "nested.token"}, manager)
	if err != nil {
		t.Fatalf("ResolveResourceSecrets: %v", err)
	}

	obj, ok := updated.AsObject()
	if !ok {
		t.Fatalf("expected object payload, got %#v", updated.V)
	}
	if obj["password"] != "p" {
		t.Fatalf("expected password to be resolved, got %#v", obj["password"])
	}
	nested, ok := obj["nested"].(map[string]any)
	if !ok || nested["token"] != "t" {
		t.Fatalf("expected nested token to be resolved, got %#v", obj["nested"])
	}
}

func TestHasSecretPlaceholders(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"password": "{{secret .}}",
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	has, err := HasSecretPlaceholders(res, []string{"password"})
	if err != nil {
		t.Fatalf("HasSecretPlaceholders: %v", err)
	}
	if !has {
		t.Fatalf("expected placeholders to be detected")
	}
}

func TestMaskResourceSecretsHandlesIndexedPaths(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"config": map[string]any{
			"bindCredential": []any{"super"},
		},
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	manager := &memorySecretsManager{}
	updated, err := MaskResourceSecrets(res, "/items/foo", []string{"config.bindCredential[0]"}, manager, true)
	if err != nil {
		t.Fatalf("MaskResourceSecrets: %v", err)
	}

	if val, err := manager.GetSecret("/items/foo", "config.bindCredential[0]"); err != nil || val != "super" {
		t.Fatalf("expected stored indexed secret, got %q (err=%v)", val, err)
	}

	obj, ok := updated.AsObject()
	if !ok {
		t.Fatalf("expected object payload, got %#v", updated.V)
	}
	config, ok := obj["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config object, got %#v", obj["config"])
	}
	creds, ok := config["bindCredential"].([]any)
	if !ok || len(creds) != 1 {
		t.Fatalf("expected bindCredential array, got %#v", config["bindCredential"])
	}
	if creds[0] != secretTemplatePath {
		t.Fatalf("expected indexed secret to be masked, got %#v", creds[0])
	}
}

func TestMaskResourceSecretsRejectsCollectionsWhenStoring(t *testing.T) {
	res, err := resource.NewResource([]any{map[string]any{"password": "p"}})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	manager := &memorySecretsManager{}
	_, err = MaskResourceSecrets(res, "/items", []string{"password"}, manager, true)
	if err == nil {
		t.Fatalf("expected collection secret storage to fail")
	}
	if !strings.Contains(err.Error(), "cannot store secrets for collection resources") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveResourceSecretsRejectsCollections(t *testing.T) {
	res, err := resource.NewResource([]any{map[string]any{"password": "{{secret .}}"}})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	manager := &memorySecretsManager{}
	_, err = ResolveResourceSecrets(res, "/items", []string{"password"}, manager)
	if err == nil {
		t.Fatalf("expected collection secret resolution to fail")
	}
	if !strings.Contains(err.Error(), "cannot resolve secrets for collection resources") {
		t.Fatalf("unexpected error: %v", err)
	}
}
