package cmd

import (
	"errors"

	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"
)

func secretPathsFor(recon reconciler.AppReconciler, path string) ([]string, error) {
	if recon == nil {
		return nil, nil
	}
	return recon.SecretPathsFor(path)
}

func saveLocalResourceWithSecrets(recon reconciler.AppReconciler, path string, res resource.Resource, storeSecrets bool) error {
	if recon == nil {
		return errors.New("reconciler is not configured")
	}
	return recon.SaveLocalResourceWithSecrets(path, res, storeSecrets)
}

func saveLocalCollectionItemsWithSecrets(recon reconciler.AppReconciler, path string, items []resource.Resource, storeSecrets bool) error {
	if recon == nil {
		return errors.New("reconciler is not configured")
	}
	return recon.SaveLocalCollectionItemsWithSecrets(path, items, storeSecrets)
}
