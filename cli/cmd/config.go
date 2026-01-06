package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ctx "declarest/internal/context"

	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	manager := &ctx.DefaultContextManager{}

	cmd := &cobra.Command{
		Use:     "config",
		GroupID: groupUserFacing,
		Short:   "Manage DeclaREST contexts and configuration files",
		Long: `Create and maintain contexts that describe how DeclaREST authenticates with managed servers
and which repository backs the resource definitions. Contexts allow you to switch between
environments (for example dev, staging, and production) with confidence.`,
		Example: `  # Add a new context from a configuration file
  declarest config add staging ./contexts/staging.yaml

  # Switch the active context
  declarest config use staging

  # Print a full config file
  declarest config print-template > ./contexts/staging.yaml`,
	}

	cmd.AddCommand(newConfigAddCommand(manager))
	cmd.AddCommand(newConfigUpdateCommand(manager))
	cmd.AddCommand(newConfigUseCommand(manager))
	cmd.AddCommand(newConfigDeleteCommand(manager))
	cmd.AddCommand(newConfigRenameCommand(manager))
	cmd.AddCommand(newConfigListCommand(manager))
	cmd.AddCommand(newConfigCurrentCommand(manager))
	cmd.AddCommand(newConfigCheckCommand(manager))
	cmd.AddCommand(newConfigPrintTemplateCommand())

	return cmd
}

func newConfigAddCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var (
		name   string
		config string
		force  bool
	)

	cmd := &cobra.Command{
		Use:     "add <name> <config>",
		Aliases: []string{"add-context"},
		Short:   "Register a new context using the provided configuration file",
		Long:    "Add a context record to the DeclaREST configuration store. If the context already exists, use update instead.",
		Example: `  declarest config add staging ./contexts/staging.yaml
  declarest config add prod ./contexts/prod.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, config, err = resolveAddArgs(cmd, name, config, args)
			if err != nil {
				return err
			}
			if config != "" {
				if name == "" {
					return usageError(cmd, "name is required when config path is provided")
				}
				if err := validateContextName(name); err != nil {
					return usageError(cmd, err.Error())
				}
				if force {
					exists, err := contextExists(manager, name)
					if err != nil {
						return err
					}
					if exists {
						return manager.UpdateContext(name, config)
					}
				}
				return manager.AddContext(name, config)
			}
			prompt := newPrompter(cmd.InOrStdin(), cmd.ErrOrStderr())
			return runInteractiveContextSetup(manager, prompt, name, force)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Context identifier to register")
	cmd.Flags().StringVar(&config, "config", "", "Path to the context configuration file")
	cmd.Flags().BoolVar(&force, "force", false, "Override an existing context definition")

	return cmd
}

func newConfigUpdateCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var (
		name   string
		config string
	)

	cmd := &cobra.Command{
		Use:     "update <name> <config>",
		Aliases: []string{"set-context", "update-context"},
		Short:   "Update an existing context definition",
		Long:    "Update an existing context by supplying a new configuration file. This does not change the active context.",
		Example: `  declarest config update staging ./contexts/staging.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, config, err = resolveNameAndConfig(cmd, name, config, args)
			if err != nil {
				return err
			}
			if err := validateContextName(name); err != nil {
				return usageError(cmd, err.Error())
			}
			return manager.UpdateContext(name, config)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Existing context identifier to update")
	cmd.Flags().StringVar(&config, "config", "", "Path to the context configuration file")

	return cmd
}

func newConfigUseCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:     "use <name>",
		Aliases: []string{"set-current-context", "use-context"},
		Short:   "Activate a context for subsequent operations",
		Long:    "Mark the selected context as the default. Subsequent resource and repository commands will use it automatically.",
		Example: `  declarest config use staging`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, err = resolveSingleArg(cmd, name, args, "name")
			if err != nil {
				return err
			}
			if err := validateContextName(name); err != nil {
				return usageError(cmd, err.Error())
			}
			return manager.SetDefaultContext(name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Context identifier to activate as default")

	return cmd
}

func newConfigDeleteCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var (
		name string
		yes  bool
	)

	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"delete-context"},
		Short:   "Remove a context definition from the configuration store",
		Long:    "Delete the named context from the DeclaREST configuration file. This does not remove any remote resources.",
		Example: `  declarest config delete staging`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, err = resolveSingleArg(cmd, name, args, "name")
			if err != nil {
				return err
			}
			if err := validateContextName(name); err != nil {
				return usageError(cmd, err.Error())
			}

			message := fmt.Sprintf("Delete context %q from %s?", name, configStoreLabel(manager))
			if err := confirmAction(cmd, yes, message); err != nil {
				return err
			}
			return manager.DeleteContext(name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Context identifier to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&yes, "force", false, "DEPRECATED: use --yes")
	_ = cmd.Flags().MarkHidden("force")

	return cmd
}

func newConfigCurrentCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	return &cobra.Command{
		Use:     "current",
		Aliases: []string{"display-current-context"},
		Short:   "Display the name of the context currently selected as default",
		Long:    "Print the name of the context that DeclaREST currently uses for resource and repository commands.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := manager.GetDefaultContext()
			if err != nil {
				return err
			}
			infof(cmd, "%s", name)
			return nil
		},
	}
}

func newConfigListCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"list-contexts"},
		Short:   "List all context names available in the configuration file",
		Long:    "Print every context defined in the DeclaREST configuration. The current context is highlighted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := manager.ListContexts()
			if err != nil {
				return err
			}
			current, _ := manager.GetDefaultContext()
			for _, name := range names {
				if name == current {
					infof(cmd, "* %s (current)", name)
					continue
				}
				infof(cmd, "- %s", name)
			}
			return nil
		},
	}
}

func newConfigRenameCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var (
		currentName string
		newName     string
	)

	cmd := &cobra.Command{
		Use:     "rename <current> <new>",
		Aliases: []string{"rename-context"},
		Short:   "Rename an existing context",
		Long:    "Change the identifier of an existing context without altering its configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			currentName, newName, err = resolveRenameArgs(cmd, currentName, newName, args)
			if err != nil {
				return err
			}
			if err := validateContextName(currentName); err != nil {
				return usageError(cmd, fmt.Sprintf("current name: %v", err))
			}
			if err := validateContextName(newName); err != nil {
				return usageError(cmd, fmt.Sprintf("new name: %v", err))
			}
			if currentName == newName {
				return usageError(cmd, "new name must differ from the current name")
			}
			return manager.RenameContext(currentName, newName)
		},
	}

	cmd.Flags().StringVar(&currentName, "current-name", "", "Existing context identifier")
	cmd.Flags().StringVar(&newName, "new-name", "", "New context identifier")

	return cmd
}

func validateContextName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func resolveNameAndConfig(cmd *cobra.Command, name, config string, args []string) (string, string, error) {
	if len(args) > 2 {
		return "", "", usageError(cmd, "expected <name> <config>")
	}
	name = strings.TrimSpace(name)
	config = strings.TrimSpace(config)

	if len(args) > 0 {
		argName := strings.TrimSpace(args[0])
		if argName != "" {
			if name != "" && name != argName {
				return "", "", usageError(cmd, "name specified twice")
			}
			if name == "" {
				name = argName
			}
		}
	}
	if len(args) > 1 {
		argConfig := strings.TrimSpace(args[1])
		if argConfig != "" {
			if config != "" && config != argConfig {
				return "", "", usageError(cmd, "config specified twice")
			}
			if config == "" {
				config = argConfig
			}
		}
	}
	if name == "" {
		return "", "", usageError(cmd, "name is required")
	}
	if config == "" {
		return "", "", usageError(cmd, "config file path is required")
	}
	return name, config, nil
}

func resolveAddArgs(cmd *cobra.Command, name, config string, args []string) (string, string, error) {
	if len(args) > 2 {
		return "", "", usageError(cmd, "expected <name> <config>")
	}
	name = strings.TrimSpace(name)
	config = strings.TrimSpace(config)

	if len(args) > 0 {
		argName := strings.TrimSpace(args[0])
		if argName != "" {
			if name != "" && name != argName {
				return "", "", usageError(cmd, "name specified twice")
			}
			name = argName
		}
	}
	if len(args) > 1 {
		argConfig := strings.TrimSpace(args[1])
		if argConfig != "" {
			if config != "" && config != argConfig {
				return "", "", usageError(cmd, "config specified twice")
			}
			config = argConfig
		}
	}
	return name, config, nil
}

func resolveSingleArg(cmd *cobra.Command, value string, args []string, label string) (string, error) {
	if len(args) > 1 {
		return "", usageError(cmd, fmt.Sprintf("expected <%s>", label))
	}
	value = strings.TrimSpace(value)
	if len(args) == 1 {
		arg := strings.TrimSpace(args[0])
		if arg != "" {
			if value != "" && value != arg {
				return "", usageError(cmd, fmt.Sprintf("%s specified twice", label))
			}
			if value == "" {
				value = arg
			}
		}
	}
	if strings.TrimSpace(value) == "" {
		return "", usageError(cmd, fmt.Sprintf("%s is required", label))
	}
	return value, nil
}

func resolveRenameArgs(cmd *cobra.Command, currentName, newName string, args []string) (string, string, error) {
	if len(args) > 2 {
		return "", "", usageError(cmd, "expected <current> <new>")
	}
	currentName = strings.TrimSpace(currentName)
	newName = strings.TrimSpace(newName)
	if len(args) > 0 {
		arg := strings.TrimSpace(args[0])
		if arg != "" {
			if currentName != "" && currentName != arg {
				return "", "", usageError(cmd, "current name specified twice")
			}
			if currentName == "" {
				currentName = arg
			}
		}
	}
	if len(args) > 1 {
		arg := strings.TrimSpace(args[1])
		if arg != "" {
			if newName != "" && newName != arg {
				return "", "", usageError(cmd, "new name specified twice")
			}
			if newName == "" {
				newName = arg
			}
		}
	}
	if currentName == "" {
		return "", "", usageError(cmd, "current name is required")
	}
	if newName == "" {
		return "", "", usageError(cmd, "new name is required")
	}
	return currentName, newName, nil
}

func configStoreLabel(manager *ctx.DefaultContextManager) string {
	path := ""
	if manager != nil {
		path = strings.TrimSpace(manager.ConfigFilePath)
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".declarest", "config")
		}
	}
	if path == "" {
		return "the local config store"
	}
	return fmt.Sprintf("config store %s", path)
}

func contextExists(manager *ctx.DefaultContextManager, name string) (bool, error) {
	if manager == nil {
		return false, errors.New("context manager is not configured")
	}
	if strings.TrimSpace(name) == "" {
		return false, errors.New("context name is required")
	}
	contexts, err := manager.ListContexts()
	if err != nil {
		return false, err
	}
	for _, existing := range contexts {
		if existing == name {
			return true, nil
		}
	}
	return false, nil
}
