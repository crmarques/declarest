// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		return faults.NewValidationError(
			fmt.Sprintf("resource %q already exists; rerun with --force to overwrite", logicalPath),
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
	if faults.IsCategory(err, faults.NotFoundError) {
		return false, nil
	}
	return false, err
}
