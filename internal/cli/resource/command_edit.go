package resource

import (
	"bytes"
	"fmt"

	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

var editTempFile = cliutil.EditTempFile

func newEditCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var editor string

	command := &cobra.Command{
		Use:   "edit [path]",
		Short: "Edit a local repository resource in an editor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			requestedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
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

			resolvedPath, currentValue, err := resolveEditSource(command.Context(), deps, requestedPath)
			if err != nil {
				return err
			}
			payloadType, err := resourcePayloadEditType(command.Context(), deps, cfg, resolvedPath, currentValue)
			if err != nil {
				return err
			}
			encoded, err := encodeResourcePayloadForEdit(payloadType, currentValue)
			if err != nil {
				return err
			}

			editedBytes, err := editTempFile(
				command,
				cliutil.ResolveEditorCommand(command.Context(), deps, editor),
				resourcePayloadEditFilename(payloadType),
				encoded,
			)
			if err != nil {
				return err
			}
			if resourcedomain.IsStructuredPayloadType(payloadType) && len(bytes.TrimSpace(editedBytes)) == 0 {
				return cliutil.ValidationError("edited resource payload is empty", nil)
			}

			editedValue, err := decodeResourcePayloadFromEdit(payloadType, editedBytes)
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
