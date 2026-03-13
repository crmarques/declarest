package resource

import (
	"bytes"
	"context"
	"path"
	"strings"
	"testing"
	"time"

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

func TestDefaultsInferCommandRejectsSaveAndCheckTogether(t *testing.T) {
	command := newDefaultsInferCommand(cliutil.CommandDependencies{}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme", "--save", "--check"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.ExecuteContext(context.Background())
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDefaultsInferCommandCheckFailsWhenStoredDefaultsDoNotMatch(t *testing.T) {
	repo := &fakeDefaultsCommandRepository{
		defaults: map[string]resourcedomain.Content{
			"/customers/acme": {
				Value: map[string]any{"labels": map[string]any{"team": "security"}},
				Descriptor: resourcedomain.NormalizePayloadDescriptor(
					resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
				),
			},
		},
	}
	command := newDefaultsInferCommand(cliutil.CommandDependencies{
		Orchestrator: &fakeDefaultsCommandOrchestrator{
			localContent: map[string]resourcedomain.Content{
				"/customers/acme": {
					Value: map[string]any{"id": "acme", "labels": map[string]any{"team": "platform"}},
					Descriptor: resourcedomain.NormalizePayloadDescriptor(
						resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
					),
				},
				"/customers/beta": {
					Value: map[string]any{"id": "beta", "labels": map[string]any{"team": "platform"}},
					Descriptor: resourcedomain.NormalizePayloadDescriptor(
						resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
					),
				},
			},
		},
		Contexts: fakeEditContextService{context: editTestContext()},
		Services: &fakeEditServiceAccessor{
			store:    repo,
			metadata: fakeEditMetadataService{},
		},
	}, &cliutil.GlobalFlags{Output: cliutil.OutputJSON})
	stdout := &bytes.Buffer{}
	command.SetArgs([]string{"/customers/acme", "--check"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(stdout)
	command.SetErr(&bytes.Buffer{})

	err := command.ExecuteContext(context.Background())
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !strings.Contains(stdout.String(), `"team": "platform"`) {
		t.Fatalf("expected inferred defaults output, got %q", stdout.String())
	}
}

func TestDefaultsInferCommandRejectsCollectionPathWithOrWithoutTrailingSlash(t *testing.T) {
	t.Parallel()

	for _, requestedPath := range []string{"/projects", "/projects/"} {
		requestedPath := requestedPath
		t.Run(requestedPath, func(t *testing.T) {
			t.Parallel()

			repo := &fakeDefaultsCommandRepository{}
			command := newDefaultsInferCommand(cliutil.CommandDependencies{
				Orchestrator: &fakeDefaultsCommandOrchestrator{
					localContent: map[string]resourcedomain.Content{
						"/projects/platform": {
							Value: map[string]any{"id": "platform", "name": "platform"},
							Descriptor: resourcedomain.NormalizePayloadDescriptor(
								resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
							),
						},
					},
				},
				Services: &fakeEditServiceAccessor{
					store:    repo,
					metadata: fakeEditMetadataService{},
				},
			}, &cliutil.GlobalFlags{})
			command.SetArgs([]string{requestedPath})
			command.SetIn(bytes.NewBuffer(nil))
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})

			err := command.ExecuteContext(context.Background())
			if !faults.IsCategory(err, faults.NotFoundError) {
				t.Fatalf("expected not found for %q, got %v", requestedPath, err)
			}
		})
	}
}

func TestParseManagedServerDefaultsWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantSet bool
		wantErr bool
	}{
		{name: "empty", input: "", wantSet: false},
		{name: "bare_seconds", input: "2", want: 2 * time.Second, wantSet: true},
		{name: "duration", input: "750ms", want: 750 * time.Millisecond, wantSet: true},
		{name: "negative_seconds", input: "-1", wantErr: true},
		{name: "invalid", input: "later", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotSet, err := parseManagedServerDefaultsWait(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if gotSet != tc.wantSet {
				t.Fatalf("expected set=%t, got %t", tc.wantSet, gotSet)
			}
			if got != tc.want {
				t.Fatalf("expected wait %s, got %s", tc.want, got)
			}
		})
	}
}

type fakeDefaultsCommandOrchestrator struct {
	orchestratordomain.Orchestrator
	content      resourcedomain.Content
	localContent map[string]resourcedomain.Content
}

func (f *fakeDefaultsCommandOrchestrator) ResolveLocalResource(
	_ context.Context,
	logicalPath string,
) (resourcedomain.Resource, error) {
	if f.localContent != nil {
		content, found := f.localContent[logicalPath]
		if !found {
			return resourcedomain.Resource{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
		}
		return resourcedomain.Resource{
			LogicalPath:       logicalPath,
			Payload:           content.Value,
			PayloadDescriptor: content.Descriptor,
		}, nil
	}
	return resourcedomain.Resource{
		LogicalPath:       logicalPath,
		Payload:           f.content.Value,
		PayloadDescriptor: f.content.Descriptor,
	}, nil
}

func (f *fakeDefaultsCommandOrchestrator) GetLocal(_ context.Context, logicalPath string) (resourcedomain.Content, error) {
	if f.localContent != nil {
		content, found := f.localContent[logicalPath]
		if !found {
			return resourcedomain.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
		}
		return content, nil
	}
	return f.content, nil
}

func (f *fakeDefaultsCommandOrchestrator) ListLocal(_ context.Context, logicalPath string, _ orchestratordomain.ListPolicy) ([]resourcedomain.Resource, error) {
	if f.localContent == nil {
		return nil, nil
	}
	items := make([]resourcedomain.Resource, 0, len(f.localContent))
	for candidate := range f.localContent {
		if path.Dir(candidate) != logicalPath {
			continue
		}
		items = append(items, resourcedomain.Resource{LogicalPath: candidate})
	}
	return items, nil
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
