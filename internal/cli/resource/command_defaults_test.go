package resource

import (
	"bytes"
	"context"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func TestDefaultsEditCommandClearsDefaultsWhenEditorIsEmpty(t *testing.T) {
	previousEditTempFile := editTempFile
	editTempFile = func(*cobra.Command, string, string, []byte) ([]byte, error) {
		return []byte(""), nil
	}
	defer func() {
		editTempFile = previousEditTempFile
	}()

	repo := &fakeDefaultsCommandRepository{
		defaults: map[string]resourcedomain.Content{
			"/customers/acme": {
				Value: map[string]any{"labels": map[string]any{"team": "platform"}},
				Descriptor: resourcedomain.NormalizePayloadDescriptor(
					resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
				),
			},
		},
	}
	command := newDefaultsEditCommand(cliutil.CommandDependencies{
		Orchestrator: &fakeDefaultsCommandOrchestrator{
			content: resourcedomain.Content{
				Value: map[string]any{
					"id": "acme",
				},
				Descriptor: resourcedomain.NormalizePayloadDescriptor(
					resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
				),
			},
		},
		Contexts: fakeEditContextService{context: editTestContext()},
		Services: &fakeEditServiceAccessor{
			store:    repo,
			metadata: fakeEditMetadataService{},
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if len(repo.saveDefaultsCalls) != 1 {
		t.Fatalf("expected one SaveDefaults call, got %#v", repo.saveDefaultsCalls)
	}
	saved := repo.saveDefaultsCalls[0]
	value, ok := saved.value.Value.(map[string]any)
	if !ok || len(value) != 0 {
		t.Fatalf("expected empty defaults object, got %#v", saved.value.Value)
	}
}

func TestDefaultsInferCommandRequiresYesForManagedServer(t *testing.T) {
	command := newDefaultsInferCommand(cliutil.CommandDependencies{}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme", "--managed-server"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.ExecuteContext(context.Background())
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

type fakeDefaultsCommandOrchestrator struct {
	orchestratordomain.Orchestrator
	content resourcedomain.Content
}

func (f *fakeDefaultsCommandOrchestrator) ResolveLocalResource(
	_ context.Context,
	logicalPath string,
) (resourcedomain.Resource, error) {
	return resourcedomain.Resource{
		LogicalPath:       logicalPath,
		Payload:           f.content.Value,
		PayloadDescriptor: f.content.Descriptor,
	}, nil
}

func (f *fakeDefaultsCommandOrchestrator) GetLocal(_ context.Context, _ string) (resourcedomain.Content, error) {
	return f.content, nil
}

type fakeDefaultsCommandRepository struct {
	repository.ResourceStore
	defaults          map[string]resourcedomain.Content
	saveDefaultsCalls []editCommandSaveCall
}

func (f *fakeDefaultsCommandRepository) GetDefaults(_ context.Context, logicalPath string) (resourcedomain.Content, error) {
	content, found := f.defaults[logicalPath]
	if !found {
		return resourcedomain.Content{}, faults.NewTypedError(faults.NotFoundError, "defaults not found", nil)
	}
	return content, nil
}

func (f *fakeDefaultsCommandRepository) SaveDefaults(_ context.Context, logicalPath string, content resourcedomain.Content) error {
	if f.defaults == nil {
		f.defaults = map[string]resourcedomain.Content{}
	}
	f.defaults[logicalPath] = content
	f.saveDefaultsCalls = append(f.saveDefaultsCalls, editCommandSaveCall{
		logicalPath: logicalPath,
		value:       content,
	})
	return nil
}

func (fakeDefaultsCommandRepository) Save(context.Context, string, resourcedomain.Content) error {
	return nil
}
func (fakeDefaultsCommandRepository) Get(context.Context, string) (resourcedomain.Content, error) {
	return resourcedomain.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
}
func (fakeDefaultsCommandRepository) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (fakeDefaultsCommandRepository) List(context.Context, string, repository.ListPolicy) ([]resourcedomain.Resource, error) {
	return nil, nil
}
func (fakeDefaultsCommandRepository) Exists(context.Context, string) (bool, error) { return false, nil }

type fakeDefaultsCommandMetadata struct {
	metadatadomain.MetadataService
}
