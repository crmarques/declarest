package repository

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	baseDir := t.TempDir()
	_, err := SafeJoin(baseDir, filepath.Join("..", "escape"))
	if err == nil {
		t.Fatalf("expected path traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestResourceFileRelPath(t *testing.T) {
	if got := ResourceFileRelPath("/"); got != filepath.Join("resource.json") {
		t.Fatalf("expected resource.json for root, got %q", got)
	}
	if got := ResourceFileRelPath("/foo/bar"); got != filepath.Join("foo", "bar", "resource.json") {
		t.Fatalf("unexpected resource path, got %q", got)
	}
}

func TestResourceDirRelPath(t *testing.T) {
	if got := ResourceDirRelPath("/"); got != "." {
		t.Fatalf("expected . for root, got %q", got)
	}
	if got := ResourceDirRelPath("/foo/bar"); got != filepath.Join("foo", "bar") {
		t.Fatalf("unexpected resource dir, got %q", got)
	}
}

func TestMetadataFileRelPath(t *testing.T) {
	if got := MetadataFileRelPath("/"); got != filepath.Join("_", "metadata.json") {
		t.Fatalf("expected root collection metadata, got %q", got)
	}
	if got := MetadataFileRelPath("/foo/"); got != filepath.Join("foo", "_", "metadata.json") {
		t.Fatalf("unexpected collection metadata, got %q", got)
	}
	if got := MetadataFileRelPath("/foo"); got != filepath.Join("foo", "metadata.json") {
		t.Fatalf("unexpected resource metadata, got %q", got)
	}
}
