package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/shared"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newTemplateCommand(deps shared.CommandDependencies, globalFlags *shared.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input shared.InputFlags

	command := &cobra.Command{
		Use:   "template [path]",
		Short: "Render payload templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := shared.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := shared.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := resourceinputapp.DecodeRequiredPayloadInput(command, input)
			if err != nil {
				return err
			}

			orchestratorService, err := shared.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			templated, err := orchestratorService.Template(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			return shared.WriteOutput(command, outputFormat, templated, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
				return writeErr
			})
		},
	}

	shared.BindPathFlag(command, &pathFlag)
	shared.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = shared.SinglePathArgCompletionFunc(deps)
	shared.BindInputFlags(command, &input)
	return command
}
