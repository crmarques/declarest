package repo

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	resetCommand := common.NewPlaceholderCommand("reset")
	resetCommand.Flags().Bool("hard", false, common.PlaceholderMessage)

	pushCommand := common.NewPlaceholderCommand("push")
	pushCommand.Flags().Bool("force", false, common.PlaceholderMessage)

	command := common.NewPlaceholderCommand("repo")
	command.AddCommand(
		common.NewPlaceholderCommand("init"),
		common.NewPlaceholderCommand("refresh"),
		resetCommand,
		common.NewPlaceholderCommand("check"),
		pushCommand,
		common.NewPlaceholderCommand("force-push"),
		common.NewPlaceholderCommand("pull-status"),
	)

	return command
}
