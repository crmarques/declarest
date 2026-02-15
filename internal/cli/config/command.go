package config

import (
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	return newCommandWithPrompter(deps, globalFlags, terminalPrompter{})
}

func newCommandWithPrompter(
	deps common.CommandWiring,
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage contexts",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newCreateCommand(deps, prompter),
		newUpdateCommand(deps),
		newDeleteCommand(deps, prompter),
		newRenameCommand(deps, prompter),
		newListCommand(deps, globalFlags),
		newUseCommand(deps, prompter),
		newShowCommand(deps, globalFlags, prompter),
		newCurrentCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newValidateCommand(deps),
	)

	return command
}

func newCreateCommand(deps common.CommandWiring, prompter configPrompter) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a context from input or interactive prompts",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := resolveCreateContextInput(command, input, prompter)
			if err != nil {
				return err
			}
			return contexts.Create(command.Context(), cfg)
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
			cfg, err := decodeContextStrict(command, input)
			if err != nil {
				return err
			}
			return contexts.Update(command.Context(), cfg)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newDeleteCommand(deps common.CommandWiring, prompter configPrompter) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				selected, err := selectContextForAction(command, contexts, prompter, "delete")
				if err != nil {
					return err
				}
				confirmed, err := prompter.Confirm(command, fmt.Sprintf("Delete context %q?", selected), false)
				if err != nil {
					return err
				}
				if !confirmed {
					return common.WriteText(command, common.OutputText, "delete canceled")
				}
				name = selected
			}
			return contexts.Delete(command.Context(), name)
		},
	}
}

func newRenameCommand(deps common.CommandWiring, prompter configPrompter) *cobra.Command {
	return &cobra.Command{
		Use:   "rename [from] [to]",
		Short: "Rename a context (interactive when args are omitted)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			fromName := ""
			toName := ""
			switch len(args) {
			case 2:
				fromName = args[0]
				toName = args[1]
			case 1:
				fromName = args[0]
				if !prompter.IsInteractive(command) {
					return common.ValidationError("new context name is required", nil)
				}
				toName, err = prompter.Input(command, "New context name: ", true)
				if err != nil {
					return err
				}
			default:
				fromName, err = selectContextForAction(command, contexts, prompter, "rename")
				if err != nil {
					return err
				}
				toName, err = prompter.Input(command, "New context name: ", true)
				if err != nil {
					return err
				}
			}

			return contexts.Rename(command.Context(), fromName, toName)
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
			items, err := contexts.List(command.Context())
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

func newUseCommand(deps common.CommandWiring, prompter configPrompter) *cobra.Command {
	return &cobra.Command{
		Use:   "use [name]",
		Short: "Set current context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				name, err = selectContextForAction(command, contexts, prompter, "use")
				if err != nil {
					return err
				}
			}
			return contexts.SetCurrent(command.Context(), name)
		},
	}
}

func newShowCommand(
	deps common.CommandWiring,
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show a context from --context or interactive selection",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if globalFlags != nil {
				name = strings.TrimSpace(globalFlags.Context)
			}
			if name == "" {
				name, err = selectContextForAction(command, contexts, prompter, "show --context")
				if err != nil {
					return err
				}
			}

			shown, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{Name: name})
			if err != nil {
				return err
			}

			return common.WriteOutput(command, common.OutputYAML, shown, nil)
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
			current, err := contexts.GetCurrent(command.Context())
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

			resolved, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{
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
			cfg, err := decodeContextStrict(command, input)
			if err != nil {
				return err
			}
			return contexts.Validate(command.Context(), cfg)
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
