package config

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	command := common.NewPlaceholderCommand("config")
	command.AddCommand(
		common.NewPlaceholderCommand("create"),
		common.NewPlaceholderCommand("update"),
		common.NewPlaceholderCommand("delete"),
		common.NewPlaceholderCommand("rename"),
		common.NewPlaceholderCommand("list"),
		common.NewPlaceholderCommand("set-current"),
		common.NewPlaceholderCommand("get-current"),
		common.NewPlaceholderCommand("load-resolved-config"),
		common.NewPlaceholderCommand("validate"),
	)

	return command
}
