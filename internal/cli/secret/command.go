package secret

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	command := common.NewPlaceholderCommand("secret")
	command.AddCommand(
		common.NewPlaceholderCommand("init"),
		common.NewPlaceholderCommand("store"),
		common.NewPlaceholderCommand("get"),
		common.NewPlaceholderCommand("delete"),
		common.NewPlaceholderCommand("list"),
		common.NewPlaceholderCommand("mask-payload"),
		common.NewPlaceholderCommand("resolve-payload"),
		common.NewPlaceholderCommand("normalize-secret-placeholders"),
		common.NewPlaceholderCommand("detect-secret-candidates"),
	)

	return command
}
