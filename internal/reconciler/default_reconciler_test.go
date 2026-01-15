package reconciler

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"declarest/internal/managedserver"
	"declarest/internal/repository"
	"declarest/internal/resource"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func newTestReconciler(t *testing.T) *DefaultReconciler {
	t.Helper()

	baseDir := filepath.Join("..", "..", "tests", "sample", "repo")
	repoManager := repository.NewGitResourceRepositoryManager(baseDir)
	if err := repoManager.Init(); err != nil {
		t.Fatalf("init repo manager: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repoManager,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(baseDir, recon)

	return recon
}

func newReconcilerWithServer(t *testing.T, server managedserver.ResourceServerManager) *DefaultReconciler {
	t.Helper()

	baseDir := filepath.Join("..", "..", "tests", "sample", "repo")
	repoManager := repository.NewGitResourceRepositoryManager(baseDir)
	if err := repoManager.Init(); err != nil {
		t.Fatalf("init repo manager: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repoManager,
		ResourceServerManager:     server,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(baseDir, recon)

	return recon
}

func newReconcilerWithServerAndRepo(t *testing.T, server managedserver.ResourceServerManager, repoDir string) *DefaultReconciler {
	t.Helper()

	repoManager := repository.NewGitResourceRepositoryManager(repoDir)
	if err := repoManager.Init(); err != nil {
		t.Fatalf("init repo manager: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repoManager,
		ResourceServerManager:     server,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(repoDir, recon)

	return recon
}

func commitGitFile(t *testing.T, repo *git.Repository, dir, name, content string) plumbing.Hash {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if _, err := wt.Add(name); err != nil {
		t.Fatalf("add file: %v", err)
	}

	sig := &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Now(),
	}

	hash, err := wt.Commit("commit "+name, &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return hash
}

func setupRemoteGitRepo(t *testing.T) (string, *git.Repository, string, plumbing.Hash) {
	t.Helper()

	remoteDir := t.TempDir()
	if _, err := git.PlainInit(remoteDir, true); err != nil {
		t.Fatalf("init remote: %v", err)
	}

	seedDir := t.TempDir()
	seedRepo, err := git.PlainInit(seedDir, false)
	if err != nil {
		t.Fatalf("init seed: %v", err)
	}

	seedHash := commitGitFile(t, seedRepo, seedDir, "seed.txt", "one")

	if _, err := seedRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push seed: %v", err)
	}

	return remoteDir, seedRepo, seedDir, seedHash
}

func TestDefaultReconcilerInitSyncsRemoteRepo(t *testing.T) {
	remoteDir, _, _, seedHash := setupRemoteGitRepo(t)

	localDir := t.TempDir()
	repoManager := repository.NewGitResourceRepositoryManager(localDir)
	repoManager.SetConfig(&repository.GitResourceRepositoryConfig{
		Remote: &repository.GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repoManager,
	}

	if err := recon.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	repo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Hash() != seedHash {
		t.Fatalf("expected local head %s, got %s", seedHash, head.Hash())
	}
}

func TestDefaultReconcilerInitSkipsRepoSyncWhenDisabled(t *testing.T) {
	remoteDir, _, _, _ := setupRemoteGitRepo(t)

	localDir := t.TempDir()
	repoManager := repository.NewGitResourceRepositoryManager(localDir)
	repoManager.SetConfig(&repository.GitResourceRepositoryConfig{
		Remote: &repository.GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repoManager,
		SkipRepositorySync:        true,
	}

	if err := recon.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(localDir, ".git")); err == nil {
		t.Fatalf("expected repository to remain unsynced")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat repo: %v", err)
	}
}

func TestGetRemoteResourcePathUsesMetadataIdentifiers(t *testing.T) {
	recon := newTestReconciler(t)

	path, err := recon.GetRemoteResourcePath("/xxx/xxx-01")
	if err != nil {
		t.Fatalf("GetRemoteResourcePath returned error: %v", err)
	}

	if path != "/xxx/blaXXX" {
		t.Fatalf("expected remote path /xxx/blaXXX, got %s", path)
	}
}

func TestGetRemoteResourcePathHandlesNestedAncestors(t *testing.T) {
	recon := newTestReconciler(t)

	path, err := recon.GetRemoteResourcePath("/xxx/xxx-01/yyy/yyy-01")
	if err != nil {
		t.Fatalf("GetRemoteResourcePath returned error: %v", err)
	}

	if path != "/xxx/blaXXX/yyy/bliYYY" {
		t.Fatalf("expected remote path /xxx/blaXXX/yyy/bliYYY, got %s", path)
	}
}

func TestGetRemoteResourceTriesLiteralThenMetadata(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"bla": "blaXXX",
		"ble": "xxx-01",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/xxx/blaXXX": res,
		},
		collections: map[string][]resource.Resource{
			"/xxx": {res},
		},
	}
	recon := newReconcilerWithServer(t, server)

	got, err := recon.GetRemoteResource("/xxx/xxx-01")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, res.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/xxx/xxx-01") {
		t.Fatalf("expected get:/xxx/xxx-01 in calls, got %#v", server.calls)
	}
	if !containsCall(server.calls, "get:/xxx/blaXXX") {
		t.Fatalf("expected get:/xxx/blaXXX in calls, got %#v", server.calls)
	}
}

func TestGetRemoteResourceStopsAfterLiteralSuccess(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"bla": "literalOnly",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/xxx/xxx-01": res,
		},
	}
	recon := newReconcilerWithServer(t, server)

	got, err := recon.GetRemoteResource("/xxx/xxx-01")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, res.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/xxx/xxx-01") {
		t.Fatalf("expected get:/xxx/xxx-01 in calls, got %#v", server.calls)
	}
}

func TestGetRemoteResourceUsesOperationPathOverride(t *testing.T) {
	res, err := resource.NewResource(map[string]any{
		"id":   "id-1",
		"name": "foo",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/items/foo/detail": res,
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  },
  "operationInfo": {
    "getResource": {
      "url": { "path": "./{{.alias}}/detail" }
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	itemDir := filepath.Join(repoDir, "items", "foo")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "resource.json"), []byte(`{"id":"id-1","name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write local resource: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	got, err := recon.GetRemoteResource("/items/foo")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, res.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/items/foo/detail") {
		t.Fatalf("expected get:/items/foo/detail in calls, got %#v", server.calls)
	}
}

func TestListRemoteResourcePathsUsesAliasAndIDFallback(t *testing.T) {
	res1 := mustResource(t, map[string]any{
		"id":   "1",
		"name": "alpha",
	})
	res2 := mustResource(t, map[string]any{
		"id": "2",
	})

	server := &fakeServer{
		collections: map[string][]resource.Resource{
			"/items": {res1, res2},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)
	entries, err := recon.ListRemoteResourceEntries("/items")
	if err != nil {
		t.Fatalf("ListRemoteResourceEntries: %v", err)
	}
	wantEntries := []RemoteResourceEntry{
		{Path: "/items/2", ID: "2", Alias: "2", AliasPath: "/items/2"},
		{Path: "/items/1", ID: "1", Alias: "alpha", AliasPath: "/items/alpha"},
	}
	if !reflect.DeepEqual(entries, wantEntries) {
		t.Fatalf("expected entries %#v, got %#v", wantEntries, entries)
	}

	paths, err := recon.ListRemoteResourcePaths("/items")
	if err != nil {
		t.Fatalf("ListRemoteResourcePaths: %v", err)
	}

	want := []string{"/items/2", "/items/1"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("expected paths %#v, got %#v", want, paths)
	}
}

func TestListRemoteResourceEntriesResolvesAncestorAlias(t *testing.T) {
	parent := mustResource(t, map[string]any{
		"id":   "realm-123",
		"name": "publico",
	})
	child := mustResource(t, map[string]any{
		"id":       "client-1",
		"clientId": "app",
	})

	server := &fakeServer{
		collections: map[string][]resource.Resource{
			"/realms":                   {parent},
			"/realms/realm-123/clients": {child},
		},
	}

	repoDir := t.TempDir()
	realmMeta := filepath.Join(repoDir, "realms", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(realmMeta), 0o755); err != nil {
		t.Fatalf("mkdir realm metadata dir: %v", err)
	}
	if err := os.WriteFile(realmMeta, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}`), 0o644); err != nil {
		t.Fatalf("write realm metadata: %v", err)
	}

	clientMeta := filepath.Join(repoDir, "realms", "_", "clients", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(clientMeta), 0o755); err != nil {
		t.Fatalf("mkdir client metadata dir: %v", err)
	}
	if err := os.WriteFile(clientMeta, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "clientId"
  }
}`), 0o644); err != nil {
		t.Fatalf("write client metadata: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)
	entries, err := recon.ListRemoteResourceEntries("/realms/publico/clients")
	if err != nil {
		t.Fatalf("ListRemoteResourceEntries: %v", err)
	}

	wantEntries := []RemoteResourceEntry{
		{
			Path:      "/realms/realm-123/clients/client-1",
			ID:        "client-1",
			Alias:     "app",
			AliasPath: "/realms/publico/clients/app",
		},
	}
	if !reflect.DeepEqual(entries, wantEntries) {
		t.Fatalf("expected entries %#v, got %#v", wantEntries, entries)
	}

	if !containsCall(server.calls, "list:/realms") {
		t.Fatalf("expected list:/realms in calls, got %#v", server.calls)
	}
	if !containsCall(server.calls, "list:/realms/realm-123/clients") {
		t.Fatalf("expected list:/realms/realm-123/clients in calls, got %#v", server.calls)
	}
}

func TestListRemoteResourcePathsFromLocalCollections(t *testing.T) {
	res1 := mustResource(t, map[string]any{
		"id": "a",
	})
	res2 := mustResource(t, map[string]any{
		"id": "b",
	})

	server := &fakeServer{
		collections: map[string][]resource.Resource{
			"/items": {res1},
			"/other": {res2},
		},
	}

	repoDir := t.TempDir()
	itemDir := filepath.Join(repoDir, "items", "a")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "resource.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatalf("write item resource: %v", err)
	}

	otherDir := filepath.Join(repoDir, "other", "b")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatalf("mkdir other dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "resource.json"), []byte(`{"id":"b"}`), 0o644); err != nil {
		t.Fatalf("write other resource: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)
	paths, err := recon.ListRemoteResourcePathsFromLocal()
	if err != nil {
		t.Fatalf("ListRemoteResourcePathsFromLocal: %v", err)
	}

	want := []string{"/items/a", "/other/b"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("expected paths %#v, got %#v", want, paths)
	}
}

func TestGetRemoteResourceFallsBackToAliasLookupOn404(t *testing.T) {
	resolved, err := resource.NewResource(map[string]any{
		"bla": "id-123",
		"ble": "testB",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/xxx/id-123": resolved,
		},
		collections: map[string][]resource.Resource{
			"/xxx": {resolved},
		},
	}

	recon := newReconcilerWithServer(t, server)

	provider := repository.NewDefaultResourceRecordProvider(filepath.Join("..", "..", "tests", "sample", "repo"), recon)
	recon.ResourceRecordProvider = provider

	got, err := recon.GetRemoteResource("/xxx/testB")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, resolved.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/xxx/testB") {
		t.Fatalf("expected get:/xxx/testB in calls, got %#v", server.calls)
	}
	if !containsCall(server.calls, "list:/xxx") {
		t.Fatalf("expected list:/xxx in calls, got %#v", server.calls)
	}
}

func TestGetRemoteResourceUsesAliasFromWildcardMetadata(t *testing.T) {
	resolved, err := resource.NewResource(map[string]any{
		"id":       "id-123",
		"clientId": "testB",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/admin/realms/publico/clients/id-123": resolved,
		},
		collections: map[string][]resource.Resource{
			"/admin/realms/publico/clients": {resolved},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "admin", "realms", "_", "clients", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"clientId"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	got, err := recon.GetRemoteResource("/admin/realms/publico/clients/testB")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, resolved.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/admin/realms/publico/clients/testB") {
		t.Fatalf("expected get:/admin/realms/publico/clients/testB in calls, got %#v", server.calls)
	}
	if !containsCall(server.calls, "list:/admin/realms/publico/clients") {
		t.Fatalf("expected list:/admin/realms/publico/clients in calls, got %#v", server.calls)
	}
}

func TestGetRemoteResourceAppliesListCollectionJQFilter(t *testing.T) {
	keep, err := resource.NewResource(map[string]any{
		"id":   "1",
		"name": "filtered",
		"kind": "keep",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	discard, err := resource.NewResource(map[string]any{
		"id":   "2",
		"name": "other",
		"kind": "skip",
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	server := &fakeServer{
		collections: map[string][]resource.Resource{
			"/items": {keep, discard},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  },
  "operationInfo": {
    "listCollection": {
      "url": { "path": "." },
      "jqFilter": "[.[] | select(.kind == \"keep\")]"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	got, err := recon.GetRemoteResource("/items/filtered")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, keep.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "list:/items") {
		t.Fatalf("expected list:/items in calls, got %#v", server.calls)
	}
}

func TestGetRemoteResourcePrefersResolvedPathWhenLocalExistsAndAliasesAreNotUnique(t *testing.T) {
	first := mustResource(t, map[string]any{"id": "id-a", "name": "foo"})
	second := mustResource(t, map[string]any{"id": "id-b", "name": "foo"})

	server := &fakeServer{
		resources: map[string]resource.Resource{
			"/items/id-a": first,
			"/items/id-b": second,
		},
		collections: map[string][]resource.Resource{
			"/items": {first, second},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"name"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	itemDir := filepath.Join(repoDir, "items", "foo")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "resource.json"), []byte(`{"id":"id-b","name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write local resource: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	got, err := recon.GetRemoteResource("/items/foo")
	if err != nil {
		t.Fatalf("GetRemoteResource returned error: %v", err)
	}

	if !reflect.DeepEqual(got.V, second.V) {
		t.Fatalf("unexpected resource returned: %+v", got.V)
	}

	if !containsCall(server.calls, "get:/items/id-b") {
		t.Fatalf("expected get:/items/id-b in calls, got %#v", server.calls)
	}
}

func TestDeleteRemoteResourcePropagatesErrors(t *testing.T) {
	server := &fakeServer{
		deleteErr: map[string]error{
			"/items/id-b": &managedserver.HTTPError{Method: http.MethodDelete, URL: "/items/id-b", StatusCode: http.StatusBadGateway},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"name"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	itemDir := filepath.Join(repoDir, "items", "foo")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "resource.json"), []byte(`{"id":"id-b","name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write local resource: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	if err := recon.DeleteRemoteResource("/items/foo"); err == nil {
		t.Fatalf("expected delete to return an error, got nil")
	}
}

func TestDeleteRemoteResourceFallsBackToAliasOnNotFound(t *testing.T) {
	first := mustResource(t, map[string]any{"id": "id-a", "name": "foo"})

	server := &fakeServer{
		collections: map[string][]resource.Resource{
			"/items": {first},
		},
		deleteErr: map[string]error{
			"/items/id-b": &managedserver.HTTPError{Method: http.MethodDelete, URL: "/items/id-b", StatusCode: http.StatusNotFound},
		},
	}

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"name"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	itemDir := filepath.Join(repoDir, "items", "foo")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "resource.json"), []byte(`{"id":"id-b","name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write local resource: %v", err)
	}

	recon := newReconcilerWithServerAndRepo(t, server, repoDir)

	if err := recon.DeleteRemoteResource("/items/foo"); err != nil {
		t.Fatalf("expected delete to succeed via alias fallback, got %v", err)
	}
}

func TestSaveLocalCollectionItems(t *testing.T) {
	tmp := t.TempDir()
	repo := repository.NewGitResourceRepositoryManager(tmp)
	if err := repo.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(tmp, recon)

	items := []resource.Resource{
		mustResource(t, map[string]any{"id": "a1", "name": "first"}),
		mustResource(t, map[string]any{"id": "a2", "name": "second"}),
	}

	if err := recon.SaveLocalCollectionItems("/items/", items); err != nil {
		t.Fatalf("SaveLocalCollectionItems returned error: %v", err)
	}

	for _, id := range []string{"a1", "a2"} {
		_, err := repo.GetResource("/items/" + id)
		if err != nil {
			t.Fatalf("expected resource /items/%s to be saved: %v", id, err)
		}
	}
}

func TestSaveLocalCollectionItemsUsesUserPath(t *testing.T) {
	tmp := t.TempDir()
	repo := repository.NewGitResourceRepositoryManager(tmp)
	if err := repo.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	metaPath := filepath.Join(tmp, "local", "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name",
    "collectionPath": "/remote/items"
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(tmp, recon)

	items := []resource.Resource{
		mustResource(t, map[string]any{"id": "1", "name": "foo"}),
		mustResource(t, map[string]any{"id": "2", "name": "bar"}),
	}

	if err := recon.SaveLocalCollectionItems("/local/items/", items); err != nil {
		t.Fatalf("SaveLocalCollectionItems returned error: %v", err)
	}

	for _, name := range []string{"foo", "bar"} {
		if _, err := repo.GetResource("/local/items/" + name); err != nil {
			t.Fatalf("expected resource saved at user path for %s: %v", name, err)
		}
	}
}

func TestResolveRemoteResourcePathUsesCollectionOverride(t *testing.T) {
	tmp := t.TempDir()
	repo := repository.NewGitResourceRepositoryManager(tmp)
	if err := repo.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	realmPath := filepath.Join(tmp, "admin", "realms", "publico")
	if err := os.MkdirAll(realmPath, 0o755); err != nil {
		t.Fatalf("mkdir realm dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realmPath, "resource.json"), []byte(`{"realm":"publico"}`), 0o644); err != nil {
		t.Fatalf("write realm resource: %v", err)
	}

	mapperDir := filepath.Join(tmp, "admin", "realms", "publico", "components", "ldap-test", "mappers", "email")
	if err := os.MkdirAll(mapperDir, 0o755); err != nil {
		t.Fatalf("mkdir mapper dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mapperDir, "resource.json"), []byte(`{"id":"mapper-id","name":"email"}`), 0o644); err != nil {
		t.Fatalf("write mapper resource: %v", err)
	}

	metaPath := filepath.Join(tmp, "admin", "realms", "_", "components", "_", "mappers", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{
  "resourceInfo": {
    "collectionPath": "/admin/realms/{{.realm}}/components",
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(tmp, recon)

	path, err := recon.GetRemoteResourcePath("/admin/realms/publico/components/ldap-test/mappers/email")
	if err != nil {
		t.Fatalf("GetRemoteResourcePath returned error: %v", err)
	}

	if path != "/admin/realms/publico/components/mapper-id" {
		t.Fatalf("expected remote path override, got %s", path)
	}
}

func TestUpdateRemoteResourceDoesNotCreateOnMissing(t *testing.T) {
	tmp := t.TempDir()
	repo := repository.NewGitResourceRepositoryManager(tmp)
	if err := repo.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	server := &fakeServer{
		resources:   map[string]resource.Resource{},
		collections: map[string][]resource.Resource{},
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
		ResourceServerManager:     server,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(tmp, recon)

	path := "/items/item-1"
	if err := repo.ApplyResource(path, mustResource(t, map[string]any{"id": "item-1"})); err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	err := recon.UpdateRemoteResource(path, mustResource(t, map[string]any{"id": "item-1"}))
	if err == nil {
		t.Fatalf("expected update to fail when remote missing")
	}
	if !managedserver.IsNotFoundError(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
	for _, call := range server.calls {
		if strings.HasPrefix(call, "create:") {
			t.Fatalf("unexpected create call: %s", call)
		}
	}
}

func mustResource(t *testing.T, v map[string]any) resource.Resource {
	t.Helper()
	res, err := resource.NewResource(v)
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}
	return res
}

func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

type fakeServer struct {
	resources   map[string]resource.Resource
	collections map[string][]resource.Resource
	calls       []string
	deleteErr   map[string]error
}

func (f *fakeServer) Init() error  { return nil }
func (f *fakeServer) Close() error { return nil }

func (f *fakeServer) GetResource(spec managedserver.RequestSpec) (resource.Resource, error) {
	path := spec.HTTP.Path
	f.calls = append(f.calls, "get:"+path)
	if res, ok := f.resources[path]; ok {
		return res, nil
	}
	return resource.Resource{}, &managedserver.HTTPError{Method: spec.HTTP.Method, URL: spec.HTTP.Path, StatusCode: http.StatusNotFound}
}

func (f *fakeServer) GetResourceCollection(spec managedserver.RequestSpec) ([]resource.Resource, error) {
	f.calls = append(f.calls, "list:"+spec.HTTP.Path)
	if res, ok := f.collections[spec.HTTP.Path]; ok {
		return res, nil
	}
	return []resource.Resource{}, nil
}

func (f *fakeServer) CreateResource(_ resource.Resource, spec managedserver.RequestSpec) error {
	f.calls = append(f.calls, "create:"+spec.HTTP.Path)
	return nil
}

func (f *fakeServer) UpdateResource(_ resource.Resource, spec managedserver.RequestSpec) error {
	f.calls = append(f.calls, "update:"+spec.HTTP.Path)
	return nil
}

func (f *fakeServer) DeleteResource(spec managedserver.RequestSpec) error {
	path := spec.HTTP.Path
	f.calls = append(f.calls, "delete:"+path)
	if f.deleteErr != nil {
		if err, ok := f.deleteErr[path]; ok {
			return err
		}
	}
	delete(f.resources, path)
	return nil
}

func (f *fakeServer) ResourceExists(spec managedserver.RequestSpec) (bool, error) {
	path := spec.HTTP.Path
	f.calls = append(f.calls, "exists:"+path)
	_, ok := f.resources[path]
	return ok, nil
}
