package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	serverdomain "github.com/crmarques/declarest/server"
)

func TestDefaultOrchestratorDelegatesRepositoryMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepository{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}

	orchestrator := &DefaultOrchestrator{
		Repository: fakeRepo,
	}

	value, err := orchestrator.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value == nil {
		t.Fatal("expected non-nil value")
	}

	if err := orchestrator.Save(context.Background(), "/customers/acme", map[string]any{"x": 1}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	items, err := orchestrator.ListLocal(context.Background(), "/customers", ListPolicy{Recursive: true})
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

func TestDefaultOrchestratorDeleteDelegatesToServer(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{}
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
	}

	if err := orchestrator.Delete(context.Background(), "/customers/acme", DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !serverManager.deleteCalled {
		t.Fatal("expected delete call to be delegated to server")
	}
	if got := serverManager.lastResource.LogicalPath; got != "/customers/acme" {
		t.Fatalf("expected normalized delete logical path /customers/acme, got %q", got)
	}
}

func TestDefaultOrchestratorRequiresRepository(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}

	_, err := orchestrator.Get(context.Background(), "/customers/acme")
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.ValidationError)
}

func TestDefaultOrchestratorGetFallsBackToRemoteWhenLocalMissing(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Repository: &fakeRepository{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		},
		Metadata: &fakeMetadata{
			resolveErr: faults.NewTypedError(faults.NotFoundError, "metadata not found", nil),
		},
		Server: &fakeServer{
			getValue: map[string]any{"realm": "master"},
		},
	}

	value, err := orchestrator.Get(context.Background(), "/admin/realms/")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	serverManager := orchestrator.Server.(*fakeServer)
	if !serverManager.getCalled {
		t.Fatal("expected remote fallback get call")
	}
	if got := serverManager.lastResource.LogicalPath; got != "/admin/realms" {
		t.Fatalf("expected normalized remote logical path /admin/realms, got %q", got)
	}
	if got := serverManager.lastResource.CollectionPath; got != "/admin" {
		t.Fatalf("expected remote collection path /admin, got %q", got)
	}
	if !reflect.DeepEqual(value, map[string]any{"realm": "master"}) {
		t.Fatalf("unexpected remote payload: %#v", value)
	}
}

func TestDefaultOrchestratorGetRemoteFallbackSeedsIdentityFromMetadata(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Repository: &fakeRepository{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		},
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "realm",
				AliasFromAttribute: "realm",
			},
		},
		Server: &fakeServer{
			getValue: map[string]any{"realm": "platform"},
		},
	}

	_, err := orchestrator.Get(context.Background(), "/admin/realms/platform")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	serverManager := orchestrator.Server.(*fakeServer)
	payload, ok := serverManager.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", serverManager.lastResource.Payload)
	}
	if got := payload["realm"]; got != "platform" {
		t.Fatalf("expected metadata-seeded realm identity, got %#v", got)
	}
	if got := serverManager.lastResource.RemoteID; got != "platform" {
		t.Fatalf("expected remote id platform, got %q", got)
	}
}

func TestDefaultOrchestratorGetRemoteFallsBackToCollectionListByAlias(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
			},
		},
		Server: &fakeServer{
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

	serverManager := orchestrator.Server.(*fakeServer)
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
	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from fallback list item, got %T", value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected alias-matched payload, got %#v", payload)
	}
}

func TestDefaultOrchestratorGetRemoteUsesSingleJQFilteredCandidateFallback(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
				CollectionPath:     "/admin/realms/{{.realm}}/components",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						JQ: `[ .[] | select(.providerId == "ldap") ]`,
					},
				},
			},
		},
		Server: serverManager,
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

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", value)
	}
	if payload["id"] != "13de4420-7c8d-4db7-b8f7-2d2a26f2053e" {
		t.Fatalf("unexpected payload %#v", payload)
	}
}

func TestDefaultOrchestratorGetRemoteDoesNotCollapseExplicitChildToSingletonJQCandidate(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
				CollectionPath:     "/admin/realms/{{.realm}}/components",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						JQ: `[ .[] | select(.providerId == "ldap") ]`,
					},
				},
			},
		},
		Server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), requestPath)
	assertTypedCategory(t, err, faults.NotFoundError)

	for _, path := range serverManager.getPaths {
		if path == expectedSingletonAliasPath {
			t.Fatalf("expected explicit child lookup to keep NotFound and avoid singleton alias collapse, get calls %#v", serverManager.getPaths)
		}
	}
}

func TestDefaultOrchestratorGetRemoteDoesNotUseSingleCandidateFallbackWithoutJQ(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
				CollectionPath:     "/admin/realms/{{.realm}}/components",
			},
		},
		Server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), requestPath)
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestDefaultOrchestratorGetRemoteResolvesAliasPathToMetadataIDBeforeCollectionFallback(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValues: map[string]metadatadomain.ResourceMetadata{
				aliasPath: {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
				resolvedIDPath: {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
			},
		},
		Server: serverManager,
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

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from resolved id read, got %T", value)
	}
	if payload["id"] != "71ba388d-9f95-4a4d-b674-a632f697b732" || payload["alias"] != "teste" {
		t.Fatalf("unexpected payload from metadata id fallback: %#v", payload)
	}
}

func TestDefaultOrchestratorGetRemoteRecursivelyResolvesParentMetadataIdentity(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValues: map[string]metadatadomain.ResourceMetadata{
				aliasPath: {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
				resolvedRealmPath: {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
				resolvedResourcePath: {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
				"/admin/realms/publico-br": {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
				"/admin/realms/realm-1": {
					IDFromAttribute:    "id",
					AliasFromAttribute: "alias",
				},
			},
		},
		Server: serverManager,
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

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", value)
	}
	if payload["id"] != "org-1" || payload["alias"] != "teste" {
		t.Fatalf("unexpected recursively-resolved payload: %#v", payload)
	}
}

func TestDefaultOrchestratorGetRemoteKeepsOriginalNotFoundWhenRecursiveFallbackProbeResponseIsInvalid(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "alias",
			},
		},
		Server: serverManager,
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

func TestDefaultOrchestratorGetRemoteTreatsCollectionNotFoundAsEmptyWhenOpenAPIHintsCollection(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected empty list payload, got %T", value)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list payload, got %#v", items)
	}
	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/master/organizations"}) {
		t.Fatalf("expected direct collection list fallback path, got %#v", serverManager.listPaths)
	}
}

func TestDefaultOrchestratorGetRemoteTreatsCollectionNotFoundAsEmptyWhenRepositoryHintsCollection(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Repository: repositoryManager,
		Server:     serverManager,
	}

	value, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	if err != nil {
		t.Fatalf("GetRemote returned error: %v", err)
	}

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected empty list payload, got %T", value)
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

func TestDefaultOrchestratorGetRemoteDoesNotTreatNotFoundAsEmptyWithoutOpenAPIOrRepositoryHints(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		),
	}
	orchestrator := &DefaultOrchestrator{
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						Method: "GET",
						Path:   "/admin/realms/{{.realm}}/organizations",
					},
				},
			},
		},
		Server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/master/organizations")
	assertTypedCategory(t, err, faults.NotFoundError)

	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/master"}) {
		t.Fatalf("expected parent collection fallback only, got %#v", serverManager.listPaths)
	}
}

func TestDefaultOrchestratorGetRemoteKeepsNotFoundForConcreteResourcePathWithOpenAPIChildEndpoints(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
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

func TestDefaultOrchestratorGetRemoteKeepsNotFoundForCollectionWhenParentResourceIsMissing(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
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

func TestDefaultOrchestratorGetRemoteKeepsNotFoundWhenParentFallbackListPayloadIsInvalid(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listErr: serverdomain.NewListPayloadShapeError(
			`list response object is ambiguous: expected an "items" array or a single array field`,
			nil,
		),
	}
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
	}

	_, err := orchestrator.GetRemote(context.Background(), "/admin/realms/publico/organizatio")
	assertTypedCategory(t, err, faults.NotFoundError)

	if !reflect.DeepEqual(serverManager.listPaths, []string{"/admin/realms/publico"}) {
		t.Fatalf("expected parent collection fallback path, got %#v", serverManager.listPaths)
	}
}

func TestDefaultOrchestratorGetLocalFallsBackToCollectionListByMetadataID(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Repository: repositoryManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
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

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected id-based local fallback payload, got %#v", payload)
	}
}

func TestDefaultOrchestratorGetLocalFallsBackToCommonIDAttributeWhenMetadataUsesAlias(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Repository: repositoryManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "clientId",
				AliasFromAttribute: "clientId",
			},
		},
	}

	value, err := orchestrator.GetLocal(context.Background(), "/admin/realms/platform/clients/client-0002")
	if err != nil {
		t.Fatalf("GetLocal returned error: %v", err)
	}

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value)
	}
	if payload["clientId"] != "web-console" || payload["id"] != "client-0002" {
		t.Fatalf("expected common-id fallback payload, got %#v", payload)
	}
}

func TestDefaultOrchestratorGetLocalFallbackPrefersListedAliasWithoutFullScan(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Repository: repositoryManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
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

	payload, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload from local fallback, got %T", value)
	}
	if payload["clientId"] != "account" {
		t.Fatalf("expected alias-matched fallback payload, got %#v", payload)
	}
}

func TestDefaultOrchestratorApplyResolvesLocalPathByMetadataIDFallback(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Repository: repositoryManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
			},
		},
		Server: &fakeServer{
			existsValue: true,
			updateValue: map[string]any{
				"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
				"clientId": "account",
			},
		},
	}

	item, err := orchestrator.Apply(context.Background(), "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c")
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	serverManager := orchestrator.Server.(*fakeServer)
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

func TestDefaultOrchestratorDeleteRetriesWithResolvedRemoteIdentityAfterNotFound(t *testing.T) {
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

	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
			},
		},
	}

	if err := orchestrator.Delete(context.Background(), "/admin/realms/master/clients/account", DeletePolicy{}); err != nil {
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

func TestDefaultOrchestratorRequestDelegatesToServer(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: map[string]any{"ok": true},
	}
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
	}

	body := resource.Value(map[string]any{"id": "a"})
	value, err := orchestrator.Request(context.Background(), "POST", "/test", body)
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
	if !reflect.DeepEqual(value, map[string]any{"ok": true}) {
		t.Fatalf("unexpected request response: %#v", value)
	}
}

func TestDefaultOrchestratorRequestPostResolvesPathFromMetadata(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: map[string]any{"ok": true},
	}
	metadataService := &fakeMetadata{
		resolveValue: metadatadomain.ResourceMetadata{
			IDFromAttribute:    "id",
			AliasFromAttribute: "name",
			CollectionPath:     "/admin/realms/{{.realm}}/components",
		},
	}
	orchestrator := &DefaultOrchestrator{
		Server:   serverManager,
		Metadata: metadataService,
	}

	body := resource.Value(map[string]any{
		"providerId": "ldap",
		"name":       "AD Production",
	})
	_, err := orchestrator.Request(context.Background(), "POST", "/admin/realms/acme/user-registry/", body)
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

func TestDefaultOrchestratorRequestGetSelectorDepthResolvesListPathFromMetadata(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestValue: []any{},
	}
	metadataService := &fakeMetadata{
		resolveValue: metadatadomain.ResourceMetadata{
			IDFromAttribute:    "id",
			AliasFromAttribute: "name",
			CollectionPath:     "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationList): {
					JQ: "[ .[] | select(.providerId == \"ldap\") ]",
				},
			},
		},
	}
	orchestrator := &DefaultOrchestrator{
		Server:   serverManager,
		Metadata: metadataService,
	}

	_, err := orchestrator.Request(context.Background(), "GET", "/admin/realms/master/user-registry", nil)
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

func TestDefaultOrchestratorRequestGetFallsBackToMetadataAwareRemoteReadAfterNotFound(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "clientId",
			},
		},
	}

	value, err := orchestrator.Request(context.Background(), "GET", "/admin/realms/master/clients/account", nil)
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	if len(serverManager.requestPaths) != 1 || serverManager.requestPaths[0] != "/admin/realms/master/clients/account" {
		t.Fatalf("expected one literal request GET attempt before fallback, got %#v", serverManager.requestPaths)
	}
	got, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected fallback GET payload object, got %T", value)
	}
	if got["clientId"] != "account" {
		t.Fatalf("expected fallback GET payload to include resolved clientId, got %#v", got)
	}
}

func TestDefaultOrchestratorRequestDeleteRetriesWithResolvedRemoteIdentityAfterNotFound(t *testing.T) {
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
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "alias",
			},
		},
	}

	_, err := orchestrator.Request(context.Background(), "DELETE", "/admin/realms/acme/organizations/alpha", nil)
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

func TestDefaultOrchestratorRequestPutCollectionPathRetriesLiteralAfterResolvedNotFound(t *testing.T) {
	t.Parallel()

	serverManager := &fakeServer{
		requestErrs: []error{
			faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			nil,
		},
		requestValue: map[string]any{"ok": true},
	}
	orchestrator := &DefaultOrchestrator{
		Server: serverManager,
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "displayName",
			},
		},
	}

	body := resource.Value(map[string]any{
		"id":          "bd43239f-2111-4c03-83b6-c0902df38ec9",
		"requirement": "ALTERNATIVE",
	})

	value, err := orchestrator.Request(
		context.Background(),
		"PUT",
		"/admin/realms/acme/authentication/flows/test/executions",
		body,
	)
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
	if !reflect.DeepEqual(value, map[string]any{"ok": true}) {
		t.Fatalf("unexpected request response: %#v", value)
	}
}

func TestDefaultOrchestratorRequestRequiresServer(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}
	_, err := orchestrator.Request(context.Background(), "GET", "/test", nil)
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

	savedPath   string
	savedValue  resource.Value
	getCalls    []string
	listCalls   []string
	existsCalls []string

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func (f *fakeRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	f.savedPath = logicalPath
	f.savedValue = value
	return nil
}

func (f *fakeRepository) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	f.getCalls = append(f.getCalls, logicalPath)

	if f.getErr != nil {
		return nil, f.getErr
	}

	if f.getValues != nil {
		if value, found := f.getValues[logicalPath]; found {
			return value, nil
		}
		return nil, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
	}

	return f.getValue, nil
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
func (f *fakeRepository) Move(context.Context, string, string) error { return nil }
func (f *fakeRepository) Init(context.Context) error                 { return nil }
func (f *fakeRepository) Refresh(context.Context) error              { return nil }
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
	updateValue  resource.Value
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
	requestBody     resource.Value
	lastResource    resource.Resource
	lastListPath    string
	listPaths       []string
	openAPISpec     resource.Value
	openAPIErr      error
	deleteResources []resource.Resource
}

func (f *fakeServer) Get(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.getCalled = true
	f.lastResource = resourceInfo
	f.getPaths = append(f.getPaths, resourceInfo.LogicalPath)
	if f.getValues != nil {
		if value, found := f.getValues[resourceInfo.LogicalPath]; found {
			return value, nil
		}
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeServer) Create(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.createCalled = true
	f.lastResource = resourceInfo
	return f.createValue, nil
}

func (f *fakeServer) Update(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.updateCalled = true
	f.lastResource = resourceInfo
	return f.updateValue, nil
}

func (f *fakeServer) Delete(_ context.Context, resourceInfo resource.Resource) error {
	f.deleteCalled = true
	f.lastResource = resourceInfo
	f.deleteResources = append(f.deleteResources, resourceInfo)
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

func (f *fakeServer) Exists(context.Context, resource.Resource) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.existsValue, nil
}

func (f *fakeServer) Request(_ context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error) {
	f.requestCalled = true
	f.requestMethod = method
	f.requestPath = endpointPath
	f.requestPaths = append(f.requestPaths, endpointPath)
	f.requestBody = body
	if len(f.requestErrs) > 0 {
		err := f.requestErrs[0]
		f.requestErrs = f.requestErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if f.requestErr != nil {
		return nil, f.requestErr
	}
	return f.requestValue, nil
}

func (f *fakeServer) GetOpenAPISpec(context.Context) (resource.Value, error) {
	if f.openAPIErr != nil {
		return nil, f.openAPIErr
	}
	return f.openAPISpec, nil
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

func TestDefaultOrchestratorApplyUsesSecretsForRemoteMutation(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":       "42",
			"alias":    "acme",
			"apiToken": "{{ secret . }}",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{.id}}"},
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
			"/customers/acme:apiToken": "super-secret",
		},
	}

	orchestrator := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
		Secrets:    secretProvider,
	}

	item, err := orchestrator.Apply(context.Background(), "/customers/acme")
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

func TestDefaultOrchestratorDiffUsesFallbackAndCompareSuppressRules(t *testing.T) {
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
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Suppress: []string{"/updatedAt"}},
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
			"/customers/acme:apiToken": "super-secret",
		},
	}

	orchestrator := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
		Secrets:    secretProvider,
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

func TestDefaultOrchestratorDiffTreatsMissingRemoteResourceAsDrift(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "42",
			"alias": "acme",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{.id}}"},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
	}

	orchestrator := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
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

func TestDefaultOrchestratorDiffReturnsConflictOnAmbiguousFallback(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "42",
			"alias": "acme",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{.id}}"},
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

	orchestrator := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
	}

	_, err := orchestrator.Diff(context.Background(), "/customers/acme")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestDefaultOrchestratorDiffReturnsConflictWhenDirectGetIdentityIsAmbiguous(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "remote-two",
			"alias": "remote-one",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{.id}}"},
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

	orchestrator := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
	}

	_, err := orchestrator.Diff(context.Background(), "/customers/local")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestDefaultOrchestratorListRemoteSortsDeterministically(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Repository: &fakeRepository{},
		Metadata:   &fakeMetadata{resolveValue: metadatadomain.ResourceMetadata{}},
		Server: &fakeServer{
			listValue: []resource.Resource{
				{LogicalPath: "/customers/zeta"},
				{LogicalPath: "/customers/acme"},
			},
		},
	}

	items, err := orchestrator.ListRemote(context.Background(), "/customers", ListPolicy{})
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

func TestDefaultOrchestratorListRemoteProvidesListJQResourceResolver(t *testing.T) {
	t.Parallel()

	metadataService := &fakeMetadata{
		resolveValues: map[string]metadatadomain.ResourceMetadata{
			"/admin/realms/publico-br/user-registry/ldap-test/mappers": {
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
			},
			"/admin/realms/publico-br/user-registry/ldap-test": {
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationList): {
						JQ: `[ .[] | select(.providerId == "ldap") ]`,
					},
				},
			},
			"/admin/realms/publico-br/user-registry": {
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
			},
		},
	}

	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listFunc: func(ctx context.Context, logicalPath string, _ metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
			switch logicalPath {
			case "/admin/realms/publico-br/user-registry/ldap-test/mappers":
				parentValue, resolved, resolveErr := serverdomain.ResolveListJQResource(
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

	orchestrator := &DefaultOrchestrator{
		Repository: &fakeRepository{},
		Metadata:   metadataService,
		Server:     serverManager,
	}

	items, err := orchestrator.ListRemote(
		context.Background(),
		"/admin/realms/publico-br/user-registry/ldap-test/mappers",
		ListPolicy{},
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

func TestDefaultOrchestratorTemplateReturnsNormalizedPayload(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Repository: &fakeRepository{},
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{.id}}"},
				},
			},
		},
	}
	orchestrator.SetResourceFormat("yaml")

	templated, err := orchestrator.Template(context.Background(), "/customers/acme", map[string]any{
		"id":     "42",
		"name":   "ACME",
		"count":  float64(2),
		"format": "{{resource_format .}}",
		"token":  "{{secret .}}",
	})
	if err != nil {
		t.Fatalf("Template returned error: %v", err)
	}

	output, ok := templated.(map[string]any)
	if !ok {
		t.Fatalf("expected templated map output, got %T", templated)
	}
	if got := output["name"]; got != "ACME" {
		t.Fatalf("expected templated payload to preserve values, got %#v", got)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected resource_format placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "{{secret .}}" {
		t.Fatalf("expected secret placeholder to remain unresolved in template output, got %#v", got)
	}
}

func TestDefaultOrchestratorResolvePayloadForRemoteSupportsResourceFormatWithoutSecrets(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}
	orchestrator.SetResourceFormat("yaml")

	resolved, err := orchestrator.resolvePayloadForRemote(
		context.Background(),
		"/customers/acme",
		map[string]any{
			"format": "{{resource_format .}}",
			"token":  "{{secret .}}",
		},
	)
	if err != nil {
		t.Fatalf("resolvePayloadForRemote returned error: %v", err)
	}

	output, ok := resolved.(map[string]any)
	if !ok {
		t.Fatalf("expected resolved payload map, got %T", resolved)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected resource_format placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "{{secret .}}" {
		t.Fatalf("expected secret placeholder to remain unresolved without secret provider, got %#v", got)
	}
}

func TestDefaultOrchestratorResolvePayloadForRemoteSupportsResourceFormatWithSecrets(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{
		Secrets: &fakeSecretProvider{
			values: map[string]string{
				"/customers/acme:token": "super-secret",
			},
		},
	}
	orchestrator.SetResourceFormat("yaml")

	resolved, err := orchestrator.resolvePayloadForRemote(
		context.Background(),
		"/customers/acme",
		map[string]any{
			"format": "{{resource_format .}}",
			"token":  "{{secret .}}",
		},
	)
	if err != nil {
		t.Fatalf("resolvePayloadForRemote returned error: %v", err)
	}

	output, ok := resolved.(map[string]any)
	if !ok {
		t.Fatalf("expected resolved payload map, got %T", resolved)
	}
	if got := output["format"]; got != "yaml" {
		t.Fatalf("expected resource_format placeholder to resolve to yaml, got %#v", got)
	}
	if got := output["token"]; got != "super-secret" {
		t.Fatalf("expected secret placeholder to resolve, got %#v", got)
	}
}

func TestDefaultOrchestratorRenderOperationSpecListUsesCollectionPathFallback(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}
	resourceInfo := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
		Metadata: metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{},
		},
	}

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resourceInfo,
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

func TestDefaultOrchestratorRenderOperationSpecCreateUsesCollectionPathFallback(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}
	resourceInfo := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
		Metadata: metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{},
		},
	}

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resourceInfo,
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

func TestDefaultOrchestratorRenderOperationSpecSupportsResourceFormatTemplateFunc(t *testing.T) {
	t.Parallel()

	orchestrator := &DefaultOrchestrator{}
	orchestrator.SetResourceFormat("yaml")

	spec, err := orchestrator.renderOperationSpec(
		context.Background(),
		resource.Resource{
			LogicalPath:    "/customers/acme",
			CollectionPath: "/customers",
			Metadata: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationGet): {
						Path:   "/api/customers/{{.id}}",
						Accept: "application/{{resource_format .}}",
					},
				},
			},
			Payload: map[string]any{"id": "42"},
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
