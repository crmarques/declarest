package reconciler

import (
	"errors"

	"github.com/crmarques/declarest/repository"
)

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
	if checker, ok := r.ResourceRepositoryManager.(repository.RemoteAccessChecker); ok {
		configured, err := checker.CheckRemoteAccess()
		if err != nil && errors.Is(err, repository.ErrRemoteRepositoryEmpty) {
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
	if checker, ok := r.ResourceRepositoryManager.(repository.LocalRepositoryStateChecker); ok {
		initialized, err := checker.IsLocalRepositoryInitialized()
		return true, initialized, err
	}
	return false, false, nil
}

func (r *DefaultReconciler) CheckRemoteSync() (bool, bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := r.ResourceRepositoryManager.(repository.RemoteSyncChecker); ok {
		return checker.CheckRemoteSync()
	}
	return false, false, nil
}
