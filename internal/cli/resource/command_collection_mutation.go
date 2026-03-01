package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/shared"
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
		return shared.WriteOutput(command, outputFormat, items[0], func(w io.Writer, value resource.Resource) error {
			_, writeErr := fmt.Fprintln(w, value.LogicalPath)
			return writeErr
		})
	}

	return shared.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.Resource) error {
		for _, item := range value {
			if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}
