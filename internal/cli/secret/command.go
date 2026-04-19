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

package secret

import (
	"fmt"
	"io"
	"strings"

	detectapp "github.com/crmarques/declarest/internal/app/secret/detect"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		Args:  cobra.NoArgs,
	}
	commandmeta.MarkRequiresContextBootstrap(command)

	getCommand := newGetCommand(deps)
	commandmeta.MarkTextOnlyOutput(getCommand)

	command.AddCommand(
		newInitCommand(deps),
		newSetCommand(deps),
		getCommand,
		newListCommand(deps, globalFlags),
		newDeleteCommand(deps),
		newMaskCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newNormalizeCommand(deps, globalFlags),
		newDetectCommand(deps, globalFlags),
	)

	return command
}

func newInitCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secret store",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Init(command.Context())
		},
	}
}

func newSetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:     "set [path] [key] [value]",
		Aliases: []string{"store"},
		Short:   "Set a secret",
		Example: strings.Join([]string{
			"  declarest secret set apiToken super-secret",
			"  declarest secret set /customers/acme /apiToken super-secret",
			"  declarest secret set --path /customers/acme --key /apiToken super-secret",
			"  declarest secret set /customers/acme:/apiToken super-secret",
		}, "\n"),
		Args: cobra.RangeArgs(1, 3),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			request, err := resolveSecretSetRequest(pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			return secretProvider.Store(command.Context(), request.Target.ResolvedKey(), request.Value)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

func newGetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:   "get [path] [key]",
		Short: "Read one secret",
		Example: strings.Join([]string{
			"  declarest secret get apiToken",
			"  declarest secret list /customers/acme",
			"  declarest secret get /customers/acme /apiToken",
			"  declarest secret get --path /customers/acme --key /apiToken",
			"  declarest secret get /customers/acme:/apiToken",
		}, "\n"),
		Args: cobra.RangeArgs(0, 2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			target, err := resolveSecretTargetRequest("get", pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			value, err := secretProvider.Get(command.Context(), target.ResolvedKey())
			if err != nil {
				return err
			}
			_, err = io.WriteString(command.OutOrStdout(), value+"\n")
			return err
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

func newListCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var recursive bool

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List stored secret keys",
		Example: strings.Join([]string{
			"  declarest secret list",
			"  declarest secret list /customers/acme",
			"  declarest secret list --path /customers/acme",
			"  declarest secret list /projects/test --recursive",
			"  declarest secret list /customers/acme --output json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			request, err := resolveSecretListRequest(pathFlag, recursive, args)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if globalFlags != nil && globalFlags.Output == cliutil.OutputAuto {
				outputFormat = cliutil.OutputAuto
			}

			items, err := listSecretKeys(command.Context(), secretProvider, request)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, items, func(w io.Writer, items []string) error {
				for _, item := range items {
					if _, writeErr := fmt.Fprintln(w, item); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "include descendant secret paths")
	return command
}

func newDeleteCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:   "delete [path] [key]",
		Short: "Delete one secret",
		Example: strings.Join([]string{
			"  declarest secret delete apiToken",
			"  declarest secret delete /customers/acme /apiToken",
			"  declarest secret delete --path /customers/acme --key /apiToken",
			"  declarest secret delete /customers/acme:/apiToken",
		}, "\n"),
		Args: cobra.RangeArgs(0, 2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			target, err := resolveSecretTargetRequest("delete", pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			return secretProvider.Delete(command.Context(), target.ResolvedKey())
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

func newMaskCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "mask",
		Short: "Mask secret values in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			masked, err := secretProvider.MaskPayload(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, masked, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newResolveCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve secret placeholders in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			resolved, err := secretProvider.ResolvePayload(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, resolved, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newNormalizeCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize secret placeholders",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			normalized, err := secretProvider.NormalizeSecretPlaceholders(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, normalized, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newDetectCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags
	var pathFlag string
	var fix bool
	var secretAttribute string

	command := &cobra.Command{
		Use:   "detect [path]",
		Short: "Detect potential secrets in payload or local resources",
		Example: strings.Join([]string{
			"  declarest secret detect /customers/",
			"  declarest secret detect --fix /customers/",
			"  declarest secret detect --secret-attribute /apiToken < payload.json",
			"  declarest secret detect --fix --path /customers/acme < payload.json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, false)
			if err != nil {
				return err
			}

			value, hasInput, err := decodeDetectInput(command, input)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			result, err := detectapp.Execute(command.Context(), deps, detectapp.Request{
				ResolvedPath:    resolvedPath,
				Value:           value,
				HasInput:        hasInput,
				Fix:             fix,
				SecretAttribute: secretAttribute,
			})
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, result.Output, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&fix, "fix", false, "write detected secret attributes to metadata")
	command.Flags().StringVar(&secretAttribute, "secret-attribute", "", "apply only one detected JSON pointer attribute")
	return command
}
