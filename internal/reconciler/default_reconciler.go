package reconciler

import (
	"errors"
	"fmt"
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/repository"
	"declarest/internal/resource"
	"declarest/internal/secrets"
)

type DefaultReconciler struct {
	ResourceServerManager     managedserver.ResourceServerManager
	ResourceRepositoryManager repository.ResourceRepositoryManager
	ResourceRecordProvider    repository.ResourceRecordProvider
	SecretsManager            secrets.SecretsManager
	SkipRepositorySync        bool
}

func (r *DefaultReconciler) Init() error {
	if r == nil {
		return errors.New("reconciler is nil")
	}

	if r.ResourceRepositoryManager != nil {
		if err := r.ResourceRepositoryManager.Init(); err != nil {
			return err
		}
		if !r.SkipRepositorySync {
			if syncer, ok := r.ResourceRepositoryManager.(interface {
				SyncLocalFromRemoteIfConfigured() error
			}); ok {
				if err := syncer.SyncLocalFromRemoteIfConfigured(); err != nil {
					return err
				}
			}
		}
	}

	if r.ResourceServerManager != nil {
		if err := r.ResourceServerManager.Init(); err != nil {
			return err
		}
	}

	if r.SecretsManager != nil {
		if err := r.SecretsManager.Init(); err != nil {
			return err
		}
	}

	return nil
}

func (r *DefaultReconciler) validateLogicalPath(path string) error {
	if err := resource.ValidateLogicalPath(path); err != nil {
		return fmt.Errorf("invalid logical path %q: %w", path, err)
	}
	return nil
}

func (r *DefaultReconciler) validateMetadataPath(path string) error {
	if err := resource.ValidateMetadataPath(path); err != nil {
		return fmt.Errorf("invalid metadata path %q: %w", path, err)
	}
	return nil
}

func (r *DefaultReconciler) recordFor(path string) (resource.ResourceRecord, error) {
	if r.ResourceRecordProvider == nil {
		return resource.ResourceRecord{}, errors.New("resource record provider is not configured")
	}

	record, err := r.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return resource.ResourceRecord{}, err
	}

	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}

	return record, nil
}
