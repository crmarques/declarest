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

	defaultsapp "github.com/crmarques/declarest/internal/app/resource/defaults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func newDefaultsConfigCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage the local defaults config block",
		Args:  cobra.NoArgs,
	}

	editCommand := newDefaultsConfigEditCommand(deps, globalFlags)
	commandmeta.MarkEmitsExecutionStatus(editCommand)
	command.AddCommand(
		newDefaultsConfigGetCommand(deps, globalFlags),
		editCommand,
	)
	return command
}

func newDefaultsConfigGetCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read the local defaults config block",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			result, err := defaultsapp.GetConfig(command.Context(), deps, resolvedPath)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, outputFormat, result.Defaults, nil)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	return command
}

func newDefaultsConfigEditCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var editor string

	command := &cobra.Command{
		Use:   "edit [path]",
		Short: "Edit the local defaults config block",
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
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource defaults config edit"); err != nil {
				return err
			}

			current, err := defaultsapp.GetConfig(command.Context(), deps, requestedPath)
			if err != nil {
				return err
			}

			encoded, err := yaml.Marshal(current.Defaults)
			if err != nil {
				return err
			}
			if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
				encoded = append(encoded, '\n')
			}

			editedBytes, err := editTempFile(
				command,
				cliutil.ResolveEditorCommand(command.Context(), deps, editor),
				"defaults-config.yaml",
				encoded,
			)
			if err != nil {
				return err
			}

			edited := metadatadomain.DefaultsSpec{}
			if len(bytes.TrimSpace(editedBytes)) > 0 {
				if err := yaml.Unmarshal(editedBytes, &edited); err != nil {
					return cliutil.ValidationError("invalid yaml defaults config", err)
				}
			}

			saved, err := defaultsapp.SaveConfig(command.Context(), deps, requestedPath, edited)
			if err != nil {
				return err
			}

			if err := commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				fmt.Sprintf("declarest: edit resource defaults config %s", saved.ResolvedPath),
			); err != nil {
				return err
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
