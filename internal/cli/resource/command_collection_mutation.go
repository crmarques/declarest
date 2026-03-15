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

package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func writeCollectionMutationOutput(
	command *cobra.Command,
	outputFormat string,
	resolvedPath string,
	items []resource.Resource,
) error {
	normalizedRequestedPath, normalizeErr := resource.NormalizeLogicalPath(resolvedPath)
	if normalizeErr == nil && len(items) == 1 && items[0].LogicalPath == normalizedRequestedPath {
		return cliutil.WriteOutput(command, outputFormat, items[0], func(w io.Writer, value resource.Resource) error {
			_, writeErr := fmt.Fprintln(w, value.LogicalPath)
			return writeErr
		})
	}

	return cliutil.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.Resource) error {
		for _, item := range value {
			if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}
