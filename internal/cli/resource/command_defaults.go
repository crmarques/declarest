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
	"strconv"
	"strings"
	"time"

	configdomain "github.com/crmarques/declarest/config"
	defaultsapp "github.com/crmarques/declarest/internal/app/resource/defaults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	metadatadomain "github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

var defaultsInferPromptConfirm = cliutil.PromptConfirm

func newDefaultsCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "defaults",
		Short: "Manage metadata-backed resource defaults",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	editCommand := newDefaultsEditCommand(deps, globalFlags)
	configCommand := newDefaultsConfigCommand(deps, globalFlags)
	profileCommand := newDefaultsProfileCommand(deps, globalFlags)
	inferCommand := newDefaultsInferCommand(deps, globalFlags)
	commandmeta.MarkEmitsExecutionStatus(editCommand)
	commandmeta.MarkEmitsExecutionStatus(configCommand)
	commandmeta.MarkEmitsExecutionStatus(profileCommand)
	commandmeta.MarkEmitsExecutionStatus(inferCommand)

	command.AddCommand(
		newDefaultsGetCommand(deps, globalFlags),
		editCommand,
		configCommand,
		profileCommand,
		inferCommand,
	)
	return command
}

func newDefaultsGetCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read effective defaults values for a path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			result, err := defaultsapp.Get(command.Context(), deps, resolvedPath)
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

func newDefaultsEditCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var editor string

	command := &cobra.Command{
		Use:   "edit [path]",
		Short: "Edit the local baseline defaults object",
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
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource defaults edit"); err != nil {
				return err
			}

			current, err := defaultsapp.GetLocalBaseline(command.Context(), deps, requestedPath)
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
				"defaults"+resourcedomain.NormalizePayloadDescriptor(current.Content.Descriptor).Extension,
				encoded,
			)
			if err != nil {
				return err
			}

			editedValue := any(map[string]any{})
			if len(bytes.TrimSpace(editedBytes)) > 0 {
				editedValue, err = resourcedomain.DecodePayload(editedBytes, payloadType)
				if err != nil {
					return err
				}
			}

			saved, err := defaultsapp.Save(
				command.Context(),
				deps,
				requestedPath,
				resourcedomain.Content{
					Value:      editedValue,
					Descriptor: current.Content.Descriptor,
				},
			)
			if err != nil {
				return err
			}

			if err := commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				fmt.Sprintf("declarest: edit resource defaults %s", saved.ResolvedPath),
			); err != nil {
				return err
			}

			if cliutil.IsVerbose(globalFlags) {
				return cliutil.WriteText(command, cliutil.OutputText, saved.ResolvedPath)
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

func newDefaultsInferCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var save bool
	var check bool
	var fromValue string
	var itemsValue string
	var waitValue string
	var yes bool

	command := &cobra.Command{
		Use:   "infer [path]",
		Short: "Infer baseline defaults values for a collection",
		Example: strings.Join([]string{
			"  declarest resource defaults infer /customers/acme",
			"  declarest resource defaults infer /customers/acme --check",
			"  declarest resource defaults infer /customers/acme --save",
			"  declarest resource defaults infer /customers/acme --from managed-service --yes",
			"  declarest resource defaults infer /customers/acme --from managed-service --wait 2s --yes",
			"  declarest resource defaults infer /customers/acme --from repository,managed-service --items acme,beta",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			sources, err := parseDefaultsInferSources(fromValue)
			if err != nil {
				return err
			}
			items, err := parseDefaultsInferItems(itemsValue)
			if err != nil {
				return err
			}
			wait, waitSet, err := parseManagedServiceDefaultsWait(waitValue)
			if err != nil {
				return err
			}
			if save && check {
				return cliutil.ValidationError("flags --save and --check cannot be combined", nil)
			}
			usesManagedService := defaultsInferUsesSource(sources, defaultsapp.InferSourceManagedService)
			if waitSet && !usesManagedService {
				return cliutil.ValidationError("flag --wait requires --from to include managed-service", nil)
			}
			if usesManagedService && !yes {
				if !cliutil.IsInteractiveTerminal(command) {
					return cliutil.ValidationError(
						"flag --yes is required when --from includes managed-service because temporary remote resources will be created",
						nil,
					)
				}
				confirmed, confirmErr := defaultsInferPromptConfirm(
					command,
					"Proceed with managed-service defaults inference? Temporary remote resources will be created and deleted.",
					false,
				)
				if confirmErr != nil {
					return confirmErr
				}
				if !confirmed {
					return cliutil.ValidationError("managed-service defaults inference aborted", nil)
				}
			}

			var cfg configdomain.Context
			if save {
				activeCfg, cfgErr := resolveActiveResourceContext(command.Context(), deps, globalFlags)
				if cfgErr != nil {
					return cfgErr
				}
				if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, activeCfg, "resource defaults infer"); err != nil {
					return err
				}
				cfg = activeCfg
			}

			if check {
				result, checkErr := defaultsapp.Check(
					command.Context(),
					deps,
					resolvedPath,
					defaultsapp.CheckRequest{
						Sources: sources,
						Items:   items,
						Wait:    wait,
					},
				)
				if checkErr != nil {
					return checkErr
				}

				outputFormat, outputErr := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, result.InferredContent)
				if outputErr != nil {
					return outputErr
				}
				if outputErr := cliutil.WriteOutput(command, outputFormat, result.InferredContent.Value, nil); outputErr != nil {
					return outputErr
				}
				if !result.Matches {
					return cliutil.ValidationError(
						fmt.Sprintf(
							"resource defaults check failed for %q: inferred defaults do not match the current resolved defaults; rerun with --save to update the local baseline defaults",
							result.ResolvedPath,
						),
						nil,
					)
				}
				return nil
			}

			inferred, err := defaultsapp.Infer(
				command.Context(),
				deps,
				resolvedPath,
				defaultsapp.InferRequest{
					Sources: sources,
					Items:   items,
					Wait:    wait,
				},
			)
			if err != nil {
				return err
			}

			if save {
				saved, saveErr := defaultsapp.Save(command.Context(), deps, inferred.ResolvedPath, inferred.Content)
				if saveErr != nil {
					return saveErr
				}

				if err := commitAndMaybeAutoSyncRepository(
					command.Context(),
					deps,
					cfg,
					fmt.Sprintf("declarest: infer resource defaults %s", saved.ResolvedPath),
				); err != nil {
					return err
				}
				return nil
			}

			outputFormat, err := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, inferred.Content)
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, outputFormat, inferred.Content.Value, nil)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&save, "save", false, "save inferred defaults into the local baseline defaults file")
	command.Flags().BoolVar(&check, "check", false, "infer defaults and fail if they do not match the current resolved defaults")
	command.Flags().StringVar(&fromValue, "from", string(defaultsapp.InferSourceRepository), "comma-separated inference sources: repository and/or managed-service")
	command.Flags().StringVar(&itemsValue, "items", "", "comma-separated item aliases to use when inferring defaults")
	command.Flags().StringVar(&waitValue, "wait", "", "with --from managed-service, wait this long before reading temporary probe resources (for example 2s or 500ms; bare integers mean seconds)")
	command.Flags().BoolVarP(&yes, "yes", "y", false, "confirm managed-service temporary resource creation")
	return command
}

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

func parseManagedServiceDefaultsWait(value string) (time.Duration, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false, nil
	}

	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds < 0 {
			return 0, true, cliutil.ValidationError("flag --wait must be non-negative", nil)
		}
		return time.Duration(seconds) * time.Second, true, nil
	}

	wait, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, true, cliutil.ValidationError("flag --wait must be a Go duration like 2s or a whole number of seconds", err)
	}
	if wait < 0 {
		return 0, true, cliutil.ValidationError("flag --wait must be non-negative", nil)
	}
	return wait, true, nil
}

func parseDefaultsInferSources(value string) ([]defaultsapp.InferSource, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []defaultsapp.InferSource{defaultsapp.InferSourceRepository}, nil
	}

	sources := make([]defaultsapp.InferSource, 0, 2)
	seen := make(map[defaultsapp.InferSource]struct{}, 2)
	for _, rawItem := range strings.Split(trimmed, ",") {
		item := defaultsapp.InferSource(strings.TrimSpace(rawItem))
		if item == "" {
			return nil, cliutil.ValidationError(
				"flag --from must be a comma-separated list of: repository, managed-service",
				nil,
			)
		}
		switch item {
		case defaultsapp.InferSourceRepository, defaultsapp.InferSourceManagedService:
		default:
			return nil, cliutil.ValidationError(
				"flag --from must be a comma-separated list of: repository, managed-service",
				nil,
			)
		}
		if _, found := seen[item]; found {
			continue
		}
		seen[item] = struct{}{}
		sources = append(sources, item)
	}
	if len(sources) == 0 {
		return nil, cliutil.ValidationError(
			"flag --from must be a comma-separated list of: repository, managed-service",
			nil,
		)
	}
	return sources, nil
}

func parseDefaultsInferItems(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	items := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, rawItem := range strings.Split(trimmed, ",") {
		item := strings.TrimSpace(rawItem)
		if item == "" {
			return nil, cliutil.ValidationError("flag --items contains an empty alias", nil)
		}
		if _, found := seen[item]; found {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}
	return items, nil
}

func defaultsInferUsesSource(sources []defaultsapp.InferSource, target defaultsapp.InferSource) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}
