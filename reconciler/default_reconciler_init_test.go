package reconciler

import (
	"errors"
	"testing"

	"github.com/crmarques/declarest/repository"
)

func TestDefaultReconcilerInitNil(t *testing.T) {
	var recon *DefaultReconciler
	if err := recon.Init(); err == nil {
		t.Fatal("expected error when reconciler is nil")
	}
}

func TestDefaultReconcilerInitPropagatesRepoInitError(t *testing.T) {
	repo := &stubRepoManager{initErr: errors.New("init failed")}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if err := recon.Init(); err == nil {
		t.Fatal("expected Init to return repository init error")
	}
	if !repo.initCalled {
		t.Fatal("expected repository Init to be called")
	}
}

func TestDefaultReconcilerInitKeepsRepoSyncError(t *testing.T) {
	syncErr := &repository.RepoSyncError{}
	repo := &syncRepoManager{syncErr: syncErr}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if err := recon.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if recon.RepoSyncError != syncErr {
		t.Fatalf("expected RepoSyncError to be preserved, got %v", recon.RepoSyncError)
	}
	if !repo.syncCalled {
		t.Fatal("expected repository sync to be called")
	}
}

func TestDefaultReconcilerInitPropagatesSyncError(t *testing.T) {
	syncErr := errors.New("sync failed")
	repo := &syncRepoManager{syncErr: syncErr}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if err := recon.Init(); err == nil {
		t.Fatal("expected Init to return sync error")
	}
	if recon.RepoSyncError != nil {
		t.Fatalf("expected RepoSyncError to remain nil, got %v", recon.RepoSyncError)
	}
	if !repo.syncCalled {
		t.Fatal("expected repository sync to be called")
	}
}

func TestDefaultReconcilerInitClearsRepoSyncErrorOnSuccess(t *testing.T) {
	repo := &syncRepoManager{}
	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
		RepoSyncError:             errors.New("stale"),
	}

	if err := recon.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if recon.RepoSyncError != nil {
		t.Fatalf("expected RepoSyncError to be cleared, got %v", recon.RepoSyncError)
	}
	if !repo.syncCalled {
		t.Fatal("expected repository sync to be called")
	}
}
