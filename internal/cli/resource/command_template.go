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
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newTemplateCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "template [path]",
		Short: "Render payload templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			value, err := resourceinputapp.DecodeRequiredPayloadInput(command, input)
			if err != nil {
				return err
			}

			orchestratorService, err := cliutil.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			templated, err := orchestratorService.Template(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, templated)
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, outputFormat, templated, func(w io.Writer, item resource.Content) error {
				_, writeErr := fmt.Fprintln(w, item.Value)
				return writeErr
			})
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	cliutil.BindResourceInputFlags(command, &input)
	return command
}
