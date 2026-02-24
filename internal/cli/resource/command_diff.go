package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newDiffCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "diff [path]",
		Short: "Compare local and remote state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			targets, err := listLocalMutationTargets(command.Context(), orchestratorService, resolvedPath, false)
			if err != nil {
				return err
			}

			items := make([]resource.DiffEntry, 0)
			for _, target := range targets {
				targetItems, diffErr := orchestratorService.Diff(command.Context(), target.LogicalPath)
				if diffErr != nil {
					return diffErr
				}
				items = append(items, targetItems...)
			}

			sort.Slice(items, func(i int, j int) bool {
				if items[i].ResourcePath == items[j].ResourcePath {
					if items[i].Path == items[j].Path {
						return items[i].Operation < items[j].Operation
					}
					return items[i].Path < items[j].Path
				}
				return items[i].ResourcePath < items[j].ResourcePath
			})

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.DiffEntry) error {
				for _, item := range value {
					line, lineErr := renderDiffTextLine(resolvedPath, item)
					if lineErr != nil {
						return lineErr
					}
					if _, writeErr := fmt.Fprintln(w, line); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func renderDiffTextLine(basePath string, entry resource.DiffEntry) (string, error) {
	local, err := marshalDiffTextValue(entry.Local)
	if err != nil {
		return "", err
	}
	remote, err := marshalDiffTextValue(entry.Remote)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s [Local=%s] => [Remote=%s]",
		formatDiffTextPath(basePath, entry),
		local,
		remote,
	), nil
}

func marshalDiffTextValue(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func formatDiffTextPath(basePath string, entry resource.DiffEntry) string {
	diffPath := joinDiffEntryPath(entry)
	normalizedBasePath, err := resource.NormalizeLogicalPath(basePath)
	if err != nil {
		return diffPath
	}

	normalizedDiffPath, err := resource.NormalizeLogicalPath(diffPath)
	if err != nil {
		return diffPath
	}

	if normalizedDiffPath == normalizedBasePath {
		return "."
	}

	if normalizedBasePath == "/" {
		return dotPathFromPointer(normalizedDiffPath)
	}

	prefix := normalizedBasePath + "/"
	if !strings.HasPrefix(normalizedDiffPath, prefix) {
		return diffPath
	}

	return dotPathFromPointer(strings.TrimPrefix(normalizedDiffPath, normalizedBasePath))
}

func joinDiffEntryPath(entry resource.DiffEntry) string {
	if entry.ResourcePath == "" {
		return entry.Path
	}
	if entry.Path == "" {
		return entry.ResourcePath
	}
	if !strings.HasPrefix(entry.Path, "/") {
		return entry.Path
	}
	if entry.ResourcePath == "/" {
		return entry.Path
	}
	return entry.ResourcePath + entry.Path
}

func dotPathFromPointer(pointerPath string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(pointerPath), "/")
	if trimmed == "" {
		return "."
	}

	segments := strings.Split(trimmed, "/")
	for idx, segment := range segments {
		segments[idx] = unescapePointerToken(segment)
	}

	return "." + strings.Join(segments, ".")
}

func unescapePointerToken(value string) string {
	unescaped := strings.ReplaceAll(value, "~1", "/")
	return strings.ReplaceAll(unescaped, "~0", "~")
}
