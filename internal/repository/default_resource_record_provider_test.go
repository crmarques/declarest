package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"declarest/internal/resource"
)

func TestMetadataFilesIncludeWildcardSegments(t *testing.T) {
	dir := t.TempDir()

	metaPath := filepath.Join(dir, "admin", "realms", "_", "clients", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"aliasFromAttribute":"clientId"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/admin/realms/publico/clients/testB")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.ResourceInfo == nil {
		t.Fatalf("expected ResourceInfo to be populated")
	}

	if got := record.Meta.ResourceInfo.AliasFromAttribute; got != "clientId" {
		t.Fatalf("expected aliasFromAttribute clientId, got %q", got)
	}
}

func TestRenderStringSupportsRelativePlaceholders(t *testing.T) {
	dir := t.TempDir()

	componentPath := filepath.Join(dir, "admin", "realms", "publico", "components", "ldap-test")
	if err := os.MkdirAll(componentPath, 0o755); err != nil {
		t.Fatalf("mkdir component dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(componentPath, "resource.json"), []byte(`{"id":"ldlOe2kYR2G5PSnSqDz9cg"}`), 0o644); err != nil {
		t.Fatalf("write component resource: %v", err)
	}

	metaPath := filepath.Join(dir, "admin", "realms", "publico", "components", "ldap-test", "mappers", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	metaContent := `{
  "operationInfo": {
    "listCollection": {
      "jqFilter": "{{../../.id}}"
    }
  }
}`
	if err := os.WriteFile(metaPath, []byte(metaContent), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/admin/realms/publico/components/ldap-test/mappers/")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.OperationInfo == nil || record.Meta.OperationInfo.ListCollection == nil {
		t.Fatalf("expected listCollection metadata to be populated")
	}

	if got := strings.TrimSpace(record.Meta.OperationInfo.ListCollection.JQFilter); got != "ldlOe2kYR2G5PSnSqDz9cg" {
		t.Fatalf("unexpected JQ filter value: %q", got)
	}
}

func TestCollectionPathOverrideApplied(t *testing.T) {
	dir := t.TempDir()

	realmPath := filepath.Join(dir, "admin", "realms", "publico")
	if err := os.MkdirAll(realmPath, 0o755); err != nil {
		t.Fatalf("mkdir realm dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realmPath, "resource.json"), []byte(`{"realm":"publico"}`), 0o644); err != nil {
		t.Fatalf("write realm resource: %v", err)
	}

	metaPath := filepath.Join(dir, "admin", "realms", "_", "components", "_", "mappers", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "collectionPath": "/admin/realms/{{.realm}}/components",
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/admin/realms/publico/components/ldap-test/mappers/email")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.ResourceInfo == nil {
		t.Fatalf("expected ResourceInfo")
	}
	if got := record.Meta.ResourceInfo.CollectionPath; got != "/admin/realms/publico/components" {
		t.Fatalf("expected collection path override, got %q", got)
	}
}

func TestDefaultDeletePathIsRendered(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "admin", "realms", "master"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "admin", "realms", "master", "resource.json"), []byte(`{"realm":"master"}`), 0o644); err != nil {
		t.Fatalf("write resource: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/admin/realms/master")
	if err != nil {
		t.Fatalf("GetResourceRecord: %v", err)
	}

	if record.Meta.OperationInfo == nil || record.Meta.OperationInfo.DeleteResource == nil || record.Meta.OperationInfo.DeleteResource.URL == nil {
		t.Fatalf("deleteResource metadata missing")
	}

	if got := record.Meta.OperationInfo.DeleteResource.URL.Path; got != "/admin/realms/{{.id}}" {
		t.Fatalf("expected rendered delete path /admin/realms/{{.id}}, got %q", got)
	}
}

func TestTemplateContextUsesRemoteFallback(t *testing.T) {
	dir := t.TempDir()

	metaPath := filepath.Join(dir, "alpha", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"collectionPath":"/alpha/{{.id}}/beta"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	alphaRes, err := resource.NewResource(map[string]any{"id": "remote-alpha"})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	loader := &fakeRemoteLoader{
		remote: map[string]resource.Resource{
			"/alpha": alphaRes,
		},
	}

	provider := NewDefaultResourceRecordProvider(dir, loader)
	record, err := provider.GetResourceRecord("/alpha/beta/gamma")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.ResourceInfo == nil {
		t.Fatalf("expected ResourceInfo")
	}
	if got := record.Meta.ResourceInfo.CollectionPath; got != "/alpha/remote-alpha/beta" {
		t.Fatalf("expected collection path to use remote fallback, got %q", got)
	}
}

func TestHTTPHeadersAcceptsObjectForm(t *testing.T) {
	dir := t.TempDir()

	metaPath := filepath.Join(dir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "operationInfo": {
    "getResource": {
      "httpHeaders": [
        { "name": "X-Test", "value": "1" }
      ]
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/items/foo")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.OperationInfo == nil || record.Meta.OperationInfo.GetResource == nil {
		t.Fatalf("expected getResource metadata")
	}

	found := false
	for _, header := range record.Meta.OperationInfo.GetResource.HTTPHeaders {
		if header == "X-Test: 1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected httpHeaders to include X-Test: 1, got %#v", record.Meta.OperationInfo.GetResource.HTTPHeaders)
	}
}

func TestSecretAttributesCanBeCleared(t *testing.T) {
	dir := t.TempDir()

	parentPath := filepath.Join(dir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(parentPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(parentPath, []byte(`{
  "resourceInfo": {
    "secretInAttributes": ["secret"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write parent metadata: %v", err)
	}

	childPath := filepath.Join(dir, "items", "foo", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatalf("mkdir child metadata dir: %v", err)
	}
	if err := os.WriteFile(childPath, []byte(`{
  "resourceInfo": {
    "secretInAttributes": []
  }
}`), 0o644); err != nil {
		t.Fatalf("write child metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/items/foo")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.ResourceInfo == nil {
		t.Fatalf("expected ResourceInfo")
	}
	if got := record.Meta.ResourceInfo.SecretInAttributes; len(got) != 0 {
		t.Fatalf("expected secretInAttributes to be cleared, got %#v", got)
	}
}

func TestMetadataLayeringPrefersLiteralOverWildcard(t *testing.T) {
	dir := t.TempDir()

	wildcardPath := filepath.Join(dir, "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(wildcardPath), 0o755); err != nil {
		t.Fatalf("mkdir wildcard metadata dir: %v", err)
	}
	if err := os.WriteFile(wildcardPath, []byte(`{"resourceInfo":{"aliasFromAttribute":"wild"}}`), 0o644); err != nil {
		t.Fatalf("write wildcard metadata: %v", err)
	}

	literalPath := filepath.Join(dir, "items", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(literalPath), 0o755); err != nil {
		t.Fatalf("mkdir literal metadata dir: %v", err)
	}
	if err := os.WriteFile(literalPath, []byte(`{"resourceInfo":{"aliasFromAttribute":"literal"}}`), 0o644); err != nil {
		t.Fatalf("write literal metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	meta, err := provider.GetMergedMetadata("/items/foo")
	if err != nil {
		t.Fatalf("GetMergedMetadata returned error: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected ResourceInfo")
	}
	if got := meta.ResourceInfo.AliasFromAttribute; got != "literal" {
		t.Fatalf("expected literal metadata to win, got %q", got)
	}
}

func TestMetadataArraysOverrideEarlierValues(t *testing.T) {
	dir := t.TempDir()

	parentPath := filepath.Join(dir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(parentPath), 0o755); err != nil {
		t.Fatalf("mkdir parent metadata dir: %v", err)
	}
	if err := os.WriteFile(parentPath, []byte(`{
  "operationInfo": {
    "compareResources": {
      "ignoreAttributes": ["a", "b"]
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write parent metadata: %v", err)
	}

	childPath := filepath.Join(dir, "items", "foo", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatalf("mkdir child metadata dir: %v", err)
	}
	if err := os.WriteFile(childPath, []byte(`{
  "operationInfo": {
    "compareResources": {
      "ignoreAttributes": ["c"]
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write child metadata: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	meta, err := provider.GetMergedMetadata("/items/foo")
	if err != nil {
		t.Fatalf("GetMergedMetadata returned error: %v", err)
	}
	if meta.OperationInfo == nil || meta.OperationInfo.CompareResources == nil {
		t.Fatalf("expected compareResources metadata")
	}

	got := meta.OperationInfo.CompareResources.IgnoreAttributes
	want := []string{"c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected ignoreAttributes %#v, got %#v", want, got)
	}
}

func TestMetadataRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	provider := NewDefaultResourceRecordProvider(dir, nil)
	_, err := provider.GetMergedMetadata("/../escape")
	if err == nil {
		t.Fatalf("expected path traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestQueryStringsRenderTemplates(t *testing.T) {
	dir := t.TempDir()

	metaPath := filepath.Join(dir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  },
  "operationInfo": {
    "getResource": {
      "url": {
        "path": "./{{.id}}",
        "queryStrings": ["trace={{.alias}}", "id={{.id}}", "flag"]
      }
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	resourcePath := filepath.Join(dir, "items", "foo")
	if err := os.MkdirAll(resourcePath, 0o755); err != nil {
		t.Fatalf("mkdir resource dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourcePath, "resource.json"), []byte(`{"id":"123","name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write resource: %v", err)
	}

	provider := NewDefaultResourceRecordProvider(dir, nil)
	record, err := provider.GetResourceRecord("/items/foo")
	if err != nil {
		t.Fatalf("GetResourceRecord returned error: %v", err)
	}

	if record.Meta.OperationInfo == nil || record.Meta.OperationInfo.GetResource == nil || record.Meta.OperationInfo.GetResource.URL == nil {
		t.Fatalf("expected getResource URL metadata")
	}

	got := record.Meta.OperationInfo.GetResource.URL.QueryStrings
	if !containsString(got, "trace=foo") || !containsString(got, "id=123") || !containsString(got, "flag") {
		t.Fatalf("unexpected queryStrings: %#v", got)
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

type fakeRemoteLoader struct {
	remote map[string]resource.Resource
}

func (f *fakeRemoteLoader) GetLocalResource(string) (resource.Resource, error) {
	return resource.Resource{}, fs.ErrNotExist
}

func (f *fakeRemoteLoader) GetRemoteResource(path string) (resource.Resource, error) {
	if f == nil {
		return resource.Resource{}, fs.ErrNotExist
	}
	if res, ok := f.remote[path]; ok {
		return res, nil
	}
	return resource.Resource{}, fs.ErrNotExist
}
