package fsmetadata

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/metadata"
)

func TestFSMetadataServiceSetRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "customers")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	service := NewFSMetadataService(root, "json")
	err := service.Set(context.Background(), "/customers/acme", metadata.ResourceMetadata{})
	if err == nil {
		t.Fatal("expected set to reject symlink escape path")
	}
	if !strings.Contains(err.Error(), "metadata path escapes metadata base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
