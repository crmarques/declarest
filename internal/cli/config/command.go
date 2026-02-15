package config

import (
	"context"
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage contexts",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newCreateCommand(deps),
		newUpdateCommand(deps),
		newDeleteCommand(deps),
		newRenameCommand(deps),
		newListCommand(deps, globalFlags),
		newUseCommand(deps),
		newCurrentCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newValidateCommand(deps),
	)

	return command
}

func newCreateCommand(deps common.CommandWiring) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := common.DecodeInput[configdomain.Context](command, input)
			if err != nil {
				return err
			}
			return contexts.Create(context.Background(), cfg)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newUpdateCommand(deps common.CommandWiring) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "update",
		Short: "Update a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := common.DecodeInput[configdomain.Context](command, input)
			if err != nil {
				return err
			}
			return contexts.Update(context.Background(), cfg)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newDeleteCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			return contexts.Delete(context.Background(), args[0])
		},
	}
}

func newRenameCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <from> <to>",
		Short: "Rename a context",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			return contexts.Rename(context.Background(), args[0], args[1])
		},
	}
}

func newListCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List contexts",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			items, err := contexts.List(context.Background())
			if err != nil {
				return err
			}
			return common.WriteOutput(command, globalFlags.Output, items, func(w io.Writer, value []configdomain.Context) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintln(w, item.Name); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}
}

func newUseCommand(deps common.CommandWiring) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			return contexts.SetCurrent(context.Background(), args[0])
		},
	}
}

func newCurrentCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Get current context",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			current, err := contexts.GetCurrent(context.Background())
			if err != nil {
				return err
			}
			return common.WriteOutput(command, globalFlags.Output, current, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}
}

func newResolveCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	var overrides []string

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve active context with overrides",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			overridesMap, err := parseOverrides(overrides)
			if err != nil {
				return err
			}

			resolved, err := contexts.ResolveContext(context.Background(), configdomain.ContextSelection{
				Name:      globalFlags.Context,
				Overrides: overridesMap,
			})
			if err != nil {
				return err
			}

			return common.WriteOutput(command, globalFlags.Output, resolved, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}

	command.Flags().StringArrayVarP(&overrides, "set", "e", nil, "override key=value (repeatable)")
	return command
}

func newValidateCommand(deps common.CommandWiring) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := common.DecodeInput[configdomain.Context](command, input)
			if err != nil {
				return err
			}
			return contexts.Validate(context.Background(), cfg)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func parseOverrides(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, common.ValidationError("invalid override: expected key=value", nil)
		}
		parsed[strings.TrimSpace(parts[0])] = parts[1]
	}

	return parsed, nil
}
