package controllers

import (
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
)

func TestPopulateMetadataConfigWithBundlePrefersBundleRef(t *testing.T) {
	t.Parallel()

	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle("/tmp/ignored.tar.gz", "keycloak-bundle:0.0.1", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.Bundle != "keycloak-bundle:0.0.1" {
		t.Fatalf("expected metadata bundle ref to be preserved, got %q", ctx.Metadata.Bundle)
	}
	if ctx.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata base-dir to remain empty, got %q", ctx.Metadata.BaseDir)
	}
}

func TestPopulateMetadataConfigStillSupportsDirectoryArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle(dir, "", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.BaseDir != dir {
		t.Fatalf("expected metadata base-dir %q, got %q", dir, ctx.Metadata.BaseDir)
	}
	if ctx.Metadata.Bundle != "" {
		t.Fatalf("expected metadata bundle to remain empty, got %q", ctx.Metadata.Bundle)
	}
}

func TestPopulateMetadataConfigStillSupportsBundleArchives(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bundle.tar.gz")
	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle(archivePath, "", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.Bundle != archivePath {
		t.Fatalf("expected metadata bundle archive %q, got %q", archivePath, ctx.Metadata.Bundle)
	}
	if ctx.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata base-dir to remain empty, got %q", ctx.Metadata.BaseDir)
	}
}
