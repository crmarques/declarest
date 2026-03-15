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
			payloadType, err := resourcePayloadEditType(command.Context(), deps, cfg, resolvedPath, currentValue.Value)
			if err != nil {
				return err
			}
			encoded, err := encodeResourcePayloadForEdit(payloadType, currentValue.Value)
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
				deps,
				resolvedPath,
				resourcedomain.Content{
					Value:      editedValue,
					Descriptor: currentValue.Descriptor,
				},
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
