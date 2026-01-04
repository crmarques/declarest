package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newSecretCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		GroupID: groupUserFacing,
		Short:   "Manage secrets stored in the configured secret store",
	}

	cmd.AddCommand(newSecretInitCommand())
	cmd.AddCommand(newSecretGetCommand())
	cmd.AddCommand(newSecretAddCommand())
	cmd.AddCommand(newSecretDeleteCommand())
	cmd.AddCommand(newSecretListCommand())
	cmd.AddCommand(newSecretCheckCommand())

	return cmd
}

func newSecretInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialise the configured secret store",
		RunE: func(cmd *cobra.Command, args []string) error {
			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if err := recon.EnsureSecretsFile(); err != nil {
				return err
			}

			successf(cmd, "initialised secret store")
			return nil
		},
	}
}

func newSecretGetCommand() *cobra.Command {
	var (
		resourcePath string
		key          string
	)

	cmd := &cobra.Command{
		Use:   "get <path> <key>",
		Short: "Fetch a secret value from the secret store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageError(cmd, "expected <path> <key>")
			}
			resourcePath = strings.TrimSpace(resourcePath)
			key = strings.TrimSpace(key)
			if len(args) > 0 {
				argPath := strings.TrimSpace(args[0])
				if argPath != "" {
					if resourcePath != "" && resourcePath != argPath {
						return usageError(cmd, "path specified twice")
					}
					if resourcePath == "" {
						resourcePath = argPath
					}
				}
			}
			if len(args) > 1 {
				argKey := strings.TrimSpace(args[1])
				if argKey != "" {
					if key != "" && key != argKey {
						return usageError(cmd, "key specified twice")
					}
					if key == "" {
						key = argKey
					}
				}
			}
			if resourcePath == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, resourcePath); err != nil {
				return err
			}

			if key == "" {
				return usageError(cmd, "key is required")
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			value, err := recon.GetSecret(resourcePath, key)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to read secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to read")

	return cmd
}

func newSecretAddCommand() *cobra.Command {
	var (
		resourcePath string
		key          string
		value        string
	)

	cmd := &cobra.Command{
		Use:   "add <path> <key> <value>",
		Short: "Add or update a secret value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 3 {
				return usageError(cmd, "expected <path> <key> <value>")
			}
			resourcePath = strings.TrimSpace(resourcePath)
			key = strings.TrimSpace(key)
			valueProvided := cmd.Flags().Changed("value")
			if len(args) > 0 {
				argPath := strings.TrimSpace(args[0])
				if argPath != "" {
					if resourcePath != "" && resourcePath != argPath {
						return usageError(cmd, "path specified twice")
					}
					if resourcePath == "" {
						resourcePath = argPath
					}
				}
			}
			if len(args) > 1 {
				argKey := strings.TrimSpace(args[1])
				if argKey != "" {
					if key != "" && key != argKey {
						return usageError(cmd, "key specified twice")
					}
					if key == "" {
						key = argKey
					}
				}
			}
			if len(args) > 2 {
				argValue := args[2]
				if valueProvided && value != argValue {
					return usageError(cmd, "value specified twice")
				}
				if !valueProvided {
					value = argValue
					valueProvided = true
				}
			}
			if resourcePath == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, resourcePath); err != nil {
				return err
			}

			if key == "" {
				return usageError(cmd, "key is required")
			}
			if !valueProvided {
				return usageError(cmd, "value is required")
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if err := recon.SetSecret(resourcePath, key, value); err != nil {
				return err
			}
			successf(cmd, "stored secret %s for %s", key, resourcePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to store secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to store")
	cmd.Flags().StringVar(&value, "value", "", "Secret value to store")

	return cmd
}

func newSecretDeleteCommand() *cobra.Command {
	var (
		resourcePath string
		key          string
		yes          bool
	)

	cmd := &cobra.Command{
		Use:   "delete <path> <key>",
		Short: "Delete a stored secret value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageError(cmd, "expected <path> <key>")
			}
			resourcePath = strings.TrimSpace(resourcePath)
			key = strings.TrimSpace(key)
			if len(args) > 0 {
				argPath := strings.TrimSpace(args[0])
				if argPath != "" {
					if resourcePath != "" && resourcePath != argPath {
						return usageError(cmd, "path specified twice")
					}
					if resourcePath == "" {
						resourcePath = argPath
					}
				}
			}
			if len(args) > 1 {
				argKey := strings.TrimSpace(args[1])
				if argKey != "" {
					if key != "" && key != argKey {
						return usageError(cmd, "key specified twice")
					}
					if key == "" {
						key = argKey
					}
				}
			}
			if resourcePath == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, resourcePath); err != nil {
				return err
			}

			if key == "" {
				return usageError(cmd, "key is required")
			}

			message := fmt.Sprintf("Delete secret %s for %s from the configured secret store. %s Continue?", key, resourcePath, impactSummary(false, false))
			if err := confirmAction(cmd, yes, message); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if err := recon.DeleteSecret(resourcePath, key); err != nil {
				return err
			}
			successf(cmd, "deleted secret %s for %s", key, resourcePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to delete secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&yes, "force", false, "DEPRECATED: use --yes")
	_ = cmd.Flags().MarkHidden("force")

	return cmd
}

func newSecretListCommand() *cobra.Command {
	var (
		resourcePath string
		pathsOnly    bool
		showSecrets  bool
	)

	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List stored secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			resourcePath, err = resolveOptionalArg(cmd, resourcePath, args, "path")
			if err != nil {
				return err
			}
			if pathsOnly && showSecrets {
				return usageError(cmd, "--paths-only cannot be combined with --show-secrets")
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if resourcePath != "" {
				if err := validateLogicalPath(cmd, resourcePath); err != nil {
					return err
				}
				keys, err := recon.ListSecretKeys(resourcePath)
				if err != nil {
					return err
				}
				if len(keys) == 0 {
					return nil
				}
				sort.Strings(keys)
				if pathsOnly {
					infof(cmd, "%s", resourcePath)
					return nil
				}
				return writeSecretKeys(cmd, resourcePath, keys, showSecrets, recon.GetSecret)
			}

			paths, err := recon.ListSecretResources()
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return nil
			}
			sort.Strings(paths)
			if pathsOnly {
				for _, path := range paths {
					infof(cmd, "%s", path)
				}
				return nil
			}
			for _, path := range paths {
				keys, err := recon.ListSecretKeys(path)
				if err != nil {
					return err
				}
				if len(keys) == 0 {
					continue
				}
				sort.Strings(keys)
				if err := writeSecretKeys(cmd, path, keys, showSecrets, recon.GetSecret); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to list secret keys for")
	cmd.Flags().BoolVar(&pathsOnly, "paths-only", false, "List only paths that have stored secrets")
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Include secret values in output")

	return cmd
}

func writeSecretKeys(cmd *cobra.Command, resourcePath string, keys []string, showSecrets bool, getSecret func(string, string) (string, error)) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s:\n", resourcePath)
	if showSecrets {
		for _, key := range keys {
			value, err := getSecret(resourcePath, key)
			if err != nil {
				return fmt.Errorf("read secret %s for %s: %w", key, resourcePath, err)
			}
			fmt.Fprintf(out, "  %s:%s\n", key, value)
		}
		return nil
	}
	for _, key := range keys {
		fmt.Fprintf(out, "  %s\n", key)
	}
	return nil
}
