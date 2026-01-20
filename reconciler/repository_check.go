package reconciler

import (
	"errors"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
)

type remoteAccessChecker interface {
	CheckRemoteAccess() (bool, error)
}

type remoteSyncChecker interface {
	CheckRemoteSync() (bool, bool, error)
}

type localRepoChecker interface {
	IsLocalRepositoryInitialized() (bool, error)
}

func (r *DefaultReconciler) CheckLocalRepositoryAccess() error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	return r.ResourceRepositoryManager.Init()
}

func (r *DefaultReconciler) CheckRemoteAccess() (bool, bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := r.ResourceRepositoryManager.(remoteAccessChecker); ok {
		configured, err := checker.CheckRemoteAccess()
		if err != nil && errors.Is(err, gittransport.ErrEmptyRemoteRepository) {
			return configured, true, nil
		}
		return configured, false, err
	}
	return false, false, nil
}

func (r *DefaultReconciler) CheckLocalRepositoryInitialized() (bool, bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := r.ResourceRepositoryManager.(localRepoChecker); ok {
		initialized, err := checker.IsLocalRepositoryInitialized()
		return true, initialized, err
	}
	return false, false, nil
}

func (r *DefaultReconciler) CheckRemoteSync() (bool, bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := r.ResourceRepositoryManager.(remoteSyncChecker); ok {
		return checker.CheckRemoteSync()
	}
	return false, false, nil
}
