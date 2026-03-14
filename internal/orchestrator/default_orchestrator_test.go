package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	managedserverhttp "github.com/crmarques/declarest/internal/providers/managedserver/http"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orch "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func jqMutation(expression string) []metadatadomain.TransformStep {
	return []metadatadomain.TransformStep{{JQExpression: expression}}
}

func suppressMutation(attributes ...string) []metadatadomain.TransformStep {
	return []metadatadomain.TransformStep{{ExcludeAttributes: attributes}}
}

func TestOrchestratorDelegatesRepositoryMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepository{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}

	orchestrator := &Orchestrator{
		repository: fakeRepo,
	}

	value, err := orchestrator.GetLocal(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}
	if value.Value == nil {
		t.Fatal("expected non-nil value")
	}

	if err := orchestrator.Save(context.Background(), "/customers/acme", testContent(map[string]any{"x": 1})); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	items, err := orchestrator.ListLocal(context.Background(), "/customers", orch.ListPolicy{Recursive: true})
	if err != nil {
		t.Fatalf("ListLocal returned error: %v", err)
	}
	if len(items) != 1 || items[0].LogicalPath != "/customers/acme" {
		t.Fatalf("unexpected list output: %#v", items)
	}
	payload, ok := items[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected list item payload map, got %T", items[0].Payload)
	}
	if payload["id"] != int64(1) {
		t.Fatalf("expected hydrated payload id=1, got %#v", payload["id"])
	}

	if !fakeRepo.listPolicy.Recursive {
		t.Fatal("expected list policy recursion to be mapped")
	}
}

func TestOrchestratorSaveExternalizesConfiguredAttributes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{
						Path: "/script",
						File: "script.sh",
					},
				},
			},
		},
	}

	err := orchestrator.Save(context.Background(), "/customers/acme", testContent(map[string]any{
		"name":   "ACME",
		"script": "echo hello",
	}))
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	wantPayload := map[string]any{
		"name":   "ACME",
		"script": "{{include script.sh}}",
	}
	if !reflect.DeepEqual(wantPayload, repo.savedValue) {
		t.Fatalf("unexpected saved payload %#v", repo.savedValue)
	}
	if len(repo.savedArtifacts) != 1 {
		t.Fatalf("expected one saved artifact, got %#v", repo.savedArtifacts)
	}
	if repo.savedArtifacts[0].File != "script.sh" || string(repo.savedArtifacts[0].Content) != "echo hello" {
		t.Fatalf("unexpected saved artifact %#v", repo.savedArtifacts[0])
	}
}

func TestOrchestratorSaveExternalizesWildcardArrayAttributes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{
						Path: "/sequence/commands/*/script",
						File: "script.sh",
					},
				},
			},
		},
	}

	err := orchestrator.Save(context.Background(), "/customers/acme", testContent(map[string]any{
		"name": "ACME",
		"sequence": map[string]any{
			"commands": []any{
				map[string]any{"script": "echo first"},
				map[string]any{"exec": "echo inline"},
				map[string]any{"script": "echo third"},
			},
		},
	}))
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	wantPayload := map[string]any{
		"name": "ACME",
		"sequence": map[string]any{
			"commands": []any{
				map[string]any{"script": "{{include script-0.sh}}"},
				map[string]any{"exec": "echo inline"},
				map[string]any{"script": "{{include script-2.sh}}"},
			},
		},
	}
	if !reflect.DeepEqual(wantPayload, repo.savedValue) {
		t.Fatalf("unexpected saved payload %#v", repo.savedValue)
	}

	wantArtifacts := []repository.ResourceArtifact{
		{File: "script-0.sh", Content: []byte("echo first")},
		{File: "script-2.sh", Content: []byte("echo third")},
	}
	if !reflect.DeepEqual(wantArtifacts, repo.savedArtifacts) {
		t.Fatalf("unexpected saved artifacts %#v", repo.savedArtifacts)
	}
}

func TestOrchestratorSaveAppliesDefaultFormatBeforePersisting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		metadataFormat      string
		content             resource.Content
		expectedPayloadType string
	}{
		{
			name:                "metadata format used for implicit descriptor",
			metadataFormat:      resource.PayloadTypeYAML,
			content:             resource.Content{Value: map[string]any{"name": "ACME"}},
			expectedPayloadType: resource.PayloadTypeYAML,
		},
		{
			name:                "explicit descriptor remains unchanged",
			metadataFormat:      resource.PayloadTypeYAML,
			content:             testContentWithType(map[string]any{"name": "ACME"}, resource.PayloadTypeJSON),
			expectedPayloadType: resource.PayloadTypeJSON,
		},
		{
			name:                "metadata any leaves descriptor unset",
			metadataFormat:      metadatadomain.ResourceFormatAny,
			content:             resource.Content{Value: map[string]any{"name": "ACME"}},
			expectedPayloadType: "",
		},
		{
			name:                "empty metadata leaves descriptor unset",
			content:             resource.Content{Value: map[string]any{"name": "ACME"}},
			expectedPayloadType: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			orchestrator := New(
				repo,
				&fakeMetadata{resolveValue: metadatadomain.ResourceMetadata{Format: tc.metadataFormat}},
				nil,
				nil,
			)

			if err := orchestrator.Save(context.Background(), "/customers/acme", tc.content); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}
			if repo.savedDescriptor.PayloadType != tc.expectedPayloadType {
				t.Fatalf("expected saved payload type %q, got %q", tc.expectedPayloadType, repo.savedDescriptor.PayloadType)
			}
		})
	}
}

func TestOrchestratorDiffUsesResolvedMetadataDefaultsPayload(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeOrchestratorTestFile(t, filepath.Join(repoDir, "customers", "_", "defaults.yaml"), `
id: acme
spec:
  enabled: true
  tags:
    - default
`)
	writeOrchestratorTestFile(t, filepath.Join(repoDir, "customers", "acme", "resource.yaml"), `
spec:
  enabled: false
`)

	repo := fsstore.NewLocalResourceRepository(repoDir)
	metadataService := fsmetadata.NewFSMetadataService(repoDir)
	if err := metadataService.Set(context.Background(), "/customers/", metadatadomain.ResourceMetadata{
		Defaults: &metadatadomain.DefaultsSpec{
			Value: metadatadomain.DefaultsIncludePlaceholder("defaults.yaml"),
		},
	}); err != nil {
		t.Fatalf("metadata set returned error: %v", err)
	}
	serverManager := &fakeServer{
		getValue: map[string]any{
			"id": "acme",
			"spec": map[string]any{
				"enabled": false,
				"tags":    []any{"default"},
			},
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	local, err := orchestrator.GetLocal(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}
	wantLocal := map[string]any{
		"id": "acme",
		"spec": map[string]any{
			"enabled": false,
			"tags":    []any{"default"},
		},
	}
	if !reflect.DeepEqual(local.Value, wantLocal) {
		t.Fatalf("unexpected merged local payload: got %#v want %#v", local.Value, wantLocal)
	}

	items, err := orchestrator.Diff(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no diff when remote matches merged local payload, got %#v", items)
	}
}

func TestOrchestratorDeleteDelegatesToServer(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	if err := orchestrator.Delete(context.Background(), "/customers/acme", orch.DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !serverManager.deleteCalled {
		t.Fatal("expected delete call to be delegated to server")
	}
	if got := serverManager.lastResource.LogicalPath; got != "/customers/acme" {
		t.Fatalf("expected normalized delete logical path /customers/acme, got %q", got)
	}
}

func TestOrchestratorRequiresRepository(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}

	_, err := orchestrator.GetLocal(context.Background(), "/customers/acme")
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.ValidationError)
}

func TestOrchestratorGetRemoteNormalizesCollectionPath(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveErr: faults.NewTypedError(faults.NotFoundError, "metadata not found", nil),
		},
		server: &fakeServer{
			getValue: map[string]any{"realm": "master"},
		},
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	serverManager := orchestrator.server.(*fakeServer)
	if !serverManager.getCalled {
		t.Fatal("expected remote get call")
	}
	if got := serverManager.lastResource.LogicalPath; got != "/admin/realms" {
		t.Fatalf("expected normalized remote logical path /admin/realms, got %q", got)
	}
	if got := serverManager.lastResource.CollectionPath; got != "/admin" {
		t.Fatalf("expected remote collection path /admin, got %q", got)
	}
	if !reflect.DeepEqual(value.Value, map[string]any{"realm": "master"}) {
		t.Fatalf("unexpected remote payload: %#v", value.Value)
	}
}

func TestOrchestratorApplyExpandsExternalizedAttributesFromRepository(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"name":   "ACME",
			"script": "{{include script.sh}}",
		},
		artifactFiles: map[string][]byte{
			"/customers/acme::script.sh": []byte("echo hello"),
		},
	}
	server := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
	}
	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{
						Path: "/script",
						File: "script.sh",
					},
				},
			},
		},
		server: server,
	}

	_, err := orchestrator.Apply(context.Background(), "/customers/acme", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	payload, ok := server.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected create payload map, got %T", server.lastResource.Payload)
	}
	if got := payload["script"]; got != "echo hello" {
		t.Fatalf("expected expanded script payload, got %#v", got)
	}
}

func TestOrchestratorApplyExpandsWildcardArrayExternalizedAttributes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"name": "ACME",
			"sequence": map[string]any{
				"commands": []any{
					map[string]any{"script": "{{include script-0.sh}}"},
					map[string]any{"exec": "echo inline"},
					map[string]any{"script": "{{include script-2.sh}}"},
				},
			},
		},
		artifactFiles: map[string][]byte{
			"/customers/acme::script-0.sh": []byte("echo first"),
			"/customers/acme::script-2.sh": []byte("echo third"),
		},
	}
	server := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
	}
	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{
						Path: "/sequence/commands/*/script",
						File: "script.sh",
					},
				},
			},
		},
		server: server,
	}

	_, err := orchestrator.Apply(context.Background(), "/customers/acme", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	payload, ok := server.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected create payload map, got %T", server.lastResource.Payload)
	}

	wantPayload := map[string]any{
		"name": "ACME",
		"sequence": map[string]any{
			"commands": []any{
				map[string]any{"script": "echo first"},
				map[string]any{"exec": "echo inline"},
				map[string]any{"script": "echo third"},
			},
		},
	}
	if !reflect.DeepEqual(wantPayload, payload) {
		t.Fatalf("unexpected expanded payload %#v", payload)
	}
}

func TestOrchestratorGetRemoteSeedsIdentityFromMetadata(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/realm}}",
				Alias: "{{/realm}}",
			},
		},
		server: &fakeServer{
			getValue: map[string]any{"realm": "platform"},
		},
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/platform")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	serverManager := orchestrator.server.(*fakeServer)
	if got := serverManager.lastResource.RemoteID; got != "platform" {
		t.Fatalf("expected remote id platform, got %q", got)
	}
}

func TestOrchestratorGetRemoteFallsBackToCollectionListByAlias(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
		server: &fakeServer{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			listValue: []resource.Resource{
				{
					LogicalPath: "/admin/realms/master/clients/account",
					LocalAlias:  "account",
					RemoteID:    "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
					Payload: map[string]any{
						"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
						"clientId": "account",
					},
				},
			},
		},
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/clients/account")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	serverManager := orchestrator.server.(*fakeServer)
	if !serverManager.listCalled {
		t.Fatal("expected fallback list call after not found get")
	}
	foundCollectionFallback := false
	for _, listPath := range serverManager.listPaths {
		if listPath == "/admin/realms/master/clients" {
			foundCollectionFallback = true
			break
		}
	}
	if !foundCollectionFallback {
		t.Fatalf("expected fallback list to include /admin/realms/master/clients, got %#v", serverManager.listPaths)
	}
	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from fallback list item, got %T", value.Value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected alias-matched payload, got %#v", payload)
	}
}

func TestOrchestratorGetRemoteResolvesComplexAliasViaListCandidatePayload(t *testing.T) {
	t.Parallel()

	metadataDir := t.TempDir()
	metadataService := fsmetadata.NewFSMetadataService(metadataDir)
	ctx := context.Background()

	if err := metadataService.Set(ctx, "/apis/_", metadatadomain.ResourceMetadata{
		ID:    "/id",
		Alias: "{{/name}} - {{/version}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "/api/apis/{{/name}}/{{/version}}",
			},
			string(metadatadomain.OperationList): {
				Path: "/api/apis",
			},
		},
	}); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	requestLog := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, r.URL.Path)
		switch r.URL.Path {
		case "/api/apis":
			_, _ = fmt.Fprint(w, `[{"id":"api-orders-v1","name":"orders","version":"v1"}]`)
		case "/api/apis/orders/v1":
			_, _ = fmt.Fprint(w, `{"id":"api-orders-v1","name":"orders","version":"v1","details":"full"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := managedserverhttp.NewClient(
		config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "token",
				}},
			},
		},
		managedserverhttp.WithMetadataRenderer(metadataService),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	orchestrator := &Orchestrator{
		metadata: metadataService,
		server:   client,
	}

	value, err := orchestrator.GetRemote(ctx, "/apis/orders - v1")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected object payload, got %T", value.Value)
	}
	if payload["details"] != "full" {
		t.Fatalf("expected full GET payload after alias fallback, got %#v", payload)
	}
	if len(requestLog) < 2 || requestLog[0] != "/api/apis" || requestLog[1] != "/api/apis/orders/v1" {
		t.Fatalf("expected list fallback then resolved GET, got %#v", requestLog)
	}
}

func TestOrchestratorRequestGetResolvesComplexAliasViaListCandidatePayload(t *testing.T) {
	t.Parallel()

	metadataDir := t.TempDir()
	metadataService := fsmetadata.NewFSMetadataService(metadataDir)
	ctx := context.Background()

	if err := metadataService.Set(ctx, "/apis/_", metadatadomain.ResourceMetadata{
		ID:    "/id",
		Alias: "{{/name}} - {{/version}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "/api/apis/{{/name}}/{{/version}}",
			},
			string(metadatadomain.OperationList): {
				Path: "/api/apis",
			},
		},
	}); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	requestLog := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, r.URL.Path)
		switch r.URL.Path {
		case "/api/apis":
			_, _ = fmt.Fprint(w, `[{"id":"api-orders-v1","name":"orders","version":"v1"}]`)
		case "/api/apis/orders/v1":
			_, _ = fmt.Fprint(w, `{"id":"api-orders-v1","name":"orders","version":"v1","details":"full"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := managedserverhttp.NewClient(
		config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "token",
				}},
			},
		},
		managedserverhttp.WithMetadataRenderer(metadataService),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	orchestrator := &Orchestrator{
		metadata: metadataService,
		server:   client,
	}

	value, err := orchestrator.Request(ctx, managedserverdomain.RequestSpec{
		Method: "GET",
		Path:   "/apis/orders - v1",
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected object payload, got %T", value.Value)
	}
	if payload["details"] != "full" {
		t.Fatalf("expected full GET payload after alias fallback, got %#v", payload)
	}
	if len(requestLog) < 2 || requestLog[0] != "/api/apis" || requestLog[1] != "/api/apis/orders/v1" {
		t.Fatalf("expected list fallback then resolved GET request, got %#v", requestLog)
	}
}

func TestOrchestratorDeleteResolvesComplexAliasViaListCandidatePayload(t *testing.T) {
	t.Parallel()

	metadataDir := t.TempDir()
	metadataService := fsmetadata.NewFSMetadataService(metadataDir)
	ctx := context.Background()

	if err := metadataService.Set(ctx, "/apis/_", metadatadomain.ResourceMetadata{
		ID:    "/id",
		Alias: "{{/name}} - {{/version}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationDelete): {
				Path: "/api/apis/{{/name}}/{{/version}}",
			},
			string(metadatadomain.OperationList): {
				Path: "/api/apis",
			},
		},
	}); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	requestLog := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, r.URL.Path)
		switch r.URL.Path {
		case "/api/apis":
			_, _ = fmt.Fprint(w, `[{"id":"api-orders-v1","name":"orders","version":"v1"}]`)
		case "/api/apis/orders/v1":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := managedserverhttp.NewClient(
		config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "token",
				}},
			},
		},
		managedserverhttp.WithMetadataRenderer(metadataService),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	orchestrator := &Orchestrator{
		metadata: metadataService,
		server:   client,
	}

	if err := orchestrator.Delete(ctx, "/apis/orders - v1", orch.DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if len(requestLog) < 2 || requestLog[0] != "/api/apis" || requestLog[1] != "/api/apis/orders/v1" {
		t.Fatalf("expected list fallback then resolved DELETE request, got %#v", requestLog)
	}
}

func TestOrchestratorRequestDeleteResolvesComplexAliasViaListCandidatePayload(t *testing.T) {
	t.Parallel()

	metadataDir := t.TempDir()
	metadataService := fsmetadata.NewFSMetadataService(metadataDir)
	ctx := context.Background()

	if err := metadataService.Set(ctx, "/apis/_", metadatadomain.ResourceMetadata{
		ID:    "/id",
		Alias: "{{/name}} - {{/version}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationDelete): {
				Path: "/api/apis/{{/name}}/{{/version}}",
			},
			string(metadatadomain.OperationList): {
				Path: "/api/apis",
			},
		},
	}); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	requestLog := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, r.URL.Path)
		switch r.URL.Path {
		case "/api/apis":
			_, _ = fmt.Fprint(w, `[{"id":"api-orders-v1","name":"orders","version":"v1"}]`)
		case "/api/apis/orders/v1":
			if r.Method != http.MethodDelete {
				t.Fatalf("expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := managedserverhttp.NewClient(
		config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "token",
				}},
			},
		},
		managedserverhttp.WithMetadataRenderer(metadataService),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	orchestrator := &Orchestrator{
		metadata: metadataService,
		server:   client,
	}

	value, err := orchestrator.Request(ctx, managedserverdomain.RequestSpec{
		Method: "DELETE",
		Path:   "/apis/orders - v1",
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}
	if value.Value != nil {
		t.Fatalf("expected empty delete response, got %#v", value.Value)
	}
	if len(requestLog) < 2 || requestLog[0] != "/api/apis" || requestLog[1] != "/api/apis/orders/v1" {
		t.Fatalf("expected list fallback then resolved DELETE request, got %#v", requestLog)
	}
}

func TestOrchestratorGetRemoteUsesSingleJQFilteredCandidateFallback(t *testing.T) {
	t.Parallel()

	requestPath := "/admin/realms/publico-br/user-registry"
	resolvedIDPath := "/admin/realms/publico-br/13de4420-7c8d-4db7-b8f7-2d2a26f2053e"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		getValues: map[string]resource.Value{
			resolvedIDPath: map[string]any{
				"id":         "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name":       "ldap-1",
				"providerId": "ldap",
			},
		},
		listValues: map[string][]resource.Resource{
			"/admin/realms/publico-br": {
				{
					LogicalPath: "/admin/realms/publico-br/ldap-1",
					LocalAlias:  "ldap-1",
					RemoteID:    "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
					Payload: map[string]any{
						"id":         "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
						"name":       "ldap-1",
						"providerId": "ldap",
					},
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:                   "{{/id}}",
				Alias:                "{{/name}}",
				RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						Transforms: jqMutation(`[ .[] | select(.providerId == "ldap") ]`),
					},
				},
			},
		},
		server: serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), requestPath)
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	if !reflect.DeepEqual(serverManager.getPaths, []string{requestPath, resolvedIDPath}) {
		t.Fatalf("expected jq singleton fallback to retry with resolved id path, got get calls %#v", serverManager.getPaths)
	}
	foundCollectionFallback := false
	for _, listPath := range serverManager.listPaths {
		if listPath == "/admin/realms/publico-br" {
			foundCollectionFallback = true
			break
		}
	}
	if !foundCollectionFallback {
		t.Fatalf("expected fallback list call for /admin/realms/publico-br, got %#v", serverManager.listPaths)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", value.Value)
	}
	if payload["id"] != "13de4420-7c8d-4db7-b8f7-2d2a26f2053e" {
		t.Fatalf("unexpected payload %#v", payload)
	}
}

func TestOrchestratorGetRemoteDoesNotCollapseExplicitChildToSingletonJQCandidate(t *testing.T) {
	t.Parallel()

	requestPath := "/admin/realms/publico-br/user-registry/xxx"
	expectedSingletonAliasPath := "/admin/realms/publico-br/user-registry/AD PRD"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValues: map[string][]resource.Resource{
			"/admin/realms/publico-br/user-registry": {
				{
					LogicalPath: expectedSingletonAliasPath,
					LocalAlias:  "AD PRD",
					RemoteID:    "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
					Payload: map[string]any{
						"id":         "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
						"name":       "AD PRD",
						"providerId": "ldap",
					},
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:                   "{{/id}}",
				Alias:                "{{/name}}",
				RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						Transforms: jqMutation(`[ .[] | select(.providerId == "ldap") ]`),
					},
				},
			},
		},
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), requestPath)
	assertTypedCategory(t, err, faults.NotFoundError)

	for _, path := range serverManager.getPaths {
		if path == expectedSingletonAliasPath {
			t.Fatalf("expected explicit child lookup to keep NotFound and avoid singleton alias collapse, get calls %#v", serverManager.getPaths)
		}
	}
}

func TestOrchestratorGetRemoteDoesNotUseSingleCandidateFallbackWithoutJQ(t *testing.T) {
	t.Parallel()

	requestPath := "/admin/realms/publico-br/user-registry"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValues: map[string][]resource.Resource{
			"/admin/realms/publico-br": {
				{
					LogicalPath: "/admin/realms/publico-br/ldap-1",
					LocalAlias:  "ldap-1",
					RemoteID:    "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
					Payload: map[string]any{
						"id":         "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
						"name":       "ldap-1",
						"providerId": "ldap",
					},
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:                   "{{/id}}",
				Alias:                "{{/name}}",
				RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
			},
		},
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), requestPath)
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestOrchestratorGetRemoteResolvesAliasPathToMetadataIDBeforeCollectionFallback(t *testing.T) {
	t.Parallel()

	aliasPath := "/admin/realms/publico-br/organizations/teste"
	resolvedIDPath := "/admin/realms/publico-br/organizations/71ba388d-9f95-4a4d-b674-a632f697b732"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		getValues: map[string]resource.Value{
			resolvedIDPath: map[string]any{
				"id":    "71ba388d-9f95-4a4d-b674-a632f697b732",
				"alias": "teste",
				"name":  "teste ltda",
			},
		},
		listValues: map[string][]resource.Resource{
			"/admin/realms/publico-br/organizations": {
				{
					LogicalPath: aliasPath,
					LocalAlias:  "teste",
					RemoteID:    "71ba388d-9f95-4a4d-b674-a632f697b732",
					Payload: map[string]any{
						"id":    "71ba388d-9f95-4a4d-b674-a632f697b732",
						"alias": "teste",
					},
				},
			},
		},
		listErrs: map[string]error{
			aliasPath: faults.NewTypedError(faults.NotFoundError, "collection not found", nil),
		},
		openAPISpec: map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}/domains": map[string]any{
					"get": map[string]any{},
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValues: map[string]metadatadomain.ResourceMetadata{
				aliasPath: {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
				resolvedIDPath: {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
			},
		},
		server: serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), aliasPath)
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	if !reflect.DeepEqual(serverManager.getPaths, []string{aliasPath, resolvedIDPath}) {
		t.Fatalf("expected literal get then metadata-resolved id get, got %#v", serverManager.getPaths)
	}
	for _, listPath := range serverManager.listPaths {
		if listPath == aliasPath {
			t.Fatalf("expected metadata id fallback to run before collection fallback, list calls=%#v", serverManager.listPaths)
		}
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from resolved id read, got %T", value.Value)
	}
	if payload["id"] != "71ba388d-9f95-4a4d-b674-a632f697b732" || payload["alias"] != "teste" {
		t.Fatalf("unexpected payload from metadata id fallback: %#v", payload)
	}
}

func TestOrchestratorGetRemoteRecursivelyResolvesParentMetadataIdentity(t *testing.T) {
	t.Parallel()

	aliasPath := "/admin/realms/publico-br/organizations/teste"
	resolvedRealmPath := "/admin/realms/realm-1/organizations/teste"
	resolvedResourcePath := "/admin/realms/realm-1/organizations/org-1"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		getValues: map[string]resource.Value{
			resolvedResourcePath: map[string]any{
				"id":    "org-1",
				"alias": "teste",
				"name":  "Teste LTDA",
			},
		},
		listValues: map[string][]resource.Resource{
			"/admin/realms": {
				{
					LogicalPath: "/admin/realms/publico-br",
					LocalAlias:  "publico-br",
					RemoteID:    "realm-1",
					Payload: map[string]any{
						"id":    "realm-1",
						"alias": "publico-br",
					},
				},
			},
			"/admin/realms/realm-1/organizations": {
				{
					LogicalPath: "/admin/realms/realm-1/organizations/teste",
					LocalAlias:  "teste",
					RemoteID:    "org-1",
					Payload: map[string]any{
						"id":    "org-1",
						"alias": "teste",
					},
				},
			},
		},
		listErrs: map[string]error{
			"/admin/realms/publico-br/organizations": faults.NewTypedError(
				faults.NotFoundError,
				"resource not found",
				nil,
			),
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValues: map[string]metadatadomain.ResourceMetadata{
				aliasPath: {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
				resolvedRealmPath: {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
				resolvedResourcePath: {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
				"/admin/realms/publico-br": {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
				"/admin/realms/realm-1": {
					ID:    "{{/id}}",
					Alias: "{{/alias}}",
				},
			},
		},
		server: serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), aliasPath)
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	if !reflect.DeepEqual(serverManager.getPaths, []string{aliasPath, resolvedRealmPath, resolvedResourcePath}) {
		t.Fatalf(
			"expected recursive higher-level metadata fallback path search, got get calls %#v",
			serverManager.getPaths,
		)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", value.Value)
	}
	if payload["id"] != "org-1" || payload["alias"] != "teste" {
		t.Fatalf("unexpected recursively-resolved payload: %#v", payload)
	}
}

func TestOrchestratorGetRemoteKeepsOriginalNotFoundWhenRecursiveFallbackProbeResponseIsInvalid(t *testing.T) {
	t.Parallel()

	requestPath := "/admin/realms/xxxxx/organizations"

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErrs: map[string]error{
			"/admin/realms/xxxxx": faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			"/admin/realms":       faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			"/admin": faults.NewTypedError(
				faults.ValidationError,
				`response body is not valid JSON: invalid character '<' looking for beginning of value`,
				nil,
			),
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/alias}}",
			},
		},
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), requestPath)
	assertTypedCategory(t, err, faults.NotFoundError)

	foundAdminProbe := false
	for _, listPath := range serverManager.listPaths {
		if listPath == "/admin" {
			foundAdminProbe = true
			break
		}
	}
	if !foundAdminProbe {
		t.Fatalf("expected recursive fallback probe to include /admin list call, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetRemoteTreatsCollectionNotFoundAsEmptyWhenOpenAPIHintsCollection(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getValues: map[string]resource.Value{
			"/admin/realms/master": map[string]any{
				"realm": "master",
			},
		},
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
		openAPISpec: map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	items, ok := value.Value.([]any)
	if !ok {
		t.Fatalf("expected empty list payload, got %T", value.Value)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list payload, got %#v", items)
	}
	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/master/organizations"}) {
		t.Fatalf("expected direct collection list fallback path, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetRemoteTreatsCollectionNotFoundAsEmptyWhenRepositoryHintsCollection(t *testing.T) {
	t.Parallel()

	repositoryManager := &fakeRepository{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		existsValues: map[string]bool{
			"/admin/realms/master/organizations": true,
		},
	}
	serverManager := &fakeServer{
		getValues: map[string]resource.Value{
			"/admin/realms/master": map[string]any{
				"realm": "master",
			},
		},
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
	}
	orchestrator := &Orchestrator{
		repository: repositoryManager,
		server:     serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	items, ok := value.Value.([]any)
	if !ok {
		t.Fatalf("expected empty list payload, got %T", value.Value)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list payload, got %#v", items)
	}

	if !reflect.DeepEqual(repositoryManager.existsCalls, []string{"/admin/realms/master/organizations"}) {
		t.Fatalf("expected repository collection hint lookup, got %#v", repositoryManager.existsCalls)
	}
	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/master/organizations"}) {
		t.Fatalf("expected direct collection list fallback path, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetRemoteDoesNotTreatNotFoundAsEmptyWithoutOpenAPIOrRepositoryHints(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
	}
	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						Method: "GET",
						Path:   "/admin/realms/{{/realm}}/organizations",
					},
				},
			},
		},
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	assertTypedCategory(t, err, faults.NotFoundError)

	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/master"}) {
		t.Fatalf("expected parent collection fallback only, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetRemoteKeepsNotFoundForConcreteResourcePathWithOpenAPIChildEndpoints(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
		openAPISpec: map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/acme")
	assertTypedCategory(t, err, faults.NotFoundError)

	for _, listPath := range serverManager.listPaths {
		if listPath == "/admin/realms/acme" {
			t.Fatalf("expected no direct collection-list fallback for concrete resource path, got %#v", serverManager.listPaths)
		}
	}
	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms"}) {
		t.Fatalf("expected parent collection fallback only, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetRemoteKeepsNotFoundForCollectionWhenParentResourceIsMissing(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
		openAPISpec: map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/acme/organizations")
	assertTypedCategory(t, err, faults.NotFoundError)

	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/acme/organizations"}) {
		t.Fatalf("expected only direct collection list fallback attempt, got %#v", serverManager.listPaths)
	}
	if !reflect.DeepEqual(serverManager.getPaths, []string{"/admin/realms/acme/organizations", "/admin/realms/acme"}) {
		t.Fatalf("expected child GET then parent existence probe, got %#v", serverManager.getPaths)
	}
}

func TestOrchestratorGetRemoteKeepsNotFoundWhenParentFallbackListPayloadIsInvalid(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: managedserverdomain.NewListPayloadShapeError(
			`list response object is ambiguous: expected an "items" array or a single array field`,
			nil,
		),
	}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/publico/organizatio")
	assertTypedCategory(t, err, faults.NotFoundError)

	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/publico"}) {
		t.Fatalf("expected parent collection fallback path, got %#v", serverManager.listPaths)
	}
}

func TestOrchestratorGetLocalFallsBackToCollectionListByMetadataID(t *testing.T) {
	t.Parallel()

	repositoryManager := &fakeRepository{
		getValues: map[string]resource.Value{
			"/admin/realms/master/clients/account": map[string]any{
				"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				"clientId": "account",
			},
		},
		listValue: []resource.Resource{
			{LogicalPath: "/admin/realms/master/clients/account"},
		},
	}

	orchestrator := &Orchestrator{
		repository: repositoryManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
	}

	value, err := orchestrator.GetLocal(context.Background(), "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}

	if len(repositoryManager.listCalls) != 1 || repositoryManager.listCalls[0] != "/admin/realms/master/clients" {
		t.Fatalf("expected fallback list call for collection path, got %#v", repositoryManager.listCalls)
	}
	if len(repositoryManager.getCalls) != 2 {
		t.Fatalf("expected literal and fallback get calls, got %#v", repositoryManager.getCalls)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value.Value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected id-based local fallback payload, got %#v", payload)
	}
}

func TestOrchestratorGetLocalFallsBackToCommonIDFieldWhenMetadataUsesAlias(t *testing.T) {
	t.Parallel()

	repositoryManager := &fakeRepository{
		getValues: map[string]resource.Value{
			"/admin/realms/platform/clients/web-console": map[string]any{
				"id":       "client-0002",
				"clientId": "web-console",
			},
		},
		listValue: []resource.Resource{
			{LogicalPath: "/admin/realms/platform/clients/web-console"},
		},
	}

	orchestrator := &Orchestrator{
		repository: repositoryManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/clientId}}",
				Alias: "{{/clientId}}",
			},
		},
	}

	value, err := orchestrator.GetLocal(context.Background(), "/admin/realms/platform/clients/client-0002")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value.Value)
	}
	if payload["clientId"] != "web-console" || payload["id"] != "client-0002" {
		t.Fatalf("expected common-id fallback payload, got %#v", payload)
	}
}

func TestOrchestratorGetLocalFallbackPrefersListedAliasWithoutFullScan(t *testing.T) {
	t.Parallel()

	repositoryManager := &fakeRepository{
		getValues: map[string]resource.Value{
			"/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c": map[string]any{
				"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				"clientId": "account",
			},
			"/admin/realms/master/clients/11111111-1111-1111-1111-111111111111": map[string]any{
				"id":       "11111111-1111-1111-1111-111111111111",
				"clientId": "billing",
			},
		},
		listValue: []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				LocalAlias:  "account",
			},
			{
				LogicalPath: "/admin/realms/master/clients/11111111-1111-1111-1111-111111111111",
				LocalAlias:  "billing",
			},
		},
	}

	orchestrator := &Orchestrator{
		repository: repositoryManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
	}

	value, err := orchestrator.GetLocal(context.Background(), "/admin/realms/master/clients/account")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}

	if len(repositoryManager.getCalls) != 2 {
		t.Fatalf("expected miss + one hydrated alias candidate get calls, got %#v", repositoryManager.getCalls)
	}
	if repositoryManager.getCalls[1] != "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c" {
		t.Fatalf("expected second get call to hydrate matched alias candidate, got %#v", repositoryManager.getCalls)
	}

	payload, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value.Value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected alias-matched fallback payload, got %#v", payload)
	}
}

func TestOrchestratorApplyResolvesLocalPathByMetadataIDFallback(t *testing.T) {
	t.Parallel()

	repositoryManager := &fakeRepository{
		getValues: map[string]resource.Value{
			"/admin/realms/master/clients/account": map[string]any{
				"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				"clientId": "account",
			},
		},
		listValue: []resource.Resource{
			{LogicalPath: "/admin/realms/master/clients/account"},
		},
	}

	orchestrator := &Orchestrator{
		repository: repositoryManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
		server: &fakeServer{
			existsValue: true,
			updateValue: map[string]any{
				"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				"clientId": "account",
			},
		},
	}

	item, err := orchestrator.Apply(context.Background(), "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	serverManager := orchestrator.server.(*fakeServer)
	if !serverManager.updateCalled {
		t.Fatal("expected update mutation after local id fallback")
	}
	if got := serverManager.lastResource.RemoteID; got != "f88c68f3-3253-49f9-94a9-fe7553d33b5c" {
		t.Fatalf("expected resolved remote id from payload, got %q", got)
	}
	if got := item.LogicalPath; got != "/admin/realms/master/clients/account" {
		t.Fatalf("expected apply to operate on resolved local alias path, got %q", got)
	}
}

func TestOrchestratorDeleteRetriesWithResolvedRemoteIdentityAfterNotFound(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		deleteErrs: []error{
			faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			nil,
		},
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/clients/account",
				LocalAlias:  "account",
				RemoteID:    "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				Payload: map[string]any{
					"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
					"clientId": "account",
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		server: serverManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
	}

	if err := orchestrator.Delete(context.Background(), "/admin/realms/master/clients/account", orch.DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if len(serverManager.deleteResources) != 2 {
		t.Fatalf("expected two delete attempts, got %d", len(serverManager.deleteResources))
	}
	if got := serverManager.deleteResources[0].RemoteID; got != "account" {
		t.Fatalf("expected first delete attempt to use literal remote id, got %q", got)
	}
	if got := serverManager.deleteResources[1].RemoteID; got != "f88c68f3-3253-49f9-94a9-fe7553d33b5c" {
		t.Fatalf("expected second delete attempt to use resolved metadata id, got %q", got)
	}
}

func TestOrchestratorRequestDelegatesToServer(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: map[string]any{"ok": true},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
	}

	body := testContent(map[string]any{"id": "a"})
	value, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "POST",
		Path:   "/test",
		Body:   body,
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if !serverManager.requestCalled {
		t.Fatal("expected request to be delegated to server")
	}
	if serverManager.requestMethod != "POST" {
		t.Fatalf("expected method POST, got %q", serverManager.requestMethod)
	}
	if serverManager.requestPath != "/test" {
		t.Fatalf("expected path /test, got %q", serverManager.requestPath)
	}
	if !reflect.DeepEqual(serverManager.requestBody, body) {
		t.Fatalf("unexpected request body: %#v", serverManager.requestBody)
	}
	if !reflect.DeepEqual(value.Value, map[string]any{"ok": true}) {
		t.Fatalf("unexpected request response: %#v", value.Value)
	}
}

func TestOrchestratorRequestPostResolvesPathFromMetadata(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: map[string]any{"ok": true},
	}
	metadataService := &fakeMetadata{
		resolveValue: metadatadomain.ResourceMetadata{
			ID:                   "{{/id}}",
			Alias:                "{{/name}}",
			RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
		},
	}
	orchestrator := &Orchestrator{
		server:   serverManager,
		metadata: metadataService,
	}

	body := testContent(map[string]any{
		"providerId": "ldap",
		"name":       "AD Production",
	})
	_, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "POST",
		Path:   "/admin/realms/acme/user-registry/",
		Body:   body,
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if !serverManager.requestCalled {
		t.Fatal("expected request to be delegated to server")
	}
	if got := serverManager.requestPath; got != "/admin/realms/acme/components" {
		t.Fatalf("expected metadata-resolved request path, got %q", got)
	}
	if len(metadataService.resolveCalls) != 1 || metadataService.resolveCalls[0] != "/admin/realms/acme/user-registry" {
		t.Fatalf("expected metadata to resolve normalized logical path, got %#v", metadataService.resolveCalls)
	}
}

func TestOrchestratorRequestGetSelectorDepthResolvesListPathFromMetadata(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: []any{},
	}
	metadataService := &fakeMetadata{
		resolveValue: metadatadomain.ResourceMetadata{
			ID:                   "{{/id}}",
			Alias:                "{{/name}}",
			RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationList): {
					Transforms: jqMutation("[ .[] | select(.providerId == \"ldap\") ]"),
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server:   serverManager,
		metadata: metadataService,
	}

	_, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "GET",
		Path:   "/admin/realms/master/user-registry",
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if !serverManager.requestCalled {
		t.Fatal("expected request to be delegated to server")
	}
	if got := serverManager.requestPath; got != "/admin/realms/master/components" {
		t.Fatalf("expected metadata-resolved list path for selector-depth GET, got %q", got)
	}
}

func TestOrchestratorRequestGetFallsBackToMetadataAwareRemoteReadAfterNotFound(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestErr: faults.NewTypedError(faults.NotFoundError, "request path not found", nil),
		getErr:     faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/clients/account",
				LocalAlias:  "account",
				RemoteID:    "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				Payload: map[string]any{
					"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
					"clientId": "account",
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/clientId}}",
			},
		},
	}

	value, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "GET",
		Path:   "/admin/realms/master/clients/account",
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if len(serverManager.requestPaths) != 1 || serverManager.requestPaths[0] != "/admin/realms/master/clients/account" {
		t.Fatalf("expected one literal request GET attempt before fallback, got %#v", serverManager.requestPaths)
	}
	got, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected fallback GET payload object, got %T", value.Value)
	}
	if got["clientId"] != "account" {
		t.Fatalf("expected fallback GET payload to include resolved clientId, got %#v", got)
	}
}

func TestOrchestratorRequestDeleteRetriesWithResolvedRemoteIdentityAfterNotFound(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestErrs: []error{
			faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			nil,
		},
		requestValue: nil,
		getErr:       faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath: "/admin/realms/acme/organizations/alpha",
				LocalAlias:  "alpha",
				RemoteID:    "1d3235d4-8e70-409a-97e7-a32f6da01816",
				Payload: map[string]any{
					"id":    "1d3235d4-8e70-409a-97e7-a32f6da01816",
					"alias": "alpha",
				},
			},
		},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/alias}}",
			},
		},
	}

	_, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "DELETE",
		Path:   "/admin/realms/acme/organizations/alpha",
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if len(serverManager.requestPaths) != 2 {
		t.Fatalf("expected two request delete attempts, got %#v", serverManager.requestPaths)
	}
	if got := serverManager.requestPaths[0]; got != "/admin/realms/acme/organizations/alpha" {
		t.Fatalf("expected first request delete attempt to use literal alias path, got %q", got)
	}
	if got := serverManager.requestPaths[1]; got != "/admin/realms/acme/organizations/1d3235d4-8e70-409a-97e7-a32f6da01816" {
		t.Fatalf("expected second request delete attempt to use resolved metadata id path, got %q", got)
	}
}

func TestOrchestratorRequestPutCollectionPathRetriesLiteralAfterResolvedNotFound(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestErrs: []error{
			faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			nil,
		},
		requestValue: map[string]any{"ok": true},
	}
	orchestrator := &Orchestrator{
		server: serverManager,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/id}}",
				Alias: "{{/displayName}}",
			},
		},
	}

	body := testContent(map[string]any{
		"id":          "bd43239f-2111-4c03-83b6-c0902df38ec9",
		"requirement": "ALTERNATIVE",
	})

	value, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "PUT",
		Path:   "/admin/realms/acme/authentication/flows/test/executions",
		Body:   body,
	})
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if len(serverManager.requestPaths) != 2 {
		t.Fatalf("expected two request PUT attempts, got %#v", serverManager.requestPaths)
	}
	if got := serverManager.requestPaths[0]; got == "/admin/realms/acme/authentication/flows/test/executions" {
		t.Fatalf("expected first PUT attempt to use a metadata-resolved path before literal retry, got %q", got)
	}
	if got := serverManager.requestPaths[1]; got != "/admin/realms/acme/authentication/flows/test/executions" {
		t.Fatalf("expected second PUT attempt to retry literal collection path, got %q", got)
	}
	if !reflect.DeepEqual(value.Value, map[string]any{"ok": true}) {
		t.Fatalf("unexpected request response: %#v", value.Value)
	}
}

func TestOrchestratorRequestRequiresServer(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}
	_, err := orchestrator.Request(context.Background(), managedserverdomain.RequestSpec{
		Method: "GET",
		Path:   "/test",
	})
	assertTypedCategory(t, err, faults.ValidationError)
}

type fakeRepository struct {
	getValue     resource.Value
	getValues    map[string]resource.Value
	getErr       error
	listValue    []resource.Resource
	listErr      error
	existsValue  bool
	existsValues map[string]bool
	existsErr    error
	statusValue  repository.SyncReport

	savedPath       string
	savedValue      resource.Value
	savedDescriptor resource.PayloadDescriptor
	savedArtifacts  []repository.ResourceArtifact
	getCalls        []string
	listCalls       []string
	existsCalls     []string
	artifactFiles   map[string][]byte

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func testContent(value any) resource.Content {
	return testContentWithType(value, resource.PayloadTypeJSON)
}

func testContentWithType(value any, payloadType string) resource.Content {
	return resource.Content{
		Value:      value,
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: payloadType}),
	}
}

func writeOrchestratorTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func (f *fakeRepository) Save(_ context.Context, logicalPath string, value resource.Content) error {
	f.savedPath = logicalPath
	f.savedValue = value.Value
	f.savedDescriptor = value.Descriptor
	return nil
}

func (f *fakeRepository) SaveResourceWithArtifacts(
	_ context.Context,
	logicalPath string,
	value resource.Content,
	artifacts []repository.ResourceArtifact,
) error {
	f.savedPath = logicalPath
	f.savedValue = value.Value
	f.savedDescriptor = value.Descriptor
	f.savedArtifacts = append([]repository.ResourceArtifact(nil), artifacts...)
	if f.artifactFiles == nil {
		f.artifactFiles = map[string][]byte{}
	}
	for _, artifact := range artifacts {
		f.artifactFiles[logicalPath+"::"+artifact.File] = append([]byte(nil), artifact.Content...)
	}
	return nil
}

func (f *fakeRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	f.getCalls = append(f.getCalls, logicalPath)

	if f.getErr != nil {
		return resource.Content{}, f.getErr
	}

	if f.getValues != nil {
		if value, found := f.getValues[logicalPath]; found {
			return testContent(value), nil
		}
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
	}

	return testContent(f.getValue), nil
}

func (f *fakeRepository) Delete(_ context.Context, _ string, policy repository.DeletePolicy) error {
	f.deletePolicy = policy
	return nil
}

func (f *fakeRepository) List(_ context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	f.listCalls = append(f.listCalls, logicalPath)
	f.listPolicy = policy
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listValue, nil
}

func (f *fakeRepository) Exists(_ context.Context, logicalPath string) (bool, error) {
	f.existsCalls = append(f.existsCalls, logicalPath)
	if f.existsErr != nil {
		return false, f.existsErr
	}

	if f.existsValues != nil {
		if exists, found := f.existsValues[logicalPath]; found {
			return exists, nil
		}
	}

	return f.existsValue, nil
}

func (f *fakeRepository) ReadResourceArtifact(_ context.Context, logicalPath string, file string) ([]byte, error) {
	if f.artifactFiles == nil {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource artifact %q not found for %q", file, logicalPath),
			nil,
		)
	}

	content, found := f.artifactFiles[logicalPath+"::"+file]
	if !found {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource artifact %q not found for %q", file, logicalPath),
			nil,
		)
	}

	return append([]byte(nil), content...), nil
}
func (f *fakeRepository) Init(context.Context) error    { return nil }
func (f *fakeRepository) Refresh(context.Context) error { return nil }
func (f *fakeRepository) Clean(context.Context) error   { return nil }
func (f *fakeRepository) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (f *fakeRepository) Check(context.Context) error { return nil }
func (f *fakeRepository) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (f *fakeRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return f.statusValue, nil
}

type fakeMetadata struct {
	resolveValue  metadatadomain.ResourceMetadata
	resolveValues map[string]metadatadomain.ResourceMetadata
	resolveErr    error
	resolveCalls  []string
}

func (f *fakeMetadata) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

func (f *fakeMetadata) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f *fakeMetadata) Unset(context.Context, string) error {
	return nil
}

func (f *fakeMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	f.resolveCalls = append(f.resolveCalls, logicalPath)
	if f.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, f.resolveErr
	}
	if f.resolveValues != nil {
		if value, found := f.resolveValues[logicalPath]; found {
			return value, nil
		}
	}
	return f.resolveValue, nil
}

func (f *fakeMetadata) RenderOperationSpec(
	ctx context.Context,
	_ string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.ResolveOperationSpec(ctx, f.resolveValue, operation, value)
}

func (f *fakeMetadata) Infer(context.Context, string, metadatadomain.InferenceRequest) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

type fakeServer struct {
	getValue     resource.Value
	getValues    map[string]resource.Value
	getErr       error
	createValue  resource.Value
	createErr    error
	updateValue  resource.Value
	updateErr    error
	listValue    []resource.Resource
	listValues   map[string][]resource.Resource
	listErr      error
	listErrs     map[string]error
	listFunc     func(context.Context, string, metadatadomain.ResourceMetadata) ([]resource.Resource, error)
	existsValue  bool
	existsErr    error
	requestValue resource.Value
	requestErr   error
	requestErrs  []error
	deleteErr    error
	deleteErrs   []error

	createCalled    bool
	updateCalled    bool
	deleteCalled    bool
	getCalled       bool
	getPaths        []string
	listCalled      bool
	requestCalled   bool
	requestMethod   string
	requestPath     string
	requestPaths    []string
	requestBody     resource.Content
	lastResource    resource.Resource
	lastListPath    string
	listPaths       []string
	openAPISpec     resource.Value
	openAPIErr      error
	deleteResources []resource.Resource
}

func (f *fakeServer) Get(_ context.Context, resolvedResource resource.Resource, _ metadatadomain.ResourceMetadata) (resource.Content, error) {
	f.getCalled = true
	f.lastResource = resolvedResource
	f.getPaths = append(f.getPaths, resolvedResource.LogicalPath)
	if f.getValues != nil {
		if value, found := f.getValues[resolvedResource.LogicalPath]; found {
			return testContent(value), nil
		}
	}
	if f.getErr != nil {
		return resource.Content{}, f.getErr
	}
	return testContent(f.getValue), nil
}

func (f *fakeServer) Create(_ context.Context, resolvedResource resource.Resource, _ metadatadomain.ResourceMetadata) (resource.Content, error) {
	f.createCalled = true
	f.lastResource = resolvedResource
	if f.createErr != nil {
		return resource.Content{}, f.createErr
	}
	return testContent(f.createValue), nil
}

func (f *fakeServer) Update(_ context.Context, resolvedResource resource.Resource, _ metadatadomain.ResourceMetadata) (resource.Content, error) {
	f.updateCalled = true
	f.lastResource = resolvedResource
	if f.updateErr != nil {
		return resource.Content{}, f.updateErr
	}
	return testContent(f.updateValue), nil
}

func (f *fakeServer) Delete(_ context.Context, resolvedResource resource.Resource, _ metadatadomain.ResourceMetadata) error {
	f.deleteCalled = true
	f.lastResource = resolvedResource
	f.deleteResources = append(f.deleteResources, resolvedResource)
	if len(f.deleteErrs) > 0 {
		err := f.deleteErrs[0]
		f.deleteErrs = f.deleteErrs[1:]
		return err
	}
	return f.deleteErr
}

func (f *fakeServer) List(ctx context.Context, logicalPath string, metadataValue metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
	f.listCalled = true
	f.lastListPath = logicalPath
	f.listPaths = append(f.listPaths, logicalPath)
	if f.listFunc != nil {
		return f.listFunc(ctx, logicalPath, metadataValue)
	}
	if f.listErrs != nil {
		if err, found := f.listErrs[logicalPath]; found {
			return nil, err
		}
	}
	if f.listValues != nil {
		if items, found := f.listValues[logicalPath]; found {
			return items, nil
		}
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listValue, nil
}

func (f *fakeServer) Exists(context.Context, resource.Resource, metadatadomain.ResourceMetadata) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.existsValue, nil
}

func (f *fakeServer) Request(_ context.Context, spec managedserverdomain.RequestSpec) (resource.Content, error) {
	f.requestCalled = true
	f.requestMethod = spec.Method
	f.requestPath = spec.Path
	f.requestPaths = append(f.requestPaths, spec.Path)
	f.requestBody = spec.Body
	if len(f.requestErrs) > 0 {
		err := f.requestErrs[0]
		f.requestErrs = f.requestErrs[1:]
		if err != nil {
			return resource.Content{}, err
		}
	}
	if f.requestErr != nil {
		return resource.Content{}, f.requestErr
	}
	return testContent(f.requestValue), nil
}

func (f *fakeServer) GetOpenAPISpec(context.Context) (resource.Content, error) {
	if f.openAPIErr != nil {
		return resource.Content{}, f.openAPIErr
	}
	return testContent(f.openAPISpec), nil
}

func (f *fakeServer) BuildRequestFromMetadata(context.Context, resource.Resource, metadatadomain.Operation) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

type fakeSecretProvider struct {
	values map[string]string
}

func (f *fakeSecretProvider) Init(context.Context) error { return nil }

func (f *fakeSecretProvider) Store(_ context.Context, key string, value string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value
	return nil
}

func (f *fakeSecretProvider) Get(_ context.Context, key string) (string, error) {
	value, found := f.values[key]
	if !found {
		return "", faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("secret %q not found", key), nil)
	}
	return value, nil
}

func (f *fakeSecretProvider) Delete(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func (f *fakeSecretProvider) List(context.Context) ([]string, error) { return nil, nil }

func (f *fakeSecretProvider) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return f.Store(ctx, key, secretValue)
	})
}

func (f *fakeSecretProvider) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return f.Get(ctx, key)
	})
}

func (f *fakeSecretProvider) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (f *fakeSecretProvider) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

func TestOrchestratorApplyUsesSecretsForRemoteMutation(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":       "42",
			"alias":    "acme",
			"apiToken": "{{ secret . }}",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{/id}}"},
			string(metadatadomain.OperationCreate): {Path: "/api/customers"},
		},
	}
	metadataService := &fakeMetadata{resolveValue: md}

	serverManager := &fakeServer{
		existsValue: true,
		updateValue: map[string]any{
			"id":       "42",
			"alias":    "acme",
			"apiToken": "super-secret",
		},
	}

	secretProvider := &fakeSecretProvider{
		values: map[string]string{
			"/customers/acme:/apiToken": "super-secret",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
		secrets:    secretProvider,
	}

	item, err := orchestrator.Apply(context.Background(), "/customers/acme", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if !serverManager.updateCalled || serverManager.createCalled {
		t.Fatalf("expected update mutation, got create=%t update=%t", serverManager.createCalled, serverManager.updateCalled)
	}

	updatePayload, ok := serverManager.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected update payload map, got %T", serverManager.lastResource.Payload)
	}
	if got := updatePayload["apiToken"]; got != "super-secret" {
		t.Fatalf("expected resolved secret for remote payload, got %#v", got)
	}

	if repo.savedValue != nil {
		t.Fatalf("expected apply to avoid implicit local persistence, got %#v", repo.savedValue)
	}

	if !reflect.DeepEqual(item.Payload, serverManager.updateValue) {
		t.Fatalf("expected returned payload to match remote mutation payload, got %#v", item.Payload)
	}
}

func TestOrchestratorApplyUsesResolvedRemoteIDForUpdateAfterRemoteFallback(t *testing.T) {
	t.Parallel()

	const remoteID = "13de4420-7c8d-4db7-b8f7-2d2a26f2053e"

	repo := &fakeRepository{
		getValue: map[string]any{
			"clientId": "testA",
			"name":     "testA",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/clientId}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):    {Path: "/admin/realms/test/clients/{{/id}}"},
			string(metadatadomain.OperationList):   {Path: "/admin/realms/test/clients"},
			string(metadatadomain.OperationUpdate): {Path: "/admin/realms/test/clients/{{/id}}"},
		},
	}
	metadataService := &fakeMetadata{resolveValue: md}

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath:    "/admin/realms/test/clients/testA",
				CollectionPath: "/admin/realms/test/clients",
				LocalAlias:     "testA",
				RemoteID:       remoteID,
				Payload: map[string]any{
					"id":       remoteID,
					"clientId": "testA",
					"name":     "before",
				},
			},
		},
		updateValue: map[string]any{
			"id":       remoteID,
			"clientId": "testA",
			"name":     "testA",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	item, err := orchestrator.Apply(context.Background(), "/admin/realms/test/clients/testA", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if serverManager.createCalled {
		t.Fatal("expected apply to update existing remote resource instead of create")
	}
	if !serverManager.updateCalled {
		t.Fatal("expected apply to update existing remote resource")
	}
	if got := serverManager.lastResource.RemoteID; got != remoteID {
		t.Fatalf("expected update to use resolved remote id %q, got %q", remoteID, got)
	}
	if !reflect.DeepEqual(item.Payload, serverManager.updateValue) {
		t.Fatalf("expected payload from update response, got %#v", item.Payload)
	}
}

func TestOrchestratorApplySkipsUpdateWhenCompareShowsNoDrift(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"updatedAt": "2026-03-01T10:00:00Z",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationCompare): {Transforms: suppressMutation("/updatedAt")},
			string(metadatadomain.OperationUpdate):  {Path: "/api/customers/{{/id}}"},
		},
	}
	metadataService := &fakeMetadata{resolveValue: md}

	serverManager := &fakeServer{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"updatedAt": "2026-03-02T18:15:00Z",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	item, err := orchestrator.Apply(context.Background(), "/customers/acme", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if serverManager.createCalled || serverManager.updateCalled {
		t.Fatalf("expected apply no-op for compare-equal payloads, got create=%t update=%t", serverManager.createCalled, serverManager.updateCalled)
	}
	if !reflect.DeepEqual(item.Payload, serverManager.getValue) {
		t.Fatalf("expected no-op apply to return remote payload, got %#v", item.Payload)
	}
}

func TestOrchestratorApplyForceUpdatesWhenCompareShowsNoDrift(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"updatedAt": "2026-03-01T10:00:00Z",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationCompare): {Transforms: suppressMutation("/updatedAt")},
			string(metadatadomain.OperationUpdate):  {Path: "/api/customers/{{/id}}"},
		},
	}
	metadataService := &fakeMetadata{resolveValue: md}

	serverManager := &fakeServer{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"updatedAt": "2026-03-02T18:15:00Z",
		},
		updateValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"updatedAt": "2026-03-01T10:00:00Z",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	item, err := orchestrator.Apply(context.Background(), "/customers/acme", orch.ApplyPolicy{Force: true})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !serverManager.updateCalled {
		t.Fatal("expected force apply to execute update even when compare output has no drift")
	}
	if !reflect.DeepEqual(item.Payload, serverManager.updateValue) {
		t.Fatalf("expected payload from force-update response, got %#v", item.Payload)
	}
}

func TestOrchestratorApplyWholeResourceOpaqueSecretUsesCompareProjection(t *testing.T) {
	t.Parallel()

	wholeResourceSecret := true
	md := metadatadomain.ResourceMetadata{
		Secret: &wholeResourceSecret,
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationCompare): {
				Transforms: jqMutation(`if type == "object" and has("meta") then {name: (.name // "{{/id}}"), type: (.meta["Rundeck-key-type"] // "password"), contentType: (.meta["Rundeck-content-type"] // "application/x-rundeck-data-password")} else {name: (.name // "{{/id}}"), type: (.type // (if (.contentType // "") == "application/pgp-keys" then "public" elif (.contentType // "") == "application/octet-stream" then "private" else "password" end)), contentType: (.contentType // (if (.type // "") == "public" then "application/pgp-keys" elif (.type // "") == "private" then "application/octet-stream" else "application/x-rundeck-data-password" end))} end`),
			},
		},
	}

	serverManager := &fakeServer{
		getValue: map[string]any{
			"name": "private-key",
			"meta": map[string]any{
				"Rundeck-key-type":     "private",
				"Rundeck-content-type": "application/octet-stream",
			},
		},
	}

	orchestrator := &Orchestrator{
		metadata: &fakeMetadata{resolveValue: md},
		server:   serverManager,
	}

	item, err := orchestrator.ApplyWithContent(
		context.Background(),
		"/projects/platform/secrets/private-key",
		resource.Content{
			Value: resource.BinaryValue{Bytes: []byte("private-key-bytes")},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
				Extension: ".key",
			}),
		},
		orch.ApplyPolicy{},
	)
	if err != nil {
		t.Fatalf("ApplyWithContent returned error: %v", err)
	}
	if serverManager.createCalled || serverManager.updateCalled {
		t.Fatalf("expected opaque whole-resource secret apply no-op, got create=%t update=%t", serverManager.createCalled, serverManager.updateCalled)
	}
	if !reflect.DeepEqual(item.Payload, serverManager.getValue) {
		t.Fatalf("expected no-op apply to return remote payload, got %#v", item.Payload)
	}
}

func TestOrchestratorApplyRetriesUpdateWhenCreateConflicts(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"realm": "test2",
		},
	}

	serverManager := &fakeServer{
		getErr:    faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		createErr: faults.NewConflictError("remote request failed with status 409: realm already exists", nil),
		updateValue: map[string]any{
			"realm": "test2",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ID:    "{{/realm}}",
				Alias: "{{/realm}}",
			},
		},
		server: serverManager,
	}

	item, err := orchestrator.Apply(context.Background(), "/admin/realms/test2", orch.ApplyPolicy{})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if !serverManager.createCalled {
		t.Fatal("expected apply to attempt create when exists check is false")
	}
	if !serverManager.updateCalled {
		t.Fatal("expected apply to retry update after create conflict")
	}
	if got := item.LogicalPath; got != "/admin/realms/test2" {
		t.Fatalf("expected logical path /admin/realms/test2, got %q", got)
	}
	if !reflect.DeepEqual(item.Payload, serverManager.updateValue) {
		t.Fatalf("expected payload from update fallback, got %#v", item.Payload)
	}
}

func TestOrchestratorDiffUsesFallbackAndCompareSuppressRules(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"name":      "ACME",
			"apiToken":  "{{ secret . }}",
			"updatedAt": "2026-02-10T10:00:00Z",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{/id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Transforms: suppressMutation("/updatedAt")},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath: "/customers/acme",
				LocalAlias:  "acme",
				RemoteID:    "42",
				Payload: map[string]any{
					"id":        "42",
					"alias":     "acme",
					"name":      "ACME",
					"apiToken":  "super-secret",
					"updatedAt": "2026-02-11T10:00:00Z",
				},
			},
		},
	}

	secretProvider := &fakeSecretProvider{
		values: map[string]string{
			"/customers/acme:/apiToken": "super-secret",
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
		secrets:    secretProvider,
	}

	items, err := orchestrator.Diff(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !serverManager.getCalled || !serverManager.listCalled {
		t.Fatalf("expected bounded fallback flow, get=%t list=%t", serverManager.getCalled, serverManager.listCalled)
	}
	if len(items) != 0 {
		t.Fatalf("expected no drift after compare transforms, got %#v", items)
	}
}

func TestOrchestratorDiffAppliesCompareJQTransforms(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"name": "platform",
			"config": map[string]any{
				"project.description": "Managed by declarest",
			},
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/name}}",
		Alias: "{{/name}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationCompare): {
				Transforms: jqMutation(`if type == "object" and has("config") then {name: .name, config: (.config + {"project.name": .name})} else . end`),
			},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getValue: map[string]any{
			"name": "platform",
			"config": map[string]any{
				"project.description": "Managed by declarest",
				"project.name":        "platform",
			},
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	items, err := orchestrator.Diff(context.Background(), "/projects/platform")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no drift after compare jq transforms, got %#v", items)
	}
}

func TestOrchestratorDiffTreatsMissingRemoteResourceAsDrift(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "42",
			"alias": "acme",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{/id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{/id}}"},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	items, err := orchestrator.Diff(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if !serverManager.getCalled || !serverManager.listCalled {
		t.Fatalf("expected bounded fallback flow, get=%t list=%t", serverManager.getCalled, serverManager.listCalled)
	}
	if len(items) != 1 {
		t.Fatalf("expected one top-level drift entry, got %#v", items)
	}
	if items[0].ResourcePath != "/customers/acme" {
		t.Fatalf("expected drift resource path /customers/acme, got %#v", items[0].ResourcePath)
	}
	if items[0].Path != "" {
		t.Fatalf("expected root drift pointer path to be empty string, got %#v", items[0].Path)
	}
	if items[0].Operation != "replace" {
		t.Fatalf("expected replace operation for missing remote payload, got %#v", items[0].Operation)
	}
	if items[0].Remote != nil {
		t.Fatalf("expected nil remote payload for missing resource, got %#v", items[0].Remote)
	}
}

func TestOrchestratorDiffReturnsConflictOnAmbiguousFallback(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "42",
			"alias": "acme",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{/id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{/id}}"},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme-1", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
			{LogicalPath: "/customers/acme-2", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	_, err := orchestrator.Diff(context.Background(), "/customers/acme")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestOrchestratorDiffReturnsConflictWhenDirectGetIdentityIsAmbiguous(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "remote-two",
			"alias": "remote-one",
		},
	}

	md := metadatadomain.ResourceMetadata{
		ID:    "{{/id}}",
		Alias: "{{/alias}}",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{/id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{/id}}"},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getValue: map[string]any{
			"id":    "remote-two",
			"alias": "remote-two",
		},
		listValues: map[string][]resource.Resource{
			"/customers": {
				{
					LogicalPath: "/customers/remote-one",
					LocalAlias:  "remote-one",
					RemoteID:    "remote-one",
					Payload:     map[string]any{"id": "remote-one", "alias": "remote-one"},
				},
				{
					LogicalPath: "/customers/remote-two",
					LocalAlias:  "remote-two",
					RemoteID:    "remote-two",
					Payload:     map[string]any{"id": "remote-two", "alias": "remote-two"},
				},
			},
		},
	}

	orchestrator := &Orchestrator{
		repository: repo,
		metadata:   metadataService,
		server:     serverManager,
	}

	_, err := orchestrator.Diff(context.Background(), "/customers/local")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestOrchestratorListRemoteSortsDeterministically(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		repository: &fakeRepository{},
		metadata:   &fakeMetadata{resolveValue: metadatadomain.ResourceMetadata{}},
		server: &fakeServer{
			listValue: []resource.Resource{
				{LogicalPath: "/customers/zeta"},
				{LogicalPath: "/customers/acme"},
			},
		},
	}

	items, err := orchestrator.ListRemote(context.Background(), "/customers", orch.ListPolicy{})
	if err != nil {
		t.Fatalf("ListRemote returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].LogicalPath != "/customers/acme" || items[1].LogicalPath != "/customers/zeta" {
		t.Fatalf("expected deterministic order, got %#v", items)
	}
}

func TestOrchestratorListRemoteProvidesListJQResourceResolver(t *testing.T) {
	t.Parallel()

	metadataService := &fakeMetadata{
		resolveValues: map[string]metadatadomain.ResourceMetadata{
			"/admin/realms/publico-br/user-registry/ldap-test/mappers": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
			"/admin/realms/publico-br/user-registry/ldap-test": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						Transforms: jqMutation(`[ .[] | select(.providerId == "ldap") ]`),
					},
				},
			},
			"/admin/realms/publico-br/user-registry": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listFunc: func(ctx context.Context, logicalPath string, _ metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
			switch logicalPath {
			case "/admin/realms/publico-br/user-registry/ldap-test/mappers":
				parentValue, resolved, resolveErr := managedserverdomain.ResolveListJQResource(
					ctx,
					"/admin/realms/publico-br/user-registry/ldap-test",
				)
				if resolveErr != nil {
					return nil, resolveErr
				}
				if !resolved {
					return nil, fmt.Errorf("expected list jq resolver in context")
				}

				parentPayload, ok := parentValue.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("expected parent payload map, got %T", parentValue)
				}
				parentID, _ := parentPayload["id"].(string)

				items := []resource.Resource{
					{
						LogicalPath: "/admin/realms/publico-br/user-registry/ldap-test/mappers/alpha",
						LocalAlias:  "alpha",
						RemoteID:    "mapper-a",
						Payload: map[string]any{
							"id":       "mapper-a",
							"name":     "alpha",
							"parentId": "ldap-id",
						},
					},
					{
						LogicalPath: "/admin/realms/publico-br/user-registry/ldap-test/mappers/beta",
						LocalAlias:  "beta",
						RemoteID:    "mapper-b",
						Payload: map[string]any{
							"id":       "mapper-b",
							"name":     "beta",
							"parentId": "other-id",
						},
					},
				}

				filtered := make([]resource.Resource, 0, len(items))
				for _, item := range items {
					payloadMap, _ := item.Payload.(map[string]any)
					if payloadMap["parentId"] == parentID {
						filtered = append(filtered, item)
					}
				}
				return filtered, nil
			case "/admin/realms/publico-br/user-registry":
				return []resource.Resource{
					{
						LogicalPath: "/admin/realms/publico-br/user-registry/ldap-test",
						LocalAlias:  "ldap-test",
						RemoteID:    "ldap-id",
						Payload: map[string]any{
							"id":         "ldap-id",
							"name":       "ldap-test",
							"providerId": "ldap",
						},
					},
				}, nil
			default:
				return nil, faults.NewTypedError(faults.NotFoundError, "list not found", nil)
			}
		},
	}

	orchestrator := &Orchestrator{
		repository: &fakeRepository{},
		metadata:   metadataService,
		server:     serverManager,
	}

	items, err := orchestrator.ListRemote(
		context.Background(),
		"/admin/realms/publico-br/user-registry/ldap-test/mappers",
		orch.ListPolicy{},
	)
	if err != nil {
		t.Fatalf("ListRemote returned error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected filtered list with 1 item, got %d", len(items))
	}
	if items[0].LogicalPath != "/admin/realms/publico-br/user-registry/ldap-test/mappers/alpha" {
		t.Fatalf("unexpected filtered list item %#v", items[0].LogicalPath)
	}
}

func TestOrchestratorTemplateReturnsNormalizedPayload(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		repository: &fakeRepository{},
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{/id}}"},
				},
			},
		},
	}

	templated, err := orchestrator.Template(context.Background(), "/customers/acme", testContentWithType(map[string]any{
		"id":     "42",
		"name":   "ACME",
		"count":  float64(2),
		"format": "{{payload_type .}}",
		"token":  "{{secret .}}",
	}, resource.PayloadTypeYAML))
	if err != nil {
		t.Fatalf("Template returned error: %v", err)
	}

	output, ok := templated.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected templated map output, got %T", templated.Value)
	}
	if got := output["name"]; got != "ACME" {
		t.Fatalf("expected templated payload to preserve values, got %#v", got)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected payload_type placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "{{secret .}}" {
		t.Fatalf("expected secret placeholder to remain unresolved in template output, got %#v", got)
	}
}

func TestOrchestratorDiffUsesExpandedExternalizedAttributes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"name":   "ACME",
			"script": "{{include script.sh}}",
		},
		artifactFiles: map[string][]byte{
			"/customers/acme::script.sh": []byte("echo hello"),
		},
	}
	server := &fakeServer{
		getValue: map[string]any{
			"name":   "ACME",
			"script": "echo hello",
		},
	}
	orchestrator := &Orchestrator{
		repository: repo,
		metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{
						Path: "/script",
						File: "script.sh",
					},
				},
			},
		},
		server: server,
	}

	items, err := orchestrator.Diff(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no diff after externalized expansion, got %#v", items)
	}
}

func TestOrchestratorResolvePayloadForRemoteSupportsPayloadDescriptorWithoutSecrets(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}

	resolved, err := orchestrator.resolvePayloadForRemote(
		context.Background(),
		"/customers/acme",
		testContentWithType(map[string]any{
			"format": "{{payload_type .}}",
			"token":  "{{secret .}}",
		}, resource.PayloadTypeYAML),
	)
	if err != nil {
		t.Fatalf("resolvePayloadForRemote returned error: %v", err)
	}

	output, ok := resolved.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected resolved payload map, got %T", resolved.Value)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected payload_type placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "{{secret .}}" {
		t.Fatalf("expected secret placeholder to remain unresolved without secret provider, got %#v", got)
	}
}

func TestOrchestratorResolvePayloadForRemoteSupportsPayloadDescriptorWithSecrets(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{
		secrets: &fakeSecretProvider{
			values: map[string]string{
				"/customers/acme:/token": "super-secret",
			},
		},
	}

	resolved, err := orchestrator.resolvePayloadForRemote(
		context.Background(),
		"/customers/acme",
		testContentWithType(map[string]any{
			"format": "{{payload_type .}}",
			"token":  "{{secret .}}",
		}, resource.PayloadTypeYAML),
	)
	if err != nil {
		t.Fatalf("resolvePayloadForRemote returned error: %v", err)
	}

	output, ok := resolved.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected resolved payload map, got %T", resolved.Value)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected payload_type placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "super-secret" {
		t.Fatalf("expected secret placeholder to resolve, got %#v", got)
	}
}

func TestOrchestratorRenderOperationSpecListUsesCollectionPathFallback(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}
	resource := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
	}
	resourceMd := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{},
	}

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resource,
		resourceMd,
		metadatadomain.OperationList,
		map[string]any{"realm": "platform", "clientId": "declarest-cli"},
	)
	if err != nil {
		t.Fatalf("renderOperationSpec returned error: %v", err)
	}

	if spec.Path != "/admin/realms/platform/clients" {
		t.Fatalf("expected list fallback path /admin/realms/platform/clients, got %q", spec.Path)
	}
}

func TestOrchestratorRenderOperationSpecCreateUsesCollectionPathFallback(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}
	resource := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
	}
	resourceMd := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{},
	}

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resource,
		resourceMd,
		metadatadomain.OperationCreate,
		map[string]any{"realm": "platform", "clientId": "declarest-cli"},
	)
	if err != nil {
		t.Fatalf("renderOperationSpec returned error: %v", err)
	}

	if spec.Path != "/admin/realms/platform/clients" {
		t.Fatalf("expected create fallback path to use collection path, got %q", spec.Path)
	}
}

func TestOrchestratorRenderOperationSpecSupportsPayloadTemplateFunc(t *testing.T) {
	t.Parallel()

	orchestrator := &Orchestrator{}

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resource.Resource{
			LogicalPath:    "/customers/acme",
			CollectionPath: "/customers",
			Payload:        map[string]any{"id": "42"},
			PayloadDescriptor: resource.NormalizePayloadDescriptor(
				resource.PayloadDescriptor{PayloadType: resource.PayloadTypeYAML},
			),
		},
		metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationGet): {
					Path:   "/api/customers/{{/id}}",
					Accept: "{{payload_media_type .}}",
				},
			},
		},
		metadatadomain.OperationGet,
		map[string]any{"id": "42"},
	)
	if err != nil {
		t.Fatalf("renderOperationSpec returned error: %v", err)
	}
	if spec.Accept != "application/yaml" {
		t.Fatalf("expected accept application/yaml, got %q", spec.Accept)
	}
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
