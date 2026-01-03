package repository

import (
	"strings"
	"testing"

	"declarest/internal/resource"
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
