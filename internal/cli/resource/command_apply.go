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
	"strings"

	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func newApplyCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input cliutil.InputFlags
	var recursive bool
	var force bool
	var httpMethod string
	var refresh bool

	command := &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply local desired state (create-or-update remote)",
		Long: strings.Join([]string{
			"Apply desired state from the resource repository by default.",
			"When --payload <path|-> or stdin is provided, the explicit payload overrides repository input for a single target path.",
			"Apply uses upsert behavior for remote writes: create when the resource does not exist, update when it differs.",
			"When remote and desired state are equal after metadata compare transforms, apply skips updates unless --force is set.",
			"This explicit-input mode is useful for direct remote operations when no repository is configured.",
			"Use --refresh to fetch the remote state after each mutation and persist it back into the repository.",
		}, " "),
		Example: strings.Join([]string{
			"  declarest resource apply /customers/acme",
			"  declarest resource apply /customers/ --recursive",
			"  declarest resource apply /customers/acme --payload payload.json",
			"  cat payload.json | declarest resource apply /customers/acme --payload -",
			"  declarest resource apply /customers/acme --force",
			"  declarest resource apply /customers/acme --refresh",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			runCtx, _, err := applyHTTPMethodOverride(
				command.Context(),
				httpMethod,
				metadata.OperationCreate,
				metadata.OperationUpdate,
			)
			if err != nil {
				return err
			}

			value, hasExplicitInput, err := resourceinputapp.DecodeOptionalMutationPayloadInput(command, input)
			if err != nil {
				return err
			}

			mutationPath := resolvedPath
			if hasExplicitInput {
				mutationPath, err = resolveExplicitMutationPayloadPath(
					command.Context(),
					command.CommandPath(),
					deps,
					resolvedPath,
					value,
				)
				if err != nil {
					return err
				}
			}

			result, err := mutateapp.Execute(runCtx, deps, mutateapp.Request{
				Operation:        mutateapp.OperationApply,
				LogicalPath:      mutationPath,
				Recursive:        recursive,
				Force:            force,
				Value:            value,
				HasExplicitInput: hasExplicitInput,
				RefreshLocal:     refresh,
			})
			if err != nil {
				return err
			}

			if !cliutil.IsVerbose(globalFlags) {
				return nil
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			return writeCollectionMutationOutput(command, outputFormat, result.ResolvedPath, result.Items)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	cliutil.BindResourceInputFlags(command, &input)
	if flag := command.Flags().Lookup("payload"); flag != nil {
		flag.Usage = "payload file path (use '-' to read object from stdin); also accepts inline JSON/YAML, JSON Pointer assignments (/a=b,/c/d=e), or dot-notation assignments (a.b=x,c=y); binary requires file or stdin"
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	command.Flags().BoolVar(&force, "force", false, "force update even when compare output has no drift")
	command.Flags().BoolVar(&refresh, "refresh", false, "re-fetch remote mutation results into the repository")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
