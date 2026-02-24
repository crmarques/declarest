package cli

import (
	"errors"
	"strings"
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestCompletionBashGeneratesScript(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "completion", "bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "declarest") {
		t.Fatalf("expected completion script output, got %q", output)
	}
	if strings.Contains(output, "--output=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
	if strings.Contains(output, "--context=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
	if strings.Contains(output, "--path=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
}

func TestPathCompletionResourceGetPrefersRemoteAndOpenAPI(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/customers/local"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/customers/remote"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/customers/{id}": map[string]any{},
			"/health":         map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/customers")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/customers/") {
		t.Fatalf("expected next-level collection completion, got %q", output)
	}
	if strings.Contains(output, "/customers/remote") || strings.Contains(output, "/customers/local") {
		t.Fatalf("expected get completion to collapse to next-level candidates, got %q", output)
	}
	if len(orchestrator.listLocalCalls) > 0 {
		t.Fatalf("expected get completion to skip local list when remote suggestions are available, calls=%#v", orchestrator.listLocalCalls)
	}
	if !strings.Contains(output, ":6") {
		t.Fatalf("expected no-file+no-space completion directive, got %q", output)
	}
}

func TestPathCompletionExpandsOpenAPITemplatesFromRemoteCollectionItems(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/master/clients/local-app"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/prod"},
		{LogicalPath: "/admin/realms/master/clients/remote-app"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/master/clients/remote-app") {
		t.Fatalf("expected remote collection item completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/clients/local-app") {
		t.Fatalf("expected get completion to avoid local fallback when remote collection items are available, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected completion to consult remote collection path, calls=%#v", orchestrator.listRemoteCalls)
	}
	if containsString(orchestrator.listLocalCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected completion to skip local collection fallback when remote items are available, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionResourceGetIncludesRepositoryCandidatesWhenRemoteMatchesOnlyOpenAPI(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/publico-br/user-registry"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/publico-br"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/users": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/publico-br/user")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/publico-br/users\n") {
		t.Fatalf("expected openapi-backed completion candidate, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/publico-br/user-registry\n") {
		t.Fatalf("expected repository-backed completion candidate, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/publico-br") {
		t.Fatalf("expected completion to query remote scoped path, calls=%#v", orchestrator.listRemoteCalls)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/publico-br") {
		t.Fatalf("expected completion to query repository scoped path when remote has no direct candidates, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionFallsBackToRepositoryWhenRemoteUnavailable(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/master/clients/local-app"},
	}
	orchestrator.listRemoteErr = errors.New("remote unavailable")
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/clients/local-app") {
		t.Fatalf("expected repository fallback completion item, got %q", output)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected fallback completion to consult local collection path, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionResourceApplyPrefersRepository(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/customers/local"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/customers/remote"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "apply", "/customers")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/customers/") {
		t.Fatalf("expected next-level repository collection completion, got %q", output)
	}
	if strings.Contains(output, "/customers/remote") || strings.Contains(output, "/customers/local") {
		t.Fatalf("expected apply completion to collapse to next-level candidates, got %q", output)
	}
	if len(orchestrator.listRemoteCalls) > 0 {
		t.Fatalf("expected apply completion to skip remote list when local suggestions are available, calls=%#v", orchestrator.listRemoteCalls)
	}
}

func TestPathCompletionUsesAliasAttributeForCollectionItems(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/clients/f88c68f3",
			CollectionPath: "/admin/realms/master/clients",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "clientId",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":       "f88c68f3",
				"clientId": "account",
			},
		},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/clients/account") {
		t.Fatalf("expected alias-based completion item, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/clients/f88c68f3") {
		t.Fatalf("expected completion to hide id-based segment when alias is available, got %q", output)
	}
}

func TestPathCompletionPreservesAliasesWithSpaces(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/publico-br/user-registry/13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
			CollectionPath: "/admin/realms/publico-br/user-registry",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/publico-br/user-registry/A",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/publico-br/user-registry/AD PRD") {
		t.Fatalf("expected completion output to keep alias spaces, got %q", output)
	}
}

func TestPathCompletionEscapedExactAliasFallsBackToRepositoryDescendants(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/user-registry/13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
			CollectionPath: "/admin/realms/master/user-registry",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha"},
	}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/master/user-registry/AD\\ PRD",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/") {
		t.Fatalf("expected escaped alias completion to advance into collection scope, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/master/user-registry") {
		t.Fatalf("expected completion to query remote parent collection first, calls=%#v", orchestrator.listRemoteCalls)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/user-registry") {
		t.Fatalf("expected completion to fallback to repository when remote result does not advance token, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionEscapedCollectionTokenCompletesNextSegment(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/user-registry/13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
			CollectionPath: "/admin/realms/master/user-registry",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha"},
	}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/master/user-registry/AD\\ PRD/",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/mappers/") {
		t.Fatalf("expected escaped collection completion to resolve next segment from repository, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/AD PRD") {
		t.Fatalf("expected completion to avoid repeating escaped alias fragment, got %q", output)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/user-registry/AD PRD") {
		t.Fatalf("expected repository completion query for escaped collection path, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionAvoidsSelfAliasSegmentDuplication(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/user-registry/AD PRD",
			CollectionPath: "/admin/realms/master/user-registry/AD PRD",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha"},
	}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/master/user-registry/AD\\ PRD/",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/AD PRD") {
		t.Fatalf("expected completion to avoid duplicated self alias segment, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/mappers/") {
		t.Fatalf("expected completion to include next segment from repository fallback, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/master/user-registry/AD PRD") {
		t.Fatalf("expected completion to query remote scoped path first, calls=%#v", orchestrator.listRemoteCalls)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/user-registry/AD PRD") {
		t.Fatalf("expected completion to fallback to repository for non-advancing remote candidates, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionAvoidsRepeatedAliasChildSuggestion(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/user-registry/AD PRD/AD PRD",
			CollectionPath: "/admin/realms/master/user-registry/AD PRD",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha"},
	}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/master/user-registry/AD\\ PRD/",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/AD PRD") {
		t.Fatalf("expected completion to suppress repeated alias child suggestion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/mappers/") {
		t.Fatalf("expected completion to include repository-descendant next segment, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/master/user-registry/AD PRD") {
		t.Fatalf("expected completion to query remote scoped path first, calls=%#v", orchestrator.listRemoteCalls)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/user-registry/AD PRD") {
		t.Fatalf("expected completion to fallback to repository for repeated alias child candidates, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionUsesMetadataOnlyBranchWhenOpenAPIHasNoPath(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/user-registry/AD PRD/AD PRD",
			CollectionPath: "/admin/realms/master/user-registry/AD PRD",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "name",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":   "13de4420-7c8d-4db7-b8f7-2d2a26f2053e",
				"name": "AD PRD",
			},
		},
	}
	orchestrator.localList = nil
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{},
	}
	metadataService := deps.Metadata.(*testMetadata)
	metadataService.collectionChildren["/admin/realms/master/user-registry/AD PRD"] = []string{"mappers"}

	output, err := executeForTest(
		deps,
		"",
		"__complete",
		"resource",
		"get",
		"/admin/realms/master/user-registry/AD\\ PRD/",
	)
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/mappers") {
		t.Fatalf("expected completion to include metadata-defined child branch, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/user-registry/AD PRD/AD PRD") {
		t.Fatalf("expected completion to avoid repeated alias child path, got %q", output)
	}
}

func TestPathCompletionRendersCollectionsWithTrailingSlash(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/clients/remote-app"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/adm")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/\n") {
		t.Fatalf("expected collection prefix with trailing slash, got %q", output)
	}
	if strings.Contains(output, "/admin\n") {
		t.Fatalf("expected collection prefix to render with trailing slash only, got %q", output)
	}
}

func TestPathCompletionCompletesScopedPrefixToCanonicalPath(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/prod"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/rea")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/\n") {
		t.Fatalf("expected scoped completion to include canonical /admin/realms/, got %q", output)
	}
	if strings.Contains(output, "/realms\n") {
		t.Fatalf("expected completion to avoid fragment-only candidate output, got %q", output)
	}
}

func TestPathCompletionNoDescRemainsPrefixCompatibleForBash(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__completeNoDesc", "resource", "get", "/admin/rea")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/") {
		t.Fatalf("expected __completeNoDesc to emit prefix-compatible canonical candidates, got %q", output)
	}
	if strings.Contains(output, "\n/realms\n") {
		t.Fatalf("expected __completeNoDesc to avoid fragment-only candidates, got %q", output)
	}
}

func TestPathCompletionShowsOnlyNextLevelForCollectionPrefix(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/alpha/clients/app-a"},
		{LogicalPath: "/admin/realms/beta/roles/viewer"},
		{LogicalPath: "/admin/realms/gamma"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/alpha/\n") {
		t.Fatalf("expected alpha realm next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/beta/\n") {
		t.Fatalf("expected beta realm next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/gamma\n") {
		t.Fatalf("expected gamma realm completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/alpha/clients") {
		t.Fatalf("expected collection prefix completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/beta/roles") {
		t.Fatalf("expected collection prefix completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "{realm}") {
		t.Fatalf("expected completion to suppress templated placeholders, got %q", output)
	}
}

func TestPathCompletionShowsOnlyNextLevelForNestedCollectionPrefix(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/aaaa/resource-a"},
		{LogicalPath: "/admin/realms/master/bbbb"},
		{LogicalPath: "/admin/realms/master/cccc/deeper/resource-c"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/master/aaaa/\n") {
		t.Fatalf("expected aaaa next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/bbbb\n") {
		t.Fatalf("expected bbbb next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/cccc/\n") {
		t.Fatalf("expected cccc next-level completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/aaaa/resource-a") {
		t.Fatalf("expected nested collection completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/cccc/deeper") {
		t.Fatalf("expected nested collection completion to omit deep descendants, got %q", output)
	}
}

func TestPathCompletionScopedQueriesAvoidRootRecursiveFallbackWhenScopedMatchesExist(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/alpha"},
		{LogicalPath: "/admin/realms/beta"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/alpha") {
		t.Fatalf("expected scoped completion output, got %q", output)
	}
	if !containsListCall(orchestrator.listRemoteDetail, "/admin/realms", false) {
		t.Fatalf("expected scoped non-recursive query for /admin/realms, calls=%#v", orchestrator.listRemoteDetail)
	}
	if containsListCall(orchestrator.listRemoteDetail, "/", true) {
		t.Fatalf("expected scoped completion to avoid root-recursive remote fallback when scoped candidates exist, calls=%#v", orchestrator.listRemoteDetail)
	}
}

func TestContextFlagCompletionShowsContextNames(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "get", "--context", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "dev") || !strings.Contains(output, "prod") {
		t.Fatalf("expected context names in completion output, got %q", output)
	}
}

func TestOutputFlagCompletionShowsSupportedValues(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "get", "--output", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	expected := []string{"auto", "text", "json", "yaml"}
	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Fatalf("expected %q output completion value, got %q", value, output)
		}
	}
}

func TestResourceListCompletionShowsSourceFlags(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "list", "--")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "--source") {
		t.Fatalf("expected source flag completion values, got %q", output)
	}
}

func TestResourceSourceFlagCompletionShowsSupportedValues(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "get", "--source", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "repository") || !strings.Contains(output, "remote-server") {
		t.Fatalf("expected source values in completion output, got %q", output)
	}
}

func TestPathCompletionAvailableForAllPathAwareCommands(t *testing.T) {
	t.Parallel()

	command := NewRootCommand(testDeps())
	pathAwareCommands := [][]string{
		{"resource", "get"},
		{"resource", "save"},
		{"resource", "apply"},
		{"resource", "create"},
		{"resource", "update"},
		{"resource", "delete"},
		{"resource", "diff"},
		{"resource", "list"},
		{"resource", "explain"},
		{"resource", "template"},
		{"resource", "request", "get"},
		{"resource", "request", "head"},
		{"resource", "request", "options"},
		{"resource", "request", "post"},
		{"resource", "request", "put"},
		{"resource", "request", "patch"},
		{"resource", "request", "delete"},
		{"resource", "request", "trace"},
		{"resource", "request", "connect"},
		{"metadata", "get"},
		{"metadata", "set"},
		{"metadata", "unset"},
		{"metadata", "resolve"},
		{"metadata", "render"},
		{"metadata", "infer"},
		{"secret", "get"},
		{"secret", "detect"},
	}

	for _, commandPath := range pathAwareCommands {
		commandPath := append([]string{}, commandPath...)
		t.Run(joinPath(commandPath), func(t *testing.T) {
			target := commandByPath(command, commandPath...)
			if target == nil {
				t.Fatalf("expected command path %q to exist", joinPath(commandPath))
			}
			if target.Flags().Lookup("path") == nil {
				t.Fatalf("expected command %q to declare --path", joinPath(commandPath))
			}
			if _, found := target.GetFlagCompletionFunc("path"); !found {
				t.Fatalf("expected command %q to register --path completion", joinPath(commandPath))
			}
			if target.ValidArgsFunction == nil {
				t.Fatalf("expected command %q to register positional path completion", joinPath(commandPath))
			}
		})
	}
}

func TestPathCompletionResourceSourceFlagsSwitchCompletionTarget(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		args            []string
		expectLocal     bool
		expectRemote    bool
		expectedSnippet string
	}{
		{
			name:            "get_default_prefers_remote",
			args:            []string{"resource", "get", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_repository_prefers_local",
			args:            []string{"resource", "get", "--source", "repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_remote_server_prefers_remote",
			args:            []string{"resource", "get", "--source", "remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_repository_path_flag_uses_local",
			args:            []string{"resource", "get", "--source", "repository", "--path", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "list_repository_prefers_local",
			args:            []string{"resource", "list", "--source", "repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "list_remote_server_prefers_remote",
			args:            []string{"resource", "list", "--source", "remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_repository_prefers_local",
			args:            []string{"resource", "delete", "--confirm-delete", "--source", "repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_remote_server_prefers_remote",
			args:            []string{"resource", "delete", "--confirm-delete", "--source", "remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_both_queries_both_sources",
			args:            []string{"resource", "delete", "--confirm-delete", "--source", "both", "/customers"},
			expectLocal:     true,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			deps := testDeps()
			orchestrator := deps.Orchestrator.(*testOrchestrator)
			orchestrator.localList = []resource.Resource{
				{LogicalPath: "/customers/local"},
			}
			orchestrator.remoteList = []resource.Resource{
				{LogicalPath: "/customers/remote"},
			}

			completeArgs := append([]string{"__complete"}, testCase.args...)
			output, err := executeForTest(deps, "", completeArgs...)
			if err != nil {
				t.Fatalf("unexpected completion error: %v", err)
			}
			if !strings.Contains(output, testCase.expectedSnippet) {
				t.Fatalf("expected completion output %q, got %q", testCase.expectedSnippet, output)
			}
			if !strings.Contains(output, ":6") {
				t.Fatalf("expected path completion directive :6, got %q", output)
			}

			localCalled := len(orchestrator.listLocalCalls) > 0
			remoteCalled := len(orchestrator.listRemoteCalls) > 0

			if localCalled != testCase.expectLocal {
				t.Fatalf("expected local completion queries=%t, got %t (calls=%#v)", testCase.expectLocal, localCalled, orchestrator.listLocalDetail)
			}
			if remoteCalled != testCase.expectRemote {
				t.Fatalf("expected remote completion queries=%t, got %t (calls=%#v)", testCase.expectRemote, remoteCalled, orchestrator.listRemoteDetail)
			}
		})
	}
}

func TestPathCompletionMetadataAndSecretPreferRepositoryPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
	}{
		{name: "metadata_get", args: []string{"metadata", "get", "/customers"}},
		{name: "secret_get", args: []string{"secret", "get", "/customers"}},
		{name: "secret_detect", args: []string{"secret", "detect", "/customers"}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			deps := testDeps()
			orchestrator := deps.Orchestrator.(*testOrchestrator)
			orchestrator.localList = []resource.Resource{
				{LogicalPath: "/customers/local"},
			}
			orchestrator.remoteList = []resource.Resource{
				{LogicalPath: "/customers/remote"},
			}

			completeArgs := append([]string{"__complete"}, testCase.args...)
			output, err := executeForTest(deps, "", completeArgs...)
			if err != nil {
				t.Fatalf("unexpected completion error: %v", err)
			}
			if !strings.Contains(output, "/customers/") {
				t.Fatalf("expected completion output, got %q", output)
			}
			if len(orchestrator.listLocalCalls) == 0 {
				t.Fatalf("expected completion to query local repository source")
			}
			if len(orchestrator.listRemoteCalls) != 0 {
				t.Fatalf("expected completion to skip remote fallback when repository candidates exist, calls=%#v", orchestrator.listRemoteDetail)
			}
		})
	}
}

func TestMetadataRenderCompletionSuggestsOperations(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "metadata", "render", "/customers", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	expected := []string{"get", "create", "update", "delete", "list", "compare"}
	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Fatalf("expected operation completion value %q, got %q", value, output)
		}
	}

	withPathFlagOutput, err := executeForTest(testDeps(), "", "__complete", "metadata", "render", "--path", "/customers", "")
	if err != nil {
		t.Fatalf("unexpected completion error with --path: %v", err)
	}
	for _, value := range expected {
		if !strings.Contains(withPathFlagOutput, value) {
			t.Fatalf("expected operation completion value %q with --path, got %q", value, withPathFlagOutput)
		}
	}
}
