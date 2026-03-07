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
			return cliutil.WriteOutput(command, outputFormat, templated, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
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
