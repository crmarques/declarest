package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestFileSystemRepositoryUnsupportedOperations(t *testing.T) {
	manager := NewFileSystemResourceRepositoryManager(t.TempDir())

	if _, ok := any(manager).(ResourceRepositoryRebaser); ok {
		t.Fatalf("expected filesystem repository to not support refresh operations")
	}
	if _, ok := any(manager).(ResourceRepositoryPusher); ok {
		t.Fatalf("expected filesystem repository to not support remote updates")
	}
	if _, ok := any(manager).(ResourceRepositoryForcePusher); ok {
		t.Fatalf("expected filesystem repository to not support force push updates")
	}
	if _, ok := any(manager).(ResourceRepositoryResetter); ok {
		t.Fatalf("expected filesystem repository to not support reset operations")
	}
}

func TestFileSystemRepositoryRejectsPathTraversal(t *testing.T) {
	manager := NewFileSystemResourceRepositoryManager(t.TempDir())
	if err := manager.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	res, err := resource.NewResource(map[string]any{"id": "x"})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	err = manager.CreateResource("/../escape", res)
	if err == nil {
		t.Fatalf("expected path traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestFileSystemRepositoryRejectsMetadataTraversal(t *testing.T) {
	manager := NewFileSystemResourceRepositoryManager(t.TempDir())
	if err := manager.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	err := manager.WriteMetadata("/../escape", map[string]any{"id": "x"})
	if err == nil {
		t.Fatalf("expected metadata traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestFileSystemRepositorySupportsDistinctMetadataDir(t *testing.T) {
	repoDir := t.TempDir()
	metaDir := t.TempDir()
	manager := NewFileSystemResourceRepositoryManager(repoDir)
	manager.SetMetadataBaseDir(metaDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	path := "/items/foo"
	payload := map[string]any{"id": "x"}
	if err := manager.WriteMetadata(path, payload); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	rel := MetadataFileRelPath(path)
	metaPath := filepath.Join(metaDir, rel)
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected metadata at %s: %v", metaPath, err)
	}

	repoMeta := filepath.Join(repoDir, rel)
	if _, err := os.Stat(repoMeta); err == nil {
		t.Fatalf("expected no metadata in repo dir %s", repoMeta)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected metadata absence, got %v", err)
	}

	read, err := manager.ReadMetadata(path)
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if read["id"] != payload["id"] {
		t.Fatalf("expected metadata id %v, got %v", payload["id"], read["id"])
	}
}

func TestFileSystemRepositoryYAMLFormatWritesYAML(t *testing.T) {
	dir := t.TempDir()
	manager := NewFileSystemResourceRepositoryManager(dir)
	manager.SetResourceFormat(ResourceFormatYAML)
	if err := manager.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	res, err := resource.NewResource(map[string]any{
		"id":    "x",
		"count": 1,
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	if err := manager.CreateResource("/items/foo", res); err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	yamlPath := filepath.Join(dir, "items", "foo", "resource.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("expected resource.yaml to exist: %v", err)
	}
	jsonPath := filepath.Join(dir, "items", "foo", "resource.json")
	if _, err := os.Stat(jsonPath); err == nil {
		t.Fatalf("expected resource.json to be removed when using yaml format")
	}

	loaded, err := manager.GetResource("/items/foo")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	obj, ok := loaded.AsObject()
	if !ok {
		t.Fatalf("expected object, got %#v", loaded.V)
	}
	if obj["id"] != "x" {
		t.Fatalf("expected id to be x, got %#v", obj["id"])
	}
	if _, ok := obj["count"].(json.Number); !ok {
		t.Fatalf("expected count to be json.Number, got %T", obj["count"])
	}
}
