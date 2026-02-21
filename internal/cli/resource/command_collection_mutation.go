package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func decodeOptionalResourceInput(command *cobra.Command, input common.InputFlags) (resource.Value, bool, error) {
	value, err := common.DecodeInput[resource.Value](command, input)
	if err == nil {
		return value, true, nil
	}
	if isMissingInputError(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func listLocalMutationTargets(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	items, err := orchestratorService.ListLocal(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && !recursive && logicalPathDepth(logicalPath) > 1 {
		localValue, getErr := orchestratorService.GetLocal(ctx, logicalPath)
		if getErr == nil {
			items = []resource.Resource{{
				LogicalPath: logicalPath,
				Payload:     localValue,
			}}
		} else if !isTypedErrorCategory(getErr, faults.NotFoundError) {
			return nil, getErr
		}
	}
	if len(items) == 0 {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("no local resources found under %q", logicalPath),
			nil,
		)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func listLocalMutationTargetsOrFallbackPath(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	items, err := listLocalMutationTargets(ctx, orchestratorService, logicalPath, recursive)
	if err == nil {
		return items, nil
	}
	if isTypedErrorCategory(err, faults.NotFoundError) {
		return []resource.Resource{{LogicalPath: logicalPath}}, nil
	}
	return nil, err
}

func executeMutationForTargets(
	ctx context.Context,
	targets []resource.Resource,
	runMutation func(context.Context, string) (resource.Resource, error),
) ([]resource.Resource, error) {
	results := make([]resource.Resource, 0, len(targets))
	for _, target := range targets {
		item, err := runMutation(ctx, target.LogicalPath)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	sort.Slice(results, func(i int, j int) bool {
		return results[i].LogicalPath < results[j].LogicalPath
	})
	return results, nil
}

func writeCollectionMutationOutput(
	command *cobra.Command,
	outputFormat string,
	resolvedPath string,
	items []resource.Resource,
) error {
	normalizedRequestedPath, normalizeErr := resource.NormalizeLogicalPath(resolvedPath)
	if normalizeErr == nil && len(items) == 1 && items[0].LogicalPath == normalizedRequestedPath {
		return common.WriteOutput(command, outputFormat, items[0], func(w io.Writer, value resource.Resource) error {
			_, writeErr := fmt.Fprintln(w, value.LogicalPath)
			return writeErr
		})
	}

	return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.Resource) error {
		for _, item := range value {
			if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}

func isMissingInputError(err error) bool {
	if err == nil {
		return false
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	return typedErr.Category == faults.ValidationError && typedErr.Message == common.MissingInputMessage
}

func logicalPathDepth(logicalPath string) int {
	trimmed := strings.Trim(strings.TrimSpace(logicalPath), "/")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}
