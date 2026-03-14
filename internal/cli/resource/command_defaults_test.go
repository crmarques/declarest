package resource

import (
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
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

	repo := &fakeDefaultsCommandRepository{}
	md := &fakeDefaultsCommandMetadata{
		items: map[string]metadatadomain.ResourceMetadata{
			"/customers/acme": {
				Defaults: &metadatadomain.DefaultsSpec{
					Value: map[string]any{"labels": map[string]any{"team": "platform"}},
				},
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
			metadata: md,
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if item, found := md.items["/customers/acme"]; found && item.Defaults != nil {
		t.Fatalf("expected local defaults to be cleared, got %#v", item.Defaults)
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
	repo := &fakeDefaultsCommandRepository{}
	md := &fakeDefaultsCommandMetadata{
		items: map[string]metadatadomain.ResourceMetadata{
			"/customers/_": {
				Defaults: &metadatadomain.DefaultsSpec{
					Value: map[string]any{"labels": map[string]any{"team": "security"}},
				},
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
			metadata: md,
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

func TestDefaultsInferCommandAcceptsCollectionAndResourcePaths(t *testing.T) {
	t.Parallel()

	tests := []string{"/admin/realms", "/admin/realms/", "/admin/realms/master"}

	for _, requestedPath := range tests {
		requestedPath := requestedPath
		t.Run(requestedPath, func(t *testing.T) {
			t.Parallel()

			repo := &fakeDefaultsCommandRepository{}
			command := newDefaultsInferCommand(cliutil.CommandDependencies{
				Orchestrator: &fakeDefaultsCommandOrchestrator{
					localContent: map[string]resourcedomain.Content{
						"/admin/realms/acme": {
							Value: map[string]any{"realm": "acme", "enabled": true, "sslRequired": "external"},
							Descriptor: resourcedomain.NormalizePayloadDescriptor(
								resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
							),
						},
						"/admin/realms/master": {
							Value: map[string]any{"realm": "master", "enabled": true, "sslRequired": "external"},
							Descriptor: resourcedomain.NormalizePayloadDescriptor(
								resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON},
							),
						},
					},
				},
				Services: &fakeEditServiceAccessor{
					store:    repo,
					metadata: &fakeDefaultsCommandMetadata{items: map[string]metadatadomain.ResourceMetadata{}},
				},
			}, &cliutil.GlobalFlags{Output: cliutil.OutputJSON})
			stdout := &bytes.Buffer{}
			command.SetArgs([]string{requestedPath})
			command.SetIn(bytes.NewBuffer(nil))
			command.SetOut(stdout)
			command.SetErr(&bytes.Buffer{})

			if err := command.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("expected success for %q, got %v", requestedPath, err)
			}
			if !strings.Contains(stdout.String(), `"enabled": true`) {
				t.Fatalf("expected inferred defaults output for %q, got %q", requestedPath, stdout.String())
			}
			if !strings.Contains(stdout.String(), `"sslRequired": "external"`) {
				t.Fatalf("expected inferred defaults output for %q, got %q", requestedPath, stdout.String())
			}
		})
	}
}

func TestDefaultsInferCommandSaveWritesToExplicitMetadataBaseDir(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metadataDir := t.TempDir()
	metadataService := fsmetadata.NewLayeredFSMetadataService(metadataDir, repoDir, fsmetadata.LayeredMetadataWriteShared)

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
		Contexts: fakeEditContextService{context: defaultsTestContext(repoDir, metadataDir, "")},
		Services: &fakeEditServiceAccessor{
			store:    &fakeDefaultsCommandRepository{},
			metadata: metadataService,
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme", "--save"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	sharedMetadataPath := filepath.Join(metadataDir, "customers", "_", "metadata.yaml")
	if _, err := os.Stat(sharedMetadataPath); err != nil {
		t.Fatalf("expected shared metadata file %q, got %v", sharedMetadataPath, err)
	}
	sharedDefaultsPath := filepath.Join(metadataDir, "customers", "_", "defaults.json")
	if _, err := os.Stat(sharedDefaultsPath); err != nil {
		t.Fatalf("expected shared defaults artifact %q, got %v", sharedDefaultsPath, err)
	}
	repoDefaultsPath := filepath.Join(repoDir, "customers", "_", "defaults.json")
	if _, err := os.Stat(repoDefaultsPath); !os.IsNotExist(err) {
		t.Fatalf("expected repo-local defaults artifact %q to remain absent, got %v", repoDefaultsPath, err)
	}
}

func TestDefaultsInferCommandSaveWritesBundleBackedDefaultsToRepoOverlay(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	sharedDir := t.TempDir()
	metadataService := fsmetadata.NewLayeredFSMetadataService(sharedDir, repoDir, fsmetadata.LayeredMetadataWriteLocal)

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
		Contexts: fakeEditContextService{context: defaultsTestContext(repoDir, "", "keycloak-bundle:0.0.1")},
		Services: &fakeEditServiceAccessor{
			store:    &fakeDefaultsCommandRepository{},
			metadata: metadataService,
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/customers/acme", "--save"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	repoMetadataPath := filepath.Join(repoDir, "customers", "_", "metadata.yaml")
	if _, err := os.Stat(repoMetadataPath); err != nil {
		t.Fatalf("expected repo-local metadata file %q, got %v", repoMetadataPath, err)
	}
	repoDefaultsPath := filepath.Join(repoDir, "customers", "_", "defaults.json")
	if _, err := os.Stat(repoDefaultsPath); err != nil {
		t.Fatalf("expected repo-local defaults artifact %q, got %v", repoDefaultsPath, err)
	}
	sharedDefaultsPath := filepath.Join(sharedDir, "customers", "_", "defaults.json")
	if _, err := os.Stat(sharedDefaultsPath); !os.IsNotExist(err) {
		t.Fatalf("expected shared defaults artifact %q to remain absent, got %v", sharedDefaultsPath, err)
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
	items     map[string]metadatadomain.ResourceMetadata
	artifacts map[string]resourcedomain.Content
}

func (f *fakeDefaultsCommandMetadata) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if item, found := f.items[logicalPath]; found {
		return item, nil
	}
	return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
}

func (f *fakeDefaultsCommandMetadata) Set(_ context.Context, logicalPath string, item metadatadomain.ResourceMetadata) error {
	if f.items == nil {
		f.items = map[string]metadatadomain.ResourceMetadata{}
	}
	f.items[logicalPath] = item
	return nil
}

func (f *fakeDefaultsCommandMetadata) Unset(_ context.Context, logicalPath string) error {
	delete(f.items, logicalPath)
	return nil
}

func (f *fakeDefaultsCommandMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if item, found := f.items[logicalPath]; found {
		return item, nil
	}
	return metadatadomain.ResourceMetadata{}, nil
}

func (f *fakeDefaultsCommandMetadata) ReadDefaultsArtifact(_ context.Context, logicalPath string, file string) (resourcedomain.Content, error) {
	if f.artifacts != nil {
		if content, found := f.artifacts[logicalPath+"::"+file]; found {
			return content, nil
		}
	}
	return resourcedomain.Content{}, faults.NewTypedError(faults.NotFoundError, "defaults artifact not found", nil)
}

func (f *fakeDefaultsCommandMetadata) WriteDefaultsArtifact(_ context.Context, logicalPath string, file string, content resourcedomain.Content) error {
	if f.artifacts == nil {
		f.artifacts = map[string]resourcedomain.Content{}
	}
	f.artifacts[logicalPath+"::"+file] = content
	return nil
}

func (f *fakeDefaultsCommandMetadata) DeleteDefaultsArtifact(_ context.Context, logicalPath string, file string) error {
	if f.artifacts != nil {
		delete(f.artifacts, logicalPath+"::"+file)
	}
	return nil
}

func defaultsTestContext(repoDir string, metadataDir string, bundle string) configdomain.Context {
	context := configdomain.Context{
		Name: "defaults-test",
		Repository: configdomain.Repository{
			Filesystem: &configdomain.FilesystemRepository{
				BaseDir: repoDir,
			},
		},
		ManagedServer: &configdomain.ManagedServer{},
	}
	if metadataDir != "" || bundle != "" {
		context.Metadata = configdomain.Metadata{
			BaseDir: metadataDir,
			Bundle:  bundle,
		}
	}
	return context
}
