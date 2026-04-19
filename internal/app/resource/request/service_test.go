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

package request

import (
	"context"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

func TestExecuteRequiresOrchestrator(t *testing.T) {
	t.Parallel()

	_, err := Execute(context.Background(), appdeps.Dependencies{}, Request{Method: "get", LogicalPath: "/items/a"})
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExecuteWithoutResolveTargetsIssuesSingleRequest(t *testing.T) {
	t.Parallel()

	orch := &fakeRequestOrchestrator{
		requestValue: resource.Content{Value: map[string]any{"id": "a"}},
	}

	result, err := Execute(context.Background(), appdeps.Dependencies{Orchestrator: orch}, Request{
		Method:      "patch",
		LogicalPath: "/items/a",
		Headers:     map[string]string{"X-Test": "one"},
		Accept:      "application/json",
		ContentType: "application/json",
		Body:        resource.Content{Value: map[string]any{"name": "updated"}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(orch.requestCalls) != 1 {
		t.Fatalf("expected one request call, got %d", len(orch.requestCalls))
	}
	wantSpec := managedservicedomain.RequestSpec{
		Method:      "PATCH",
		Path:        "/items/a",
		Headers:     map[string]string{"X-Test": "one"},
		Accept:      "application/json",
		ContentType: "application/json",
		Body:        resource.Content{Value: map[string]any{"name": "updated"}},
	}
	if !reflect.DeepEqual(orch.requestCalls[0], wantSpec) {
		t.Fatalf("unexpected request spec: %#v", orch.requestCalls[0])
	}
	if !reflect.DeepEqual(result.Values, []resource.Content{{Value: map[string]any{"id": "a"}}}) {
		t.Fatalf("unexpected result values: %#v", result.Values)
	}
}

func TestExecuteWithResolveTargetsUsesLocalTargets(t *testing.T) {
	t.Parallel()

	orch := &fakeRequestOrchestrator{
		listLocalValue: []resource.Resource{
			{LogicalPath: "/items/b"},
			{LogicalPath: "/items/a"},
		},
		requestByPath: map[string]resource.Content{
			"/items/a": {Value: "first"},
			"/items/b": {Value: "second"},
		},
	}

	result, err := Execute(context.Background(), appdeps.Dependencies{Orchestrator: orch}, Request{
		Method:         "get",
		LogicalPath:    "/items",
		ResolveTargets: true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	gotPaths := []string{orch.requestCalls[0].Path, orch.requestCalls[1].Path}
	if !reflect.DeepEqual(gotPaths, []string{"/items/a", "/items/b"}) {
		t.Fatalf("unexpected request paths: %#v", gotPaths)
	}
	if !reflect.DeepEqual(result.Values, []resource.Content{{Value: "first"}, {Value: "second"}}) {
		t.Fatalf("unexpected result values: %#v", result.Values)
	}
}

func TestExecuteWithResolveTargetsFallsBackToRequestedPath(t *testing.T) {
	t.Parallel()

	orch := &fakeRequestOrchestrator{
		listLocalErr: faults.NotFound("missing", nil),
		requestValue: resource.Content{Value: "fallback"},
	}

	result, err := Execute(context.Background(), appdeps.Dependencies{Orchestrator: orch}, Request{
		Method:         "get",
		LogicalPath:    "/items/a",
		ResolveTargets: true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(orch.requestCalls) != 1 || orch.requestCalls[0].Path != "/items/a" {
		t.Fatalf("expected fallback request for original path, got %#v", orch.requestCalls)
	}
	if !reflect.DeepEqual(result.Values, []resource.Content{{Value: "fallback"}}) {
		t.Fatalf("unexpected fallback result: %#v", result.Values)
	}
}

type fakeRequestOrchestrator struct {
	listLocalValue []resource.Resource
	listLocalErr   error
	requestValue   resource.Content
	requestByPath  map[string]resource.Content
	requestCalls   []managedservicedomain.RequestSpec
}

func (f *fakeRequestOrchestrator) GetLocal(context.Context, string) (resource.Content, error) {
	return resource.Content{}, faults.NotFound("missing", nil)
}

func (f *fakeRequestOrchestrator) ListLocal(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return f.listLocalValue, f.listLocalErr
}

func (f *fakeRequestOrchestrator) GetRemote(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}

func (f *fakeRequestOrchestrator) ListRemote(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}

func (f *fakeRequestOrchestrator) GetOpenAPISpec(context.Context) (resource.Content, error) {
	return resource.Content{}, nil
}

func (f *fakeRequestOrchestrator) Request(_ context.Context, spec managedservicedomain.RequestSpec) (resource.Content, error) {
	f.requestCalls = append(f.requestCalls, spec)
	if f.requestByPath != nil {
		if value, ok := f.requestByPath[spec.Path]; ok {
			return value, nil
		}
	}
	return f.requestValue, nil
}

func (f *fakeRequestOrchestrator) Save(context.Context, string, resource.Content) error { return nil }

func (f *fakeRequestOrchestrator) Apply(context.Context, string, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (f *fakeRequestOrchestrator) ApplyWithContent(context.Context, string, resource.Content, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (f *fakeRequestOrchestrator) Create(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (f *fakeRequestOrchestrator) Update(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (f *fakeRequestOrchestrator) Delete(context.Context, string, orchestratordomain.DeletePolicy) error {
	return nil
}

func (f *fakeRequestOrchestrator) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}

func (f *fakeRequestOrchestrator) Template(context.Context, string, resource.Content) (resource.Content, error) {
	return resource.Content{}, nil
}
