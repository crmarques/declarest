package defaults

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestGetReturnsEmptyObjectWhenDefaultsSidecarIsMissing(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Get(context.Background(), deps, "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	want := map[string]any{}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected defaults payload: got %#v want %#v", result.Content.Value, want)
	}
	if result.Content.Descriptor.PayloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json defaults descriptor, got %#v", result.Content.Descriptor)
	}
}

func TestInferFromRepositoryUsesCommonSiblingValues(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServerCreatesAndDeletesTemporaryResources(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/acme": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}

	if len(orch.createCalls) != 2 {
		t.Fatalf("expected two temporary creates, got %#v", orch.createCalls)
	}
	if len(orch.deleteCalls) != 2 {
		t.Fatalf("expected two cleanup deletes, got %#v", orch.deleteCalls)
	}
	if orch.deleteCalls[0] != orch.createCalls[1].logicalPath || orch.deleteCalls[1] != orch.createCalls[0].logicalPath {
		t.Fatalf("expected cleanup deletes in reverse order, got creates=%#v deletes=%#v", orch.createCalls, orch.deleteCalls)
	}
}

func TestInferAcceptsCollectionAndResourcePathsForSameCollection(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"id":          "acme-id",
				"realm":       "acme",
				"enabled":     true,
				"sslRequired": "external",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"id":          "master-id",
				"realm":       "master",
				"enabled":     true,
				"sslRequired": "external",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	tests := []struct {
		name             string
		requestedPath    string
		wantResolvedPath string
	}{
		{name: "collection_without_trailing_slash", requestedPath: "/admin/realms", wantResolvedPath: "/admin/realms/acme"},
		{name: "collection_with_trailing_slash", requestedPath: "/admin/realms/", wantResolvedPath: "/admin/realms/acme"},
		{name: "specific_resource", requestedPath: "/admin/realms/master", wantResolvedPath: "/admin/realms/master"},
	}

	want := map[string]any{
		"enabled":     true,
		"sslRequired": "external",
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Infer(context.Background(), deps, tc.requestedPath, InferRequest{})
			if err != nil {
				t.Fatalf("Infer returned error: %v", err)
			}
			if result.ResolvedPath != tc.wantResolvedPath {
				t.Fatalf("expected resolved path %q, got %q", tc.wantResolvedPath, result.ResolvedPath)
			}
			if !reflect.DeepEqual(result.Content.Value, want) {
				t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
			}
		})
	}
}

func TestInferManagedServerAcceptsCollectionPathWithOrWithoutTrailingSlash(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "acme",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "master",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)

	tests := []struct {
		name             string
		requestedPath    string
		wantResolvedPath string
	}{
		{name: "collection_without_trailing_slash", requestedPath: "/admin/realms", wantResolvedPath: "/admin/realms/acme"},
		{name: "collection_with_trailing_slash", requestedPath: "/admin/realms/", wantResolvedPath: "/admin/realms/acme"},
		{name: "specific_resource", requestedPath: "/admin/realms/master", wantResolvedPath: "/admin/realms/master"},
	}

	want := map[string]any{"status": "active"}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Infer(context.Background(), deps, tc.requestedPath, InferRequest{ManagedServer: true})
			if err != nil {
				t.Fatalf("Infer returned error: %v", err)
			}
			if result.ResolvedPath != tc.wantResolvedPath {
				t.Fatalf("expected resolved path %q, got %q", tc.wantResolvedPath, result.ResolvedPath)
			}
			if !reflect.DeepEqual(result.Content.Value, want) {
				t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
			}
		})
	}

	if len(orch.createCalls) != len(tests)*2 {
		t.Fatalf("expected %d temporary creates, got %#v", len(tests)*2, orch.createCalls)
	}
	for idx, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for create %d (%q), got %#v", tempName, idx, call.logicalPath, got)
		}
	}
}

func TestInferManagedServerRewritesCollectionIdentityFieldWhenMetadataMissing(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "acme",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)

	result, err := Infer(context.Background(), deps, "/admin/realms/acme", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if result.ResolvedPath != "/admin/realms/acme" {
		t.Fatalf("expected resolved path /admin/realms/acme, got %q", result.ResolvedPath)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}

	if len(orch.createCalls) != 2 {
		t.Fatalf("expected two temporary creates, got %#v", orch.createCalls)
	}
	for _, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for %q, got %#v", tempName, call.logicalPath, got)
		}
		if got := payload["displayName"]; got != "acme" {
			t.Fatalf("expected displayName to remain unchanged, got %#v", got)
		}
	}
}

func TestInferFromManagedServerIgnoresStoredDefaultsSidecarValues(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":     "acme",
				"name":   "acme",
				"status": "active",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/acme"] = resource.Content{
		Value: map[string]any{
			"status": "active",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferManagedServerRetriesCleanupDeleteAfterAuthError(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/acme": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.deleteErr = faults.NewTypedError(faults.AuthError, "remote request failed with status 403: forbidden", nil)

	managedServerClient := &fakeDefaultsManagedServerClient{}
	serviceAccessor := deps.Services.(*fakeDefaultsServiceAccessor)
	serviceAccessor.managedServer = managedServerClient

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
	if len(orch.deleteCalls) != 2 {
		t.Fatalf("expected two orchestrator delete attempts, got %#v", orch.deleteCalls)
	}
	if managedServerClient.invalidateCalls != 3 {
		t.Fatalf("expected one probe-read invalidation plus two delete-retry invalidations, got %d", managedServerClient.invalidateCalls)
	}
	if len(managedServerClient.requestCalls) != 2 {
		t.Fatalf("expected two direct managed-server delete retries, got %#v", managedServerClient.requestCalls)
	}
	for _, call := range managedServerClient.requestCalls {
		if call.Method != "DELETE" {
			t.Fatalf("expected DELETE retry request, got %#v", call)
		}
		if !strings.HasPrefix(call.Path, "/customers/declarest-defaults-probe-") {
			t.Fatalf("unexpected retry path %q", call.Path)
		}
	}
}

func TestInferFromManagedServerWaitsForStableProbeRead(t *testing.T) {
	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, call int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		if call >= 3 {
			payload["tier"] = "standard"
		}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
		"tier":   "standard",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServerWaitsBeforeFirstProbeRead(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	wait := 25 * time.Millisecond

	result, err := Infer(context.Background(), deps, "/projects/platform", InferRequest{
		ManagedServer: true,
		Wait:          wait,
	})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if orch.lastCreateAt.IsZero() {
		t.Fatal("expected create timestamp to be recorded")
	}
	if orch.firstGetRemoteAt.IsZero() {
		t.Fatal("expected first remote read timestamp to be recorded")
	}
	if delay := orch.firstGetRemoteAt.Sub(orch.lastCreateAt); delay < wait {
		t.Fatalf("expected first remote read delay >= %s, got %s", wait, delay)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServerIncludesSharedEmptyObjectDefaults(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, _ int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["smtpServer"] = map[string]any{}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"smtpServer": map[string]any{},
		"status":     "active",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServerInvalidatesAuthCacheBeforeProbeRead(t *testing.T) {
	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	managedServerClient := &fakeDefaultsManagedServerClient{}
	serviceAccessor := deps.Services.(*fakeDefaultsServiceAccessor)
	serviceAccessor.managedServer = managedServerClient

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, _ int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		if managedServerClient.invalidateCalls > 0 {
			payload["tier"] = "standard"
		}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
		"tier":   "standard",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}
	if managedServerClient.invalidateCalls != 1 {
		t.Fatalf("expected one auth cache invalidation before probe reads, got %d", managedServerClient.invalidateCalls)
	}
}

func TestInferCollectionPathWithMultipleDirectChildrenUsesSharedDefaults(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Infer(context.Background(), deps, "/customers/", InferRequest{})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{"team": "platform"},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestCompactContentAgainstStoredDefaultsReturnsOnlyOverrides(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/acme"] = resource.Content{
		Value: map[string]any{
			"status": "active",
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	content, pruned, err := CompactContentAgainstStoredDefaults(context.Background(), deps, "/customers/acme", resource.Content{
		Value: map[string]any{
			"id":     "acme",
			"name":   "acme",
			"status": "active",
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	})
	if err != nil {
		t.Fatalf("CompactContentAgainstStoredDefaults returned error: %v", err)
	}
	if !pruned {
		t.Fatal("expected defaults pruning to be applied")
	}

	want := map[string]any{
		"id":   "acme",
		"name": "acme",
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected pruned payload: got %#v want %#v", content.Value, want)
	}
}

func TestCompactContentAgainstStoredDefaultsRemovesSharedEmptyObjects(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/acme"] = resource.Content{
		Value: map[string]any{
			"smtpServer": map[string]any{},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	content, pruned, err := CompactContentAgainstStoredDefaults(context.Background(), deps, "/customers/acme", resource.Content{
		Value: map[string]any{
			"name":       "acme",
			"smtpServer": map[string]any{},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	})
	if err != nil {
		t.Fatalf("CompactContentAgainstStoredDefaults returned error: %v", err)
	}
	if !pruned {
		t.Fatal("expected defaults pruning to be applied")
	}

	want := map[string]any{
		"name": "acme",
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected pruned payload: got %#v want %#v", content.Value, want)
	}
}

func TestCheckMatchesStoredDefaultsWhenInferredDefaultsAreEqual(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/acme"] = resource.Content{
		Value: map[string]any{
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	result, err := Check(context.Background(), deps, "/customers/acme", CheckRequest{})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !result.Matches {
		t.Fatalf("expected matching defaults, got %#v vs %#v", result.CurrentContent.Value, result.InferredContent.Value)
	}
}

func TestCheckDetectsMismatchAgainstManagedServerInference(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/acme"] = resource.Content{
		Value: map[string]any{
			"status": "inactive",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/acme": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	result, err := Check(context.Background(), deps, "/customers/acme", CheckRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Matches {
		t.Fatalf("expected mismatching defaults, got %#v vs %#v", result.CurrentContent.Value, result.InferredContent.Value)
	}
	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.InferredContent.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.InferredContent.Value, want)
	}
}

type fakeDefaultsOrchestrator struct {
	orchestratordomain.Orchestrator
	localContent     map[string]resource.Content
	createCalls      []savedResource
	deleteCalls      []string
	deleteErr        error
	getRemoteFn      func(item savedResource, call int) (resource.Content, error)
	getCalls         map[string]int
	lastCreateAt     time.Time
	firstGetRemoteAt time.Time
}

type savedResource struct {
	logicalPath string
	content     resource.Content
}

func (f *fakeDefaultsOrchestrator) ResolveLocalResource(_ context.Context, logicalPath string) (resource.Resource, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Resource{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return resource.Resource{
		LogicalPath:       logicalPath,
		CollectionPath:    collectionPathFor(logicalPath),
		Payload:           content.Value,
		PayloadDescriptor: content.Descriptor,
	}, nil
}

func (f *fakeDefaultsOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return content, nil
}

func (f *fakeDefaultsOrchestrator) ListLocal(_ context.Context, logicalPath string, _ orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0, len(f.localContent))
	for candidate := range f.localContent {
		if collectionPathFor(candidate) != logicalPath {
			continue
		}
		items = append(items, resource.Resource{LogicalPath: candidate})
	}
	return items, nil
}

func (f *fakeDefaultsOrchestrator) Create(_ context.Context, logicalPath string, content resource.Content) (resource.Resource, error) {
	f.createCalls = append(f.createCalls, savedResource{logicalPath: logicalPath, content: content})
	f.lastCreateAt = time.Now()
	return resource.Resource{LogicalPath: logicalPath}, nil
}

func (f *fakeDefaultsOrchestrator) GetRemote(_ context.Context, logicalPath string) (resource.Content, error) {
	if f.firstGetRemoteAt.IsZero() {
		f.firstGetRemoteAt = time.Now()
	}
	for _, item := range f.createCalls {
		if item.logicalPath != logicalPath {
			continue
		}
		if f.getCalls == nil {
			f.getCalls = map[string]int{}
		}
		f.getCalls[logicalPath]++
		if f.getRemoteFn != nil {
			return f.getRemoteFn(item, f.getCalls[logicalPath])
		}
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["id"] = path.Base(logicalPath)
		payload["name"] = path.Base(logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}
	return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
}

func (f *fakeDefaultsOrchestrator) Delete(_ context.Context, logicalPath string, _ orchestratordomain.DeletePolicy) error {
	f.deleteCalls = append(f.deleteCalls, logicalPath)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

type fakeDefaultsManagedServerClient struct {
	managedserver.ManagedServerClient
	invalidateCalls int
	requestCalls    []managedserver.RequestSpec
	requestErr      error
}

func (f *fakeDefaultsManagedServerClient) InvalidateAuthCache() {
	f.invalidateCalls++
}

func (f *fakeDefaultsManagedServerClient) Request(
	_ context.Context,
	spec managedserver.RequestSpec,
) (resource.Content, error) {
	f.requestCalls = append(f.requestCalls, spec)
	if f.requestErr != nil {
		return resource.Content{}, f.requestErr
	}
	return resource.Content{}, nil
}

type fakeDefaultsRepository struct {
	repository.ResourceStore
	defaults map[string]resource.Content
}

func (f *fakeDefaultsRepository) Save(context.Context, string, resource.Content) error { return nil }
func (f *fakeDefaultsRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	return resource.Content{}, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
}
func (f *fakeDefaultsRepository) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (f *fakeDefaultsRepository) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (f *fakeDefaultsRepository) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeDefaultsRepository) GetDefaults(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.defaults[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "defaults not found", nil)
	}
	return content, nil
}
func (f *fakeDefaultsRepository) SaveDefaults(_ context.Context, logicalPath string, content resource.Content) error {
	if f.defaults == nil {
		f.defaults = map[string]resource.Content{}
	}
	f.defaults[logicalPath] = content
	return nil
}

type fakeDefaultsMetadata struct {
	metadata.MetadataService
	items map[string]metadata.ResourceMetadata
}

func (f *fakeDefaultsMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadata.ResourceMetadata, error) {
	if item, found := f.items[logicalPath]; found {
		return item, nil
	}
	return metadata.ResourceMetadata{}, nil
}

type fakeDefaultsServiceAccessor struct {
	store         repository.ResourceStore
	metadata      metadata.MetadataService
	secrets       secretdomain.SecretProvider
	managedServer managedserver.ManagedServerClient
}

func (f *fakeDefaultsServiceAccessor) RepositoryStore() repository.ResourceStore   { return f.store }
func (f *fakeDefaultsServiceAccessor) RepositorySync() repository.RepositorySync   { return nil }
func (f *fakeDefaultsServiceAccessor) MetadataService() metadata.MetadataService   { return f.metadata }
func (f *fakeDefaultsServiceAccessor) SecretProvider() secretdomain.SecretProvider { return f.secrets }
func (f *fakeDefaultsServiceAccessor) ManagedServerClient() managedserver.ManagedServerClient {
	return f.managedServer
}

func testDefaultsDeps() appdeps.Dependencies {
	return testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":     "acme",
				"name":   "acme",
				"status": "custom-a",
				"labels": map[string]any{"team": "platform"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/customers/beta": {
			Value: map[string]any{
				"id":     "beta",
				"name":   "beta",
				"status": "custom-b",
				"labels": map[string]any{"team": "platform"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
}

func testDefaultsDepsWithLocalContent(localContent map[string]resource.Content) appdeps.Dependencies {
	orch := &fakeDefaultsOrchestrator{
		localContent: localContent,
	}
	repo := &fakeDefaultsRepository{defaults: map[string]resource.Content{}}
	md := &fakeDefaultsMetadata{items: map[string]metadata.ResourceMetadata{}}

	return appdeps.Dependencies{
		Orchestrator: orch,
		Repository:   repo,
		Metadata:     md,
		Services: &fakeDefaultsServiceAccessor{
			store:    repo,
			metadata: md,
		},
	}
}
