package config

import (
	"errors"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestSelectContextForViewCompactsDefaultMetadataBaseDir(t *testing.T) {
	t.Parallel()

	contexts := []configdomain.Context{
		{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
		},
	}

	selected, idx, err := selectContextForView(contexts, "dev")
	if err != nil {
		t.Fatalf("selectContextForView returned error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected selected index 0, got %d", idx)
	}
	if selected.Metadata.BaseDir != "" {
		t.Fatalf("expected default metadata base-dir to be compacted, got %q", selected.Metadata.BaseDir)
	}
}

func TestSelectContextForViewReturnsNotFound(t *testing.T) {
	t.Parallel()

	_, _, err := selectContextForView([]configdomain.Context{{Name: "dev"}}, "prod")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != faults.NotFoundError {
		t.Fatalf("expected not found category, got %q", typedErr.Category)
	}
}

func TestCompactContextCatalogForViewCompactsEntries(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
		},
		CurrentCtx: "dev",
	}

	compacted := compactContextCatalogForView(catalog)
	if compacted.Contexts[0].Metadata.BaseDir != "" {
		t.Fatalf("expected compacted metadata base-dir, got %q", compacted.Contexts[0].Metadata.BaseDir)
	}
}
