// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"bytes"
	"context"
	"errors"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	resourcedomain "github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

type editCommandSaveCall struct {
	logicalPath string
	value       resourcedomain.Content
}

func TestEditCommandSavesUsingCanonicalLocalPath(t *testing.T) {
	previousEditTempFile := editTempFile
	editTempFile = func(*cobra.Command, string, string, []byte) ([]byte, error) {
		return []byte(`{"name":"updated"}`), nil
	}
	defer func() {
		editTempFile = previousEditTempFile
	}()

	orch := &fakeEditCommandOrchestrator{
		resolvedLocal: resourcedomain.Resource{
			LogicalPath: "/projects/canonical-test",
			Payload: map[string]any{
				"name": "test",
			},
		},
	}

	command := newEditCommand(cliutil.CommandDependencies{
		Orchestrator: orch,
		Contexts:     fakeEditContextService{context: editTestContext()},
		Services: &fakeEditServiceAccessor{
			store:    &fakeEditRepositoryStore{},
			metadata: fakeEditMetadataService{},
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/projects/test"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if len(orch.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %#v", orch.saveCalls)
	}
	if got := orch.saveCalls[0].logicalPath; got != "/projects/canonical-test" {
		t.Fatalf("expected canonical save path, got %q", got)
	}
	if len(orch.remoteCalls) != 0 {
		t.Fatalf("expected no remote fallback, got %#v", orch.remoteCalls)
	}
}

func TestEditCommandRejectsBinaryPayloads(t *testing.T) {
	previousEditTempFile := editTempFile
	editTempFile = func(*cobra.Command, string, string, []byte) ([]byte, error) {
		t.Fatal("editTempFile should not be called for binary payloads")
		return nil, nil
	}
	defer func() {
		editTempFile = previousEditTempFile
	}()

	orch := &fakeEditCommandOrchestrator{
		resolvedLocal: resourcedomain.Resource{
			LogicalPath: "/projects/binary-test",
			Payload:     resourcedomain.BinaryValue{Bytes: []byte("abc")},
		},
	}

	command := newEditCommand(cliutil.CommandDependencies{
		Orchestrator: orch,
		Contexts:     fakeEditContextService{context: editTestContext()},
		Services: &fakeEditServiceAccessor{
			store:    &fakeEditRepositoryStore{},
			metadata: fakeEditMetadataService{},
		},
	}, &cliutil.GlobalFlags{})
	command.SetArgs([]string{"/projects/binary-test"})
	command.SetIn(bytes.NewBuffer(nil))
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.ExecuteContext(context.Background())
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("octet-stream")) {
		t.Fatalf("expected octet-stream validation message, got %v", err)
	}
}

type fakeEditCommandOrchestrator struct {
	orchestratordomain.Orchestrator
	resolvedLocal   resourcedomain.Resource
	resolveLocalErr error
	remoteValue     resourcedomain.Value
	remoteErr       error
	remoteCalls     []string
	saveCalls       []editCommandSaveCall
}

func (f *fakeEditCommandOrchestrator) ResolveLocalResource(
	_ context.Context,
	_ string,
) (resourcedomain.Resource, error) {
	if f.resolveLocalErr != nil {
		return resourcedomain.Resource{}, f.resolveLocalErr
	}
	if f.resolvedLocal.LogicalPath == "" {
		return resourcedomain.Resource{}, faults.NewTypedError(
			faults.NotFoundError,
			"not found",
			nil,
		)
	}
	return f.resolvedLocal, nil
}

func (f *fakeEditCommandOrchestrator) GetRemote(_ context.Context, logicalPath string) (resourcedomain.Content, error) {
	f.remoteCalls = append(f.remoteCalls, logicalPath)
	if f.remoteErr != nil {
		return resourcedomain.Content{}, f.remoteErr
	}
	return resourcedomain.Content{
		Value:      f.remoteValue,
		Descriptor: resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON}),
	}, nil
}

func (f *fakeEditCommandOrchestrator) Save(_ context.Context, logicalPath string, value resourcedomain.Content) error {
	f.saveCalls = append(f.saveCalls, editCommandSaveCall{
		logicalPath: logicalPath,
		value:       value,
	})
	return nil
}

type fakeEditRepositoryStore struct {
	repository.ResourceStore
}

type fakeEditContextService struct {
	configdomain.ContextService
	context configdomain.Context
}

func (f fakeEditContextService) ResolveContext(context.Context, configdomain.ContextSelection) (configdomain.Context, error) {
	if f.context.Name == "" {
		return configdomain.Context{}, errors.New("missing context")
	}
	return f.context, nil
}

type fakeEditMetadataService struct {
	metadatadomain.MetadataService
}

func (fakeEditMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (fakeEditMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (fakeEditMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (fakeEditMetadataService) Unset(context.Context, string) error {
	return nil
}

func (fakeEditMetadataService) ReadDefaultsArtifact(context.Context, string, string) (resourcedomain.Content, error) {
	return resourcedomain.Content{}, faults.NotFound("defaults artifact not found", nil)
}

func (fakeEditMetadataService) WriteDefaultsArtifact(context.Context, string, string, resourcedomain.Content) error {
	return nil
}

func (fakeEditMetadataService) DeleteDefaultsArtifact(context.Context, string, string) error {
	return nil
}

type fakeEditServiceAccessor struct {
	store    repository.ResourceStore
	metadata metadatadomain.MetadataService
}

func (a *fakeEditServiceAccessor) RepositoryStore() repository.ResourceStore       { return a.store }
func (a *fakeEditServiceAccessor) RepositorySync() repository.RepositorySync       { return nil }
func (a *fakeEditServiceAccessor) MetadataService() metadatadomain.MetadataService { return a.metadata }
func (a *fakeEditServiceAccessor) SecretProvider() secretdomain.SecretProvider     { return nil }
func (a *fakeEditServiceAccessor) ManagedServiceClient() managedservicedomain.ManagedServiceClient {
	return nil
}

func editTestContext() configdomain.Context {
	return configdomain.Context{
		Name: "edit-test",
		Repository: configdomain.Repository{
			Filesystem: &configdomain.FilesystemRepository{
				BaseDir: "/tmp",
			},
		},
		ManagedService: &configdomain.ManagedService{},
	}
}
