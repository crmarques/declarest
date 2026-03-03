package resource

import (
	"bytes"
	"fmt"

	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

func newEditCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var editor string

	command := &cobra.Command{
		Use:   "edit [path]",
		Short: "Edit a local repository resource in an editor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			repositoryService, err := cliutil.RequireResourceStore(deps)
			if err != nil {
				return err
			}
			cfg, err := resolveActiveResourceContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if err := ensureCleanGitWorktreeForResourceEdit(command.Context(), deps, cfg); err != nil {
				return err
			}

			currentValue, err := repositoryService.Get(command.Context(), resolvedPath)
			if err != nil {
				return err
			}
			encoded, err := encodeResourcePayloadForEdit(cfg, currentValue)
			if err != nil {
				return err
			}

			editedBytes, err := cliutil.EditTempFile(
				command,
				cliutil.ResolveEditorCommand(command.Context(), deps, editor),
				resourcePayloadEditFilename(cfg),
				encoded,
			)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(editedBytes)) == 0 {
				return cliutil.ValidationError("edited resource payload is empty", nil)
			}

			editedValue, err := decodeResourcePayloadFromEdit(cfg, editedBytes)
			if err != nil {
				return err
			}

			if err := resourcesave.Execute(
				command.Context(),
				resourcesave.Dependencies{
					Orchestrator: deps.Orchestrator,
					Repository:   deps.ResourceStore,
					Metadata:     deps.Metadata,
					Secrets:      deps.Secrets,
				},
				resolvedPath,
				editedValue,
				true,
				resourcesave.ExecuteOptions{
					AsOneResource: true,
					Force:         true,
				},
			); err != nil {
				return err
			}

			if err := commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				fmt.Sprintf("declarest: edit resource %s", resolvedPath),
			); err != nil {
				return err
			}

			if cliutil.IsVerbose(globalFlags) {
				return cliutil.WriteText(command, cliutil.OutputText, resolvedPath)
			}
			return nil
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	cliutil.BindEditorFlag(command, &editor)
	return command
}
