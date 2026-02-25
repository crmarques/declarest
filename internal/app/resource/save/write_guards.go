package save

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
)

func ensureSaveTargetAllowed(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	logicalPath string,
	force bool,
) error {
	if force {
		return nil
	}

	exists, err := resourceExists(ctx, repositoryService, logicalPath)
	if err != nil {
		return err
	}
	if exists {
		return validationError(
			fmt.Sprintf("resource %q already exists; rerun with --overwrite to overwrite", logicalPath),
			nil,
		)
	}
	return nil
}

func ensureSaveEntriesWritable(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	entries []saveEntry,
	force bool,
) error {
	if force {
		return nil
	}
	for _, entry := range entries {
		if err := ensureSaveTargetAllowed(ctx, repositoryService, entry.LogicalPath, false); err != nil {
			return err
		}
	}
	return nil
}

func resourceExists(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	logicalPath string,
) (bool, error) {
	_, err := repositoryService.Get(ctx, logicalPath)
	if err == nil {
		return true, nil
	}
	if isTypedErrorCategory(err, faults.NotFoundError) {
		return false, nil
	}
	return false, err
}
