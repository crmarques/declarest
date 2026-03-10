package metadata

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func TestEditCommandCreatesMetadataWhenMissing(t *testing.T) {
	previousEditTempFile := editTempFile
	editTempFile = func(*cobra.Command, string, string, []byte) ([]byte, error) {
		return []byte("resource:\n  id: /name\n"), nil
	}
	defer func() {
		editTempFile = previousEditTempFile
	}()

	service := &fakeEditMetadataService{
		items: map[string]metadatadomain.ResourceMetadata{},
	}
	command := newEditCommand(cliutil.CommandDependencies{
		Services: fakeEditServiceAccessor{metadata: service},
	})
	command.SetArgs([]string{"/customers/acme"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	saved, found := service.items["/customers/acme"]
	if !found {
		t.Fatal("expected metadata to be saved")
	}
	if saved.ID != "/name" {
		t.Fatalf("expected saved id /name, got %q", saved.ID)
	}
}

func TestEditCommandRejectsInvalidYAML(t *testing.T) {
	previousEditTempFile := editTempFile
	editTempFile = func(*cobra.Command, string, string, []byte) ([]byte, error) {
		return []byte("resource: ["), nil
	}
	defer func() {
		editTempFile = previousEditTempFile
	}()

	service := &fakeEditMetadataService{
		items: map[string]metadatadomain.ResourceMetadata{
			"/customers/acme": {ID: "/id"},
		},
	}
	command := newEditCommand(cliutil.CommandDependencies{
		Services: fakeEditServiceAccessor{metadata: service},
	})
	command.SetArgs([]string{"/customers/acme"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error category, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid yaml metadata") {
		t.Fatalf("expected invalid yaml metadata error, got %v", err)
	}
}

type fakeEditMetadataService struct {
	metadatadomain.MetadataService
	items map[string]metadatadomain.ResourceMetadata
}

func (f *fakeEditMetadataService) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	item, found := f.items[logicalPath]
	if !found {
		return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
	}
	return item, nil
}

func (f *fakeEditMetadataService) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	if f.items == nil {
		f.items = map[string]metadatadomain.ResourceMetadata{}
	}
	f.items[logicalPath] = metadata
	return nil
}

type fakeEditServiceAccessor struct {
	orchestrator.ServiceAccessor
	metadata metadatadomain.MetadataService
}

func (f fakeEditServiceAccessor) RepositoryStore() repository.ResourceStore { return nil }
func (f fakeEditServiceAccessor) RepositorySync() repository.RepositorySync { return nil }
func (f fakeEditServiceAccessor) MetadataService() metadatadomain.MetadataService {
	return f.metadata
}
func (f fakeEditServiceAccessor) SecretProvider() secretdomain.SecretProvider { return nil }
func (f fakeEditServiceAccessor) ManagedServerClient() managedserverdomain.ManagedServerClient {
	return nil
}
