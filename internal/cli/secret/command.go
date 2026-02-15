package secret

import (
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newInitCommand(deps),
		newStoreCommand(deps),
		newGetCommand(deps, globalFlags),
		newDeleteCommand(deps),
		newListCommand(deps, globalFlags),
		newMaskCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newNormalizeCommand(deps, globalFlags),
		newDetectCommand(deps, globalFlags),
	)

	return command
}

func newInitCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secret store",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Init(command.Context())
		},
	}
}

func newStoreCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "store <key> <value>",
		Short: "Store a secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Store(command.Context(), args[0], args[1])
		},
	}
}

func newGetCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := secretProvider.Get(command.Context(), args[0])
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item string) error {
				_, writeErr := io.WriteString(w, item+"\n")
				return writeErr
			})
		},
	}
}

func newDeleteCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Delete(command.Context(), args[0])
		},
	}
}

func newListCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List secrets",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			items, err := secretProvider.List(command.Context())
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, nil)
		},
	}
}

func newMaskCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "mask",
		Short: "Mask secret values in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			masked, err := secretProvider.MaskPayload(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, masked, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newResolveCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve secret placeholders in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			resolved, err := secretProvider.ResolvePayload(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, resolved, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newNormalizeCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize secret placeholders",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			normalized, err := secretProvider.NormalizeSecretPlaceholders(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, normalized, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newDetectCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "detect",
		Short: "Detect potential secrets in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			keys, err := secretProvider.DetectSecretCandidates(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, keys, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}
