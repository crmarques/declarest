package reconciler

import (
	"errors"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
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
	if checker, ok := r.ResourceRepositoryManager.(interface {
		CheckRemoteAccess() (bool, error)
	}); ok {
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
	if checker, ok := r.ResourceRepositoryManager.(interface {
		IsLocalRepositoryInitialized() (bool, error)
	}); ok {
		initialized, err := checker.IsLocalRepositoryInitialized()
		return true, initialized, err
	}
	return false, false, nil
}

func (r *DefaultReconciler) CheckRemoteSync() (bool, bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := r.ResourceRepositoryManager.(interface {
		CheckRemoteSync() (bool, bool, error)
	}); ok {
		return checker.CheckRemoteSync()
	}
	return false, false, nil
}
