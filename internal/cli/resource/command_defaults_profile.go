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
	"strings"

	defaultsapp "github.com/crmarques/declarest/internal/app/resource/defaults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newDefaultsProfileCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "profile",
		Short: "Manage named defaults profiles",
		Args:  cobra.NoArgs,
	}

	editCommand := newDefaultsProfileEditCommand(deps, globalFlags)
	deleteCommand := newDefaultsProfileDeleteCommand(deps, globalFlags)
	commandmeta.MarkEmitsExecutionStatus(editCommand)
	commandmeta.MarkEmitsExecutionStatus(deleteCommand)
	command.AddCommand(
		newDefaultsProfileGetCommand(deps, globalFlags),
		editCommand,
		deleteCommand,
	)
	return command
}

func newDefaultsProfileGetCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path] [profile]",
		Short: "Read one effective defaults profile object",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, profile, err := resolveDefaultsProfileArgs(pathFlag, args)
			if err != nil {
				return err
			}

			result, err := defaultsapp.GetProfile(command.Context(), deps, resolvedPath, profile)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, result.Content)
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, outputFormat, result.Content.Value, nil)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	return command
}

func newDefaultsProfileEditCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var editor string

	command := &cobra.Command{
		Use:   "edit [path] [profile]",
		Short: "Edit one local defaults profile object",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(command *cobra.Command, args []string) error {
			requestedPath, profile, err := resolveDefaultsProfileArgs(pathFlag, args)
			if err != nil {
				return err
			}

			cfg, err := resolveActiveResourceContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource defaults profile edit"); err != nil {
				return err
			}

			current, err := defaultsapp.GetLocalProfile(command.Context(), deps, requestedPath, profile)
			if err != nil {
				return err
			}

			payloadType := resourcedomain.NormalizePayloadDescriptor(current.Content.Descriptor).PayloadType
			encoded, err := resourcedomain.EncodePayloadPretty(current.Content.Value, payloadType)
			if err != nil {
				return err
			}

			editedBytes, err := editTempFile(
				command,
				cliutil.ResolveEditorCommand(command.Context(), deps, editor),
				"profile-"+profile+resourcedomain.NormalizePayloadDescriptor(current.Content.Descriptor).Extension,
				encoded,
			)
			if err != nil {
				return err
			}

			if len(bytes.TrimSpace(editedBytes)) == 0 {
				if err := defaultsapp.DeleteProfile(command.Context(), deps, requestedPath, profile); err != nil {
					return err
				}
			} else {
				editedValue, err := resourcedomain.DecodePayload(editedBytes, payloadType)
				if err != nil {
					return err
				}
				if _, err := defaultsapp.SaveProfile(command.Context(), deps, requestedPath, profile, resourcedomain.Content{
					Value:      editedValue,
					Descriptor: current.Content.Descriptor,
				}); err != nil {
					return err
				}
			}

			if err := commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				fmt.Sprintf("declarest: edit resource defaults profile %s %s", requestedPath, profile),
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

func newDefaultsProfileDeleteCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "delete [path] [profile]",
		Short: "Delete one local defaults profile object",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(command *cobra.Command, args []string) error {
			requestedPath, profile, err := resolveDefaultsProfileArgs(pathFlag, args)
			if err != nil {
				return err
			}

			cfg, err := resolveActiveResourceContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource defaults profile delete"); err != nil {
				return err
			}

			if err := defaultsapp.DeleteProfile(command.Context(), deps, requestedPath, profile); err != nil {
				return err
			}
			if err := commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				fmt.Sprintf("declarest: delete resource defaults profile %s %s", requestedPath, profile),
			); err != nil {
				return err
			}
			return nil
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	return command
}

func resolveDefaultsProfileArgs(pathFlag string, args []string) (string, string, error) {
	if strings.TrimSpace(pathFlag) != "" {
		if len(args) != 1 {
			return "", "", cliutil.ValidationError("profile is required", nil)
		}
		if strings.TrimSpace(args[0]) == "" {
			return "", "", cliutil.ValidationError("profile is required", nil)
		}
		return pathFlag, args[0], nil
	}
	if len(args) == 0 {
		return "", "", cliutil.ValidationError("path is required", nil)
	}
	if len(args) == 1 {
		return "", "", cliutil.ValidationError("profile is required", nil)
	}

	resolvedPath, err := cliutil.ResolvePathInput("", args[:1], true)
	if err != nil {
		return "", "", err
	}

	profile := args[1]
	if strings.TrimSpace(profile) == "" {
		return "", "", cliutil.ValidationError("profile is required", nil)
	}
	return resolvedPath, profile, nil
}
