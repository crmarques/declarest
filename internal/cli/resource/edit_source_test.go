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
	"context"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	resourcedomain "github.com/crmarques/declarest/resource"
)

func TestResolveEditSourceUsesCanonicalLocalPath(t *testing.T) {
	t.Parallel()

	orch := &fakeEditSourceOrchestrator{
		resolvedLocal: resourcedomain.Resource{
			LogicalPath: "/projects/canonical-test",
			Payload: map[string]any{
				"name": "test",
			},
		},
	}

	resolvedPath, value, err := resolveEditSource(context.Background(), cliutil.CommandDependencies{
		Orchestrator: orch,
	}, "/projects/test")
	if err != nil {
		t.Fatalf("resolveEditSource returned error: %v", err)
	}
	if resolvedPath != "/projects/canonical-test" {
		t.Fatalf("expected canonical local path, got %q", resolvedPath)
	}
	if !reflect.DeepEqual(value.Value, orch.resolvedLocal.Payload) {
		t.Fatalf("expected local payload %#v, got %#v", orch.resolvedLocal.Payload, value.Value)
	}
	if want := []string{"/projects/test"}; !reflect.DeepEqual(orch.resolveLocalCalls, want) {
		t.Fatalf("expected local resolution calls %#v, got %#v", want, orch.resolveLocalCalls)
	}
	if len(orch.remoteCalls) != 0 {
		t.Fatalf("expected no remote fallback, got %#v", orch.remoteCalls)
	}
}

func TestResolveEditSourceFallsBackToRemoteOnLocalMiss(t *testing.T) {
	t.Parallel()

	orch := &fakeEditSourceOrchestrator{
		resolveLocalErr: faults.NotFound("not found", nil),
		remoteValue: map[string]any{
			"name": "test",
		},
	}

	resolvedPath, value, err := resolveEditSource(context.Background(), cliutil.CommandDependencies{
		Orchestrator: orch,
	}, "/projects/test")
	if err != nil {
		t.Fatalf("resolveEditSource returned error: %v", err)
	}
	if resolvedPath != "/projects/test" {
		t.Fatalf("expected requested path after remote bootstrap, got %q", resolvedPath)
	}
	if !reflect.DeepEqual(value.Value, orch.remoteValue) {
		t.Fatalf("expected remote payload %#v, got %#v", orch.remoteValue, value.Value)
	}
	if want := []string{"/projects/test"}; !reflect.DeepEqual(orch.resolveLocalCalls, want) {
		t.Fatalf("expected local resolution calls %#v, got %#v", want, orch.resolveLocalCalls)
	}
	if want := []string{"/projects/test"}; !reflect.DeepEqual(orch.remoteCalls, want) {
		t.Fatalf("expected remote fallback calls %#v, got %#v", want, orch.remoteCalls)
	}
}

func TestResolveEditSourcePreservesLocalResolutionErrors(t *testing.T) {
	t.Parallel()

	expectedErr := faults.Conflict("ambiguous local fallback", nil)
	orch := &fakeEditSourceOrchestrator{
		resolveLocalErr: expectedErr,
	}

	_, _, err := resolveEditSource(context.Background(), cliutil.CommandDependencies{
		Orchestrator: orch,
	}, "/projects/test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !faults.IsCategory(err, faults.ConflictError) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if len(orch.remoteCalls) != 0 {
		t.Fatalf("expected no remote fallback, got %#v", orch.remoteCalls)
	}
}

type fakeEditSourceOrchestrator struct {
	orchestratordomain.Orchestrator
	resolvedLocal     resourcedomain.Resource
	resolveLocalErr   error
	resolveLocalCalls []string
	remoteValue       resourcedomain.Value
	remoteErr         error
	remoteCalls       []string
}

func (f *fakeEditSourceOrchestrator) ResolveLocalResource(
	_ context.Context,
	logicalPath string,
) (resourcedomain.Resource, error) {
	f.resolveLocalCalls = append(f.resolveLocalCalls, logicalPath)
	if f.resolveLocalErr != nil {
		return resourcedomain.Resource{}, f.resolveLocalErr
	}
	return f.resolvedLocal, nil
}

func (f *fakeEditSourceOrchestrator) GetRemote(_ context.Context, logicalPath string) (resourcedomain.Content, error) {
	f.remoteCalls = append(f.remoteCalls, logicalPath)
	if f.remoteErr != nil {
		return resourcedomain.Content{}, f.remoteErr
	}
	return resourcedomain.Content{
		Value:      f.remoteValue,
		Descriptor: resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{PayloadType: resourcedomain.PayloadTypeJSON}),
	}, nil
}
