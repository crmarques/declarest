package metadata

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	inferCommand := common.NewPlaceholderCommand("infer")
	inferCommand.Flags().Bool("apply", false, common.PlaceholderMessage)
	inferCommand.Flags().Bool("recursive", false, common.PlaceholderMessage)

	command := common.NewPlaceholderCommand("metadata")
	command.AddCommand(
		common.NewPlaceholderCommand("get"),
		common.NewPlaceholderCommand("set"),
		common.NewPlaceholderCommand("unset"),
		common.NewPlaceholderCommand("resolve-for-path"),
		common.NewPlaceholderCommand("render-operation-spec"),
		inferCommand,
	)

	return command
}
