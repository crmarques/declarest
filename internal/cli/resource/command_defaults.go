package resource

import (
	"bytes"
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	defaultsapp "github.com/crmarques/declarest/internal/app/resource/defaults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newDefaultsCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "defaults",
		Short: "Manage raw resource defaults sidecars",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	editCommand := newDefaultsEditCommand(deps, globalFlags)
	inferCommand := newDefaultsInferCommand(deps, globalFlags)
	commandmeta.MarkEmitsExecutionStatus(editCommand)
	commandmeta.MarkEmitsExecutionStatus(inferCommand)

	command.AddCommand(
		newDefaultsGetCommand(deps, globalFlags),
		editCommand,
		inferCommand,
	)
	return command
}

func newDefaultsGetCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read raw defaults values from the repository",
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
		Short: "Edit raw defaults values in an editor",
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

			current, err := defaultsapp.Get(command.Context(), deps, requestedPath)
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
				current.ResolvedPath,
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
	var managedServer bool
	var yes bool

	command := &cobra.Command{
		Use:   "infer [path]",
		Short: "Infer raw defaults values for a resource",
		Example: strings.Join([]string{
			"  declarest resource defaults infer /customers/acme",
			"  declarest resource defaults infer /customers/acme --save",
			"  declarest resource defaults infer /customers/acme --managed-server --yes",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			if managedServer && !yes {
				return cliutil.ValidationError("flag --yes is required with --managed-server because temporary remote resources will be created", nil)
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

			inferred, err := defaultsapp.Infer(
				command.Context(),
				deps,
				resolvedPath,
				defaultsapp.InferRequest{ManagedServer: managedServer},
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
	command.Flags().BoolVar(&save, "save", false, "save inferred defaults into the repository")
	command.Flags().BoolVar(&managedServer, "managed-server", false, "probe the managed server by creating temporary resources before inferring defaults")
	command.Flags().BoolVarP(&yes, "yes", "y", false, "confirm managed-server temporary resource creation")
	return command
}
