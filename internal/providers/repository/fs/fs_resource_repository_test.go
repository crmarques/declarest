package fs

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
)

func TestFSRepositorySaveGetByFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
	}{
		{name: "json", format: config.ResourceFormatJSON},
		{name: "yaml", format: config.ResourceFormatYAML},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := NewFSResourceRepository(t.TempDir(), test.format)
			if err := repo.Init(context.Background()); err != nil {
				t.Fatalf("Init returned error: %v", err)
			}

			input := map[string]any{"id": 1, "name": "acme"}
			if err := repo.Save(context.Background(), "/customers/acme", input); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}

			got, err := repo.Get(context.Background(), "/customers/acme")
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}

			gotMap, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("expected map payload, got %T", got)
			}
			if gotMap["name"] != "acme" {
				t.Fatalf("expected name=acme, got %#v", gotMap["name"])
			}
		})
	}
}

func TestFSRepositoryTraversalRejected(t *testing.T) {
	t.Parallel()

	repo := NewFSResourceRepository(t.TempDir(), config.ResourceFormatJSON)
	err := repo.Save(context.Background(), "/customers/../passwd", map[string]any{"x": 1})
	assertTypedCategory(t, err, faults.ValidationError)
}

func TestFSRepositoryListDirectAndRecursive(t *testing.T) {
	t.Parallel()

	repo := NewFSResourceRepository(t.TempDir(), config.ResourceFormatJSON)
	saveFixtureResources(t, repo)

	direct, err := repo.List(context.Background(), "/customers", repository.ListPolicy{})
	if err != nil {
		t.Fatalf("List direct returned error: %v", err)
	}
	if len(direct) != 1 {
		t.Fatalf("expected 1 direct resource, got %d", len(direct))
	}
	if direct[0].LogicalPath != "/customers/acme" {
		t.Fatalf("unexpected direct resource %q", direct[0].LogicalPath)
	}

	recursive, err := repo.List(context.Background(), "/customers", repository.ListPolicy{Recursive: true})
	if err != nil {
		t.Fatalf("List recursive returned error: %v", err)
	}
	if len(recursive) != 3 {
		t.Fatalf("expected 3 recursive resources, got %d", len(recursive))
	}
	if recursive[0].LogicalPath != "/customers/acme" ||
		recursive[1].LogicalPath != "/customers/east/zen" ||
		recursive[2].LogicalPath != "/customers/west/zeta" {
		t.Fatalf("unexpected recursive order: %#v", recursive)
	}
}

func TestFSRepositoryDeleteDirectAndRecursive(t *testing.T) {
	t.Parallel()

	repo := NewFSResourceRepository(t.TempDir(), config.ResourceFormatJSON)
	saveFixtureResources(t, repo)

	if err := repo.Delete(context.Background(), "/customers", repository.DeletePolicy{}); err != nil {
		t.Fatalf("Delete direct returned error: %v", err)
	}

	exists, err := repo.Exists(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected /customers/acme to be deleted")
	}

	exists, err = repo.Exists(context.Background(), "/customers/east/zen")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected nested resource to remain after direct delete")
	}

	if err := repo.Delete(context.Background(), "/customers", repository.DeletePolicy{Recursive: true}); err != nil {
		t.Fatalf("Delete recursive returned error: %v", err)
	}

	exists, err = repo.Exists(context.Background(), "/customers/east/zen")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected nested resource to be deleted recursively")
	}
}

func TestFSRepositoryMoveAndExists(t *testing.T) {
	t.Parallel()

	repo := NewFSResourceRepository(t.TempDir(), config.ResourceFormatJSON)
	saveFixtureResources(t, repo)

	if err := repo.Move(context.Background(), "/customers/acme", "/customers/acme-inc"); err != nil {
		t.Fatalf("Move returned error: %v", err)
	}

	oldExists, err := repo.Exists(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Exists old returned error: %v", err)
	}
	if oldExists {
		t.Fatal("expected old path not to exist")
	}

	newExists, err := repo.Exists(context.Background(), "/customers/acme-inc")
	if err != nil {
		t.Fatalf("Exists new returned error: %v", err)
	}
	if !newExists {
		t.Fatal("expected new path to exist")
	}
}

func TestFSRepositoryNoRemoteSyncSemantics(t *testing.T) {
	t.Parallel()

	repo := NewFSResourceRepository(t.TempDir(), config.ResourceFormatJSON)

	status, err := repo.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus returned error: %v", err)
	}
	if status.State != repository.SyncStateNoRemote || status.Ahead != 0 || status.Behind != 0 || status.HasUncommitted {
		t.Fatalf("unexpected sync status: %#v", status)
	}

	err = repo.Push(context.Background(), repository.PushPolicy{})
	assertTypedCategory(t, err, faults.ValidationError)
}

func saveFixtureResources(t *testing.T, repo *FSResourceRepository) {
	t.Helper()

	if err := repo.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	fixtures := []string{
		"/customers/acme",
		"/customers/east/zen",
		"/customers/west/zeta",
	}
	for _, logicalPath := range fixtures {
		err := repo.Save(context.Background(), logicalPath, map[string]any{"path": logicalPath})
		if err != nil {
			t.Fatalf("Save %s returned error: %v", logicalPath, err)
		}
	}
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != category {
		t.Fatalf("expected %q category, got %q", category, typed.Category)
	}
}
