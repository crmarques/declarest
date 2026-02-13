package resource

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	deleteCommand := common.NewPlaceholderCommand("delete")
	deleteCommand.Flags().Bool("force", false, common.PlaceholderMessage)
	deleteCommand.Flags().Bool("recursive", false, common.PlaceholderMessage)

	command := common.NewPlaceholderCommand("resource")
	command.AddCommand(
		common.NewPlaceholderCommand("get"),
		common.NewPlaceholderCommand("save"),
		common.NewPlaceholderCommand("apply"),
		common.NewPlaceholderCommand("create"),
		common.NewPlaceholderCommand("update"),
		deleteCommand,
		common.NewPlaceholderCommand("diff"),
		common.NewPlaceholderCommand("list"),
		common.NewPlaceholderCommand("explain"),
		common.NewPlaceholderCommand("template"),
	)

	return command
}
