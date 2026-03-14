package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	resourcediffapp "github.com/crmarques/declarest/internal/app/resource/diff"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newDiffCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var recursive bool
	var listOnly bool
	var colorFlag string

	command := &cobra.Command{
		Use:   "diff [path]",
		Short: "Compare local and remote state",
		Args:  cobra.MaximumNArgs(1),
		Example: "" +
			"  declarest resource diff /customers/acme\n" +
			"  declarest resource diff /customers --recursive\n" +
			"  declarest resource diff /customers --recursive --list\n" +
			"  declarest resource diff /customers/acme --color always\n",
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat := cliutil.ResolveCommandOutputFormat(command, globalFlags)
			colorMode, err := resolveDiffColorMode(colorFlag, command.Flags().Lookup("color").Changed, globalFlags)
			if err != nil {
				return err
			}

			orchestratorService, err := cliutil.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			targets, err := mutateapp.ListLocalTargets(command.Context(), orchestratorService, resolvedPath, recursive)
			if err != nil {
				return err
			}

			documents, items, err := collectDiffDocuments(command.Context(), orchestratorService, targets)
			if err != nil {
				return err
			}

			if listOnly {
				paths := collectChangedDiffPaths(documents)
				return cliutil.WriteOutput(command, outputFormat, paths, func(w io.Writer, value []string) error {
					return renderDiffPathList(w, value)
				})
			}

			if outputFormat == cliutil.OutputJSON || outputFormat == cliutil.OutputYAML {
				sortDiffEntries(items)
				return cliutil.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.DiffEntry) error {
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
			}

			report, err := buildDiffReport(documents)
			if err != nil {
				return err
			}
			return renderDiffReportText(command.OutOrStdout(), report, diffRenderOptions{
				RequestedPath: resolvedPath,
				ColorMode:     colorMode,
			})
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	command.Flags().BoolVar(&listOnly, "list", false, "list only resource paths with differences")
	command.Flags().StringVar(&colorFlag, "color", string(diffColorAuto), "diff color: auto|always|never")
	cliutil.RegisterFlagValueCompletions(command, "color", []string{
		string(diffColorAuto),
		string(diffColorAlways),
		string(diffColorNever),
	})
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	return command
}

type diffDocumentReader interface {
	DiffDocument(context.Context, string) (resourcediffapp.Document, error)
}

func collectDiffDocuments(
	ctx context.Context,
	orchestratorService interface{},
	targets []resource.Resource,
) ([]diffDocument, []resource.DiffEntry, error) {
	reader, ok := orchestratorService.(diffDocumentReader)
	if !ok {
		return nil, nil, cliutil.ValidationError("configured orchestrator does not support resource diff rendering", nil)
	}

	documents := make([]diffDocument, 0, len(targets))
	items := make([]resource.DiffEntry, 0)
	for _, target := range targets {
		document, err := reader.DiffDocument(ctx, target.LogicalPath)
		if err != nil {
			return nil, nil, err
		}

		documents = append(documents, diffDocument{
			ResourcePath: document.ResourcePath,
			Local:        document.Local,
			Remote:       document.Remote,
			Entries:      append([]resource.DiffEntry(nil), document.Entries...),
		})
		items = append(items, document.Entries...)
	}

	sort.Slice(documents, func(i int, j int) bool {
		return documents[i].ResourcePath < documents[j].ResourcePath
	})
	return documents, items, nil
}

func resolveDiffColorMode(rawValue string, explicit bool, globalFlags *cliutil.GlobalFlags) (diffColorMode, error) {
	if explicit {
		switch diffColorMode(strings.TrimSpace(rawValue)) {
		case diffColorAlways, diffColorNever:
			return diffColorMode(strings.TrimSpace(rawValue)), nil
		case diffColorAuto:
			return diffColorAuto, nil
		default:
			return "", cliutil.ValidationError("flag --color must be one of: auto, always, never", nil)
		}
	}

	if globalFlags != nil && globalFlags.NoColor {
		return diffColorNever, nil
	}
	return diffColorAuto, nil
}

func sortDiffEntries(items []resource.DiffEntry) {
	sort.Slice(items, func(i int, j int) bool {
		if items[i].ResourcePath == items[j].ResourcePath {
			if items[i].Path == items[j].Path {
				return items[i].Operation < items[j].Operation
			}
			return items[i].Path < items[j].Path
		}
		return items[i].ResourcePath < items[j].ResourcePath
	})
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
