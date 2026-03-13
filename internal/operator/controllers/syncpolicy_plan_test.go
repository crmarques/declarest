package controllers

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildIncrementalPlanFromRepositoryDiff(t *testing.T) {
	t.Parallel()

	repo, repoDir := initTestGitRepo(t)

	writeFile(t, filepath.Join(repoDir, "customers", "acme", "resource.json"), `{"id":"acme","name":"Acme"}`)
	writeFile(t, filepath.Join(repoDir, "customers", "bravo", "resource.json"), `{"id":"bravo","name":"Bravo"}`)
	writeFile(t, filepath.Join(repoDir, "customers", "_", "metadata.json"), `{"resource":{"id":"{{/id}}"}}`)
	rev1 := commitAll(t, repo, "initial")

	writeFile(t, filepath.Join(repoDir, "customers", "acme", "resource.json"), `{"id":"acme","name":"Acme Updated"}`)
	rev2 := commitAll(t, repo, "update acme")

	plan, err := buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, rev1, rev2, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	if plan.requiresFull {
		t.Fatal("expected incremental plan without full-sync fallback")
	}
	targets := normalizeSyncApplyTargets(plan.applyTargets)
	expectedTargets := []syncApplyTarget{{Path: "/customers/acme", Recursive: false}}
	if !reflect.DeepEqual(targets, expectedTargets) {
		t.Fatalf("unexpected apply targets: got %#v want %#v", targets, expectedTargets)
	}

	writeFile(t, filepath.Join(repoDir, "customers", "_", "metadata.json"), `{"resource":{"id":"{{/id}}","alias":"{{/name}}"}}`)
	rev3 := commitAll(t, repo, "update customers metadata")

	plan, err = buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, rev2, rev3, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	targets = normalizeSyncApplyTargets(plan.applyTargets)
	expectedTargets = []syncApplyTarget{{Path: "/customers", Recursive: true}}
	if !reflect.DeepEqual(targets, expectedTargets) {
		t.Fatalf("unexpected metadata apply targets: got %#v want %#v", targets, expectedTargets)
	}

	writeFile(t, filepath.Join(repoDir, "customers", "_", "defaults.yaml"), "spec:\n  enabled: true\n")
	revDefaults := commitAll(t, repo, "add acme defaults")

	plan, err = buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, rev3, revDefaults, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	targets = normalizeSyncApplyTargets(plan.applyTargets)
	expectedTargets = []syncApplyTarget{{Path: "/customers", Recursive: true}}
	if !reflect.DeepEqual(targets, expectedTargets) {
		t.Fatalf("unexpected defaults apply targets: got %#v want %#v", targets, expectedTargets)
	}

	removeFile(t, filepath.Join(repoDir, "customers", "_", "defaults.yaml"))
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("repo.Worktree() error = %v", err)
	}
	if _, err := wt.Remove("customers/_/defaults.yaml"); err != nil {
		t.Fatalf("worktree remove defaults error = %v", err)
	}
	revDefaultsRemoved := commitAll(t, repo, "remove acme defaults")

	plan, err = buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, revDefaults, revDefaultsRemoved, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	targets = normalizeSyncApplyTargets(plan.applyTargets)
	expectedTargets = []syncApplyTarget{{Path: "/customers", Recursive: true}}
	if !reflect.DeepEqual(targets, expectedTargets) {
		t.Fatalf("unexpected defaults removal apply targets: got %#v want %#v", targets, expectedTargets)
	}
	if len(plan.pruneTargets) != 0 {
		t.Fatalf("expected no prune targets for defaults removal, got %#v", plan.pruneTargets)
	}

	writeFile(t, filepath.Join(repoDir, "customers", "acme", "defaults.yaml"), "spec:\n  enabled: true\n")
	revLegacyDefaults := commitAll(t, repo, "add legacy defaults")

	plan, err = buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, revDefaultsRemoved, revLegacyDefaults, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	if !plan.requiresFull {
		t.Fatal("expected legacy per-resource defaults diff to force full-sync fallback")
	}

	removeFile(t, filepath.Join(repoDir, "customers", "bravo", "resource.json"))
	wt, err = repo.Worktree()
	if err != nil {
		t.Fatalf("repo.Worktree() error = %v", err)
	}
	if _, err := wt.Remove("customers/bravo/resource.json"); err != nil {
		t.Fatalf("worktree remove error = %v", err)
	}
	rev4 := commitAll(t, repo, "remove bravo")

	plan, err = buildIncrementalPlanFromRepositoryDiff(context.Background(), repoDir, revLegacyDefaults, rev4, "/customers")
	if err != nil {
		t.Fatalf("buildIncrementalPlanFromRepositoryDiff() error = %v", err)
	}
	if !reflect.DeepEqual(stringSet(plan.pruneTargets), []string{"/customers/bravo"}) {
		t.Fatalf("unexpected prune targets: got %#v", plan.pruneTargets)
	}
}

func TestBuildSyncExecutionPlanFallsBackToFullWhenSecretHashChanges(t *testing.T) {
	t.Parallel()

	policy := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 4,
		},
		Spec: declarestv1alpha1.SyncPolicySpec{
			Source: declarestv1alpha1.SyncPolicySource{
				Path:      "/customers",
				Recursive: boolPtr(true),
			},
		},
		Status: declarestv1alpha1.SyncPolicyStatus{
			ObservedGeneration:      4,
			LastAppliedRepoRevision: "abc123",
		},
	}

	plan, err := buildSyncExecutionPlan(context.Background(), policy, "/tmp/does-not-matter", "def456", true, false)
	if err != nil {
		t.Fatalf("buildSyncExecutionPlan() error = %v", err)
	}
	if plan.Mode != syncModeFull {
		t.Fatalf("expected full mode, got %q", plan.Mode)
	}
	if len(plan.ApplyTargets) != 1 || plan.ApplyTargets[0].Path != "/customers" || !plan.ApplyTargets[0].Recursive {
		t.Fatalf("unexpected full plan targets: %#v", plan.ApplyTargets)
	}
}

func initTestGitRepo(t *testing.T) (*gogit.Repository, string) {
	t.Helper()

	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit() error = %v", err)
	}
	return repo, dir
}

func commitAll(t *testing.T, repo *gogit.Repository, message string) string {
	t.Helper()

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("repo.Worktree() error = %v", err)
	}
	if err := wt.AddGlob("."); err != nil {
		t.Fatalf("worktree add error = %v", err)
	}
	hash, err := wt.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "declarest-test",
			Email: "declarest@test.local",
			When:  time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("worktree commit error = %v", err)
	}
	return hash.String()
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func removeFile(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
