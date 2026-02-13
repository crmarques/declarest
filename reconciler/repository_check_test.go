package reconciler

import (
	"errors"
	"testing"

	"github.com/crmarques/declarest/repository"
)

func TestCheckLocalRepositoryAccessMissingManager(t *testing.T) {
	var recon *DefaultReconciler
	if err := recon.CheckLocalRepositoryAccess(); err == nil {
		t.Fatal("expected error when repository manager is missing")
	}
}

func TestCheckLocalRepositoryAccessDelegatesInit(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if err := recon.CheckLocalRepositoryAccess(); err != nil {
		t.Fatalf("CheckLocalRepositoryAccess: %v", err)
	}
	if !repo.initCalled {
		t.Fatal("expected repository Init to be called")
	}
}

func TestCheckLocalRepositoryAccessPropagatesInitError(t *testing.T) {
	repo := &stubRepoManager{initErr: errors.New("init failed")}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if err := recon.CheckLocalRepositoryAccess(); err == nil {
		t.Fatal("expected init error to propagate")
	}
	if !repo.initCalled {
		t.Fatal("expected repository Init to be called")
	}
}

func TestCheckRemoteAccessMissingManager(t *testing.T) {
	var recon *DefaultReconciler
	if _, _, err := recon.CheckRemoteAccess(); err == nil {
		t.Fatal("expected error when repository manager is missing")
	}
}

func TestCheckRemoteAccessUnsupported(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	configured, empty, err := recon.CheckRemoteAccess()
	if err != nil {
		t.Fatalf("CheckRemoteAccess: %v", err)
	}
	if configured || empty {
		t.Fatalf("expected false, false for unsupported check, got %v, %v", configured, empty)
	}
}

func TestCheckRemoteAccessTreatsEmptyRemoteAsNonError(t *testing.T) {
	repo := &remoteCheckerRepo{configured: true, err: repository.ErrRemoteRepositoryEmpty}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	configured, empty, err := recon.CheckRemoteAccess()
	if err != nil {
		t.Fatalf("CheckRemoteAccess: %v", err)
	}
	if !configured || !empty {
		t.Fatalf("expected configured and empty to be true, got %v, %v", configured, empty)
	}
}

func TestCheckRemoteAccessPropagatesError(t *testing.T) {
	repo := &remoteCheckerRepo{configured: true, err: errors.New("remote down")}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	if _, _, err := recon.CheckRemoteAccess(); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCheckLocalRepositoryInitializedMissingManager(t *testing.T) {
	var recon *DefaultReconciler
	if _, _, err := recon.CheckLocalRepositoryInitialized(); err == nil {
		t.Fatal("expected error when repository manager is missing")
	}
}

func TestCheckLocalRepositoryInitializedUnsupported(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	supported, initialized, err := recon.CheckLocalRepositoryInitialized()
	if err != nil {
		t.Fatalf("CheckLocalRepositoryInitialized: %v", err)
	}
	if supported || initialized {
		t.Fatalf("expected false, false for unsupported check, got %v, %v", supported, initialized)
	}
}

func TestCheckLocalRepositoryInitializedSupported(t *testing.T) {
	repo := &localInitCheckerRepo{initialized: true}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	supported, initialized, err := recon.CheckLocalRepositoryInitialized()
	if err != nil {
		t.Fatalf("CheckLocalRepositoryInitialized: %v", err)
	}
	if !supported || !initialized {
		t.Fatalf("expected true, true, got %v, %v", supported, initialized)
	}
}

func TestCheckRemoteSyncMissingManager(t *testing.T) {
	var recon *DefaultReconciler
	if _, _, err := recon.CheckRemoteSync(); err == nil {
		t.Fatal("expected error when repository manager is missing")
	}
}

func TestCheckRemoteSyncUnsupported(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	supported, inSync, err := recon.CheckRemoteSync()
	if err != nil {
		t.Fatalf("CheckRemoteSync: %v", err)
	}
	if supported || inSync {
		t.Fatalf("expected false, false for unsupported check, got %v, %v", supported, inSync)
	}
}

func TestCheckRemoteSyncSupported(t *testing.T) {
	repo := &remoteSyncCheckerRepo{supported: true, inSync: true}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	supported, inSync, err := recon.CheckRemoteSync()
	if err != nil {
		t.Fatalf("CheckRemoteSync: %v", err)
	}
	if !supported || !inSync {
		t.Fatalf("expected true, true, got %v, %v", supported, inSync)
	}
}
