package secret

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newInitCommand(),
		newStoreCommand(),
		newGetCommand(),
		newDeleteCommand(),
		newListCommand(),
		newMaskCommand(),
		newResolveCommand(),
		newNormalizeCommand(),
		newDetectCommand(),
	)

	return command
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secret store",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("Secret", "Init")
		},
	}
}

func newStoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "store <key> <value>",
		Short: "Store a secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("Secret", "Store")
		},
	}
}

func newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("Secret", "Get")
		},
	}
}

func newDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("Secret", "Delete")
		},
	}
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List secrets",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("Secret", "List")
		},
	}
}

func newMaskCommand() *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "mask",
		Short: "Mask secret values in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := common.DecodeInput[resource.Value](command, input); err != nil {
				return err
			}
			return common.NotImplementedError("Secret", "Mask")
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newResolveCommand() *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve secret placeholders in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := common.DecodeInput[resource.Value](command, input); err != nil {
				return err
			}
			return common.NotImplementedError("Secret", "Resolve")
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newNormalizeCommand() *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize secret placeholders",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := common.DecodeInput[resource.Value](command, input); err != nil {
				return err
			}
			return common.NotImplementedError("Secret", "Normalize")
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newDetectCommand() *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "detect",
		Short: "Detect potential secrets in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := common.DecodeInput[resource.Value](command, input); err != nil {
				return err
			}
			return common.NotImplementedError("Secret", "Detect")
		},
	}

	common.BindInputFlags(command, &input)
	return command
}
