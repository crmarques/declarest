package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestResourceCopySourceLookup(t *testing.T) {
	t.Parallel()

	t.Run("uses local source when available", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestratorService := &testOrchestrator{
			metadataService: metadataService,
			getLocalValues: map[string]resource.Value{
				"/admin/realms/test": map[string]any{"realm": "test", "source": "local"},
			},
		}
		deps := newResourceSaveDeps(orchestratorService, metadataService)

		if _, err := executeForTest(deps, "", "resource", "copy", "/admin/realms/test", "/admin/realms/test-copy"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestratorService.getLocalCalls) != 1 || orchestratorService.getLocalCalls[0] != "/admin/realms/test" {
			t.Fatalf("expected one local read for source path, got %#v", orchestratorService.getLocalCalls)
		}
		if len(orchestratorService.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote reads on local hit, got %#v", orchestratorService.getRemoteCalls)
		}
		if len(orchestratorService.saveCalls) != 1 {
			t.Fatalf("expected one save call, got %#v", orchestratorService.saveCalls)
		}
		if orchestratorService.saveCalls[0].logicalPath != "/admin/realms/test-copy" {
			t.Fatalf("expected save target path %q, got %q", "/admin/realms/test-copy", orchestratorService.saveCalls[0].logicalPath)
		}
		if !reflect.DeepEqual(orchestratorService.saveCalls[0].value, map[string]any{"realm": "test", "source": "local"}) {
			t.Fatalf("expected local payload to be copied, got %#v", orchestratorService.saveCalls[0].value)
		}
	})

	t.Run("does not fall back to remote source when local resource is not found", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestratorService := &testOrchestrator{
			metadataService: metadataService,
			getLocalErr: faults.NewTypedError(
				faults.NotFoundError,
				`resource "/admin/realms/test" not found`,
				nil,
			),
			getRemoteValues: map[string]resource.Value{
				"/admin/realms/test": map[string]any{"realm": "test", "source": "remote"},
			},
		}
		deps := newResourceSaveDeps(orchestratorService, metadataService)

		_, err := executeForTest(deps, "", "resource", "copy", "/admin/realms/test", "/admin/realms/test-copy")
		assertTypedCategory(t, err, faults.NotFoundError)
		if len(orchestratorService.getLocalCalls) != 1 || orchestratorService.getLocalCalls[0] != "/admin/realms/test" {
			t.Fatalf("expected one local read for source path, got %#v", orchestratorService.getLocalCalls)
		}
		if len(orchestratorService.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote fallback reads for source path, got %#v", orchestratorService.getRemoteCalls)
		}
		if len(orchestratorService.saveCalls) != 0 {
			t.Fatalf("expected no save calls on local miss, got %#v", orchestratorService.saveCalls)
		}
	})
}

func TestResourceCopyOverrideAttributes(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test", "enabled": false},
		},
	}
	deps := newResourceSaveDeps(orchestratorService, metadataService)

	if _, err := executeForTest(
		deps,
		"",
		"resource", "copy",
		"/admin/realms/test", "/admin/realms/test-copy",
		"--override-attributes", "realm=test-copy,enabled=true",
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orchestratorService.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %#v", orchestratorService.saveCalls)
	}
	if !reflect.DeepEqual(orchestratorService.saveCalls[0].value, map[string]any{"realm": "test-copy", "enabled": "true"}) {
		t.Fatalf("expected overridden payload, got %#v", orchestratorService.saveCalls[0].value)
	}
}

func TestResourceCopyRequiresOverwriteWhenTargetExists(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test"},
		},
	}
	deps := testDepsWith(orchestratorService, metadataService)
	repoService := deps.ResourceStore.(*testRepository)

	_, err := executeForTest(
		deps,
		"",
		"resource", "copy",
		"/admin/realms/test", "/admin/realms/test-copy",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected --overwrite guidance error, got %v", err)
	}
	if len(repoService.commitCalls) != 0 {
		t.Fatalf("expected no commit calls on overwrite guard failure, got %#v", repoService.commitCalls)
	}
}

func TestResourceCopyGitContextCommitsRepositoryChange(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test"},
		},
	}
	deps := testDepsWith(orchestratorService, metadataService)
	repoService := &copyCommitTestRepository{}
	deps.ResourceStore = repoService
	deps.RepositorySync = repoService

	if _, err := executeForTest(
		deps,
		"",
		"--context", "git",
		"resource", "copy",
		"/admin/realms/test",
		"/admin/realms/test-copy",
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repoService.commitCalls) != 1 {
		t.Fatalf("expected one commit call, got %#v", repoService.commitCalls)
	}
	if repoService.commitCalls[0] != "declarest: copy resource /admin/realms/test to /admin/realms/test-copy" {
		t.Fatalf("unexpected commit message: %q", repoService.commitCalls[0])
	}
	if repoService.pushCalls != 0 {
		t.Fatalf("expected copy auto-commit to avoid push, got %d push calls", repoService.pushCalls)
	}
}

func TestResourceCopyMessageAppendsToDefaultCommitMessage(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test"},
		},
	}
	deps := testDepsWith(orchestratorService, metadataService)
	repoService := &copyCommitTestRepository{}
	deps.ResourceStore = repoService
	deps.RepositorySync = repoService

	if _, err := executeForTest(
		deps,
		"",
		"--context", "git",
		"resource", "copy",
		"/admin/realms/test",
		"/admin/realms/test-copy",
		"--message", "ticket-123",
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repoService.commitCalls) != 1 {
		t.Fatalf("expected one commit call, got %#v", repoService.commitCalls)
	}
	if repoService.commitCalls[0] != "declarest: copy resource /admin/realms/test to /admin/realms/test-copy - ticket-123" {
		t.Fatalf("unexpected commit message: %q", repoService.commitCalls[0])
	}
}

func TestResourceCopyOverwriteAllowsReplacingExistingTargetAndCommits(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test"},
		},
	}
	deps := testDepsWith(orchestratorService, metadataService)
	repoService := deps.ResourceStore.(*testRepository)

	if _, err := executeForTest(
		deps,
		"",
		"--context", "git",
		"resource", "copy",
		"/admin/realms/test", "/admin/realms/test-copy",
		"--overwrite",
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repoService.commitCalls) != 1 {
		t.Fatalf("expected one commit call, got %#v", repoService.commitCalls)
	}
	if repoService.commitCalls[0] != "declarest: copy resource /admin/realms/test to /admin/realms/test-copy" {
		t.Fatalf("unexpected commit message: %q", repoService.commitCalls[0])
	}
}

func TestResourceCopyOverrideAliasStillAccepted(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestratorService := &testOrchestrator{
		metadataService: metadataService,
		getLocalValues: map[string]resource.Value{
			"/admin/realms/test": map[string]any{"realm": "test"},
		},
	}
	deps := testDepsWith(orchestratorService, metadataService)

	if _, err := executeForTest(
		deps,
		"",
		"--context", "git",
		"resource", "copy",
		"/admin/realms/test", "/admin/realms/test-copy",
		"--override",
	); err != nil {
		t.Fatalf("unexpected error with --override alias: %v", err)
	}
}

type copyCommitTestRepository struct {
	resourceSaveTestRepository
	commitCalls []string
	pushCalls   int
}

func (r *copyCommitTestRepository) Commit(_ context.Context, message string) (bool, error) {
	r.commitCalls = append(r.commitCalls, message)
	return true, nil
}

func (r *copyCommitTestRepository) Push(context.Context, repository.PushPolicy) error {
	r.pushCalls++
	return nil
}
