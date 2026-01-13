package cmd

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"declarest/internal/reconciler"

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
	cmd.AddCommand(newSecretExportCommand())
	cmd.AddCommand(newSecretImportCommand())
	cmd.AddCommand(newSecretCheckCommand())

	return cmd
}

func newSecretInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the configured secret store",
		RunE: func(cmd *cobra.Command, args []string) error {
			recon, cleanup, err := loadDefaultReconcilerForSecrets()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return wrapSecretStoreError(err)
			}

			if err := recon.EnsureSecretsFile(); err != nil {
				return wrapSecretStoreError(err)
			}

			successf(cmd, "initialized secret store")
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
				return wrapSecretStoreError(err)
			}

			value, err := recon.GetSecret(resourcePath, key)
			if err != nil {
				return wrapSecretStoreError(err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to read secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to read")

	registerSecretPathAndKeyCompletion(cmd)

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
				return wrapSecretStoreError(err)
			}

			if err := recon.SetSecret(resourcePath, key, value); err != nil {
				return wrapSecretStoreError(err)
			}
			successf(cmd, "stored secret %s for %s", key, resourcePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to store secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to store")
	cmd.Flags().StringVar(&value, "value", "", "Secret value to store")

	registerSecretPathAndKeyCompletion(cmd)

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

			message := fmt.Sprintf("Delete secret %s for %s from the configured secret store. Continue?", key, resourcePath)
			if err := confirmAction(cmd, yes, message); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return wrapSecretStoreError(err)
			}

			if err := recon.DeleteSecret(resourcePath, key); err != nil {
				return wrapSecretStoreError(err)
			}
			successf(cmd, "deleted secret %s for %s", key, resourcePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to delete secrets for")
	cmd.Flags().StringVar(&key, "key", "", "Secret key to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	registerSecretPathAndKeyCompletion(cmd)

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
				return wrapSecretStoreError(err)
			}

			if resourcePath != "" {
				if err := validateLogicalPath(cmd, resourcePath); err != nil {
					return err
				}
				keys, err := recon.ListSecretKeys(resourcePath)
				if err != nil {
					return wrapSecretStoreError(err)
				}
				if len(keys) == 0 {
					return nil
				}
				sort.Strings(keys)
				if pathsOnly {
					infof(cmd, "%s", resourcePath)
					return nil
				}
				return wrapSecretStoreError(writeSecretKeys(cmd, resourcePath, keys, showSecrets, recon.GetSecret))
			}

			paths, err := recon.ListSecretResources()
			if err != nil {
				return wrapSecretStoreError(err)
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
					return wrapSecretStoreError(err)
				}
				if len(keys) == 0 {
					continue
				}
				sort.Strings(keys)
				if err := writeSecretKeys(cmd, path, keys, showSecrets, recon.GetSecret); err != nil {
					return wrapSecretStoreError(err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to list secret keys for")
	cmd.Flags().BoolVar(&pathsOnly, "paths-only", false, "List only paths that have stored secrets")
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Include secret values in output")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

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

type secretCSVRow struct {
	Path  string
	Key   string
	Value string
	Row   int
}

func newSecretExportCommand() *cobra.Command {
	var (
		resourcePath string
		exportAll    bool
	)

	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Export stored secrets to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			resourcePath, err = resolveOptionalArg(cmd, resourcePath, args, "path")
			if err != nil {
				return err
			}
			if exportAll && resourcePath != "" {
				return usageError(cmd, "--all cannot be combined with --path")
			}
			if !exportAll && resourcePath == "" {
				return usageError(cmd, "either --path or --all is required")
			}
			if resourcePath != "" {
				if err := validateLogicalPath(cmd, resourcePath); err != nil {
					return err
				}
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return wrapSecretStoreError(err)
			}

			rows, err := secretExportRows(recon, exportAll, resourcePath)
			if err != nil {
				return wrapSecretStoreError(err)
			}

			if err := writeSecretCSV(cmd.OutOrStdout(), rows); err != nil {
				return wrapSecretStoreError(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "path", "", "Resource path to export secrets for")
	cmd.Flags().BoolVar(&exportAll, "all", false, "Export secrets for all resources")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newSecretImportCommand() *cobra.Command {
	var (
		filePath string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import secrets from a CSV file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			filePath, err = resolveOptionalArg(cmd, filePath, args, "file")
			if err != nil {
				return err
			}
			if strings.TrimSpace(filePath) == "" {
				return usageError(cmd, "file path is required (use --file or positional argument)")
			}

			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open %s: %w", filePath, err)
			}
			defer f.Close()

			rows, err := readSecretCSV(f)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return wrapSecretStoreError(err)
			}

			for _, row := range rows {
				if err := validateLogicalPath(cmd, row.Path); err != nil {
					return err
				}
			}

			conflicts, err := secretImportConflicts(recon, rows)
			if err != nil {
				return wrapSecretStoreError(err)
			}
			if len(conflicts) > 0 && !force {
				fmt.Fprintln(cmd.ErrOrStderr(), "Import would overwrite existing secrets; use --force to continue.")
				for _, conflict := range conflicts {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", conflict)
				}
				return handledError{msg: "import stopped due to conflicts"}
			}

			for _, row := range rows {
				if err := recon.SetSecret(row.Path, row.Key, row.Value); err != nil {
					return wrapSecretStoreError(err)
				}
			}

			successf(cmd, "imported %d secret(s)", len(rows))
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "CSV file containing secrets to import")
	cmd.Flags().BoolVar(&force, "force", false, "Allow overwriting existing secrets")

	return cmd
}

func secretExportRows(recon *reconciler.DefaultReconciler, exportAll bool, resourcePath string) ([]secretCSVRow, error) {
	var paths []string
	if exportAll {
		p, err := recon.ListSecretResources()
		if err != nil {
			return nil, err
		}
		paths = p
	} else {
		paths = []string{resourcePath}
	}

	var rows []secretCSVRow
	for _, path := range paths {
		keys, err := recon.ListSecretKeys(path)
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		for _, key := range keys {
			value, err := recon.GetSecret(path, key)
			if err != nil {
				return nil, fmt.Errorf("read secret %s for %s: %w", key, path, err)
			}
			rows = append(rows, secretCSVRow{
				Path:  path,
				Key:   key,
				Value: value,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path == rows[j].Path {
			return rows[i].Key < rows[j].Key
		}
		return rows[i].Path < rows[j].Path
	})
	return rows, nil
}

func writeSecretCSV(out io.Writer, rows []secretCSVRow) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"path", "key", "value"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write([]string{row.Path, row.Key, row.Value}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func readSecretCSV(r io.Reader) ([]secretCSVRow, error) {
	reader := csv.NewReader(r)
	line := 0
	headerSkipped := false
	var rows []secretCSVRow
	for {
		line++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV: %w", err)
		}
		if len(record) == 0 {
			continue
		}
		if !headerSkipped && isSecretCSVHeader(record) {
			headerSkipped = true
			continue
		}
		if len(record) < 3 {
			return nil, fmt.Errorf("row %d: expected at least 3 columns", line)
		}
		path := strings.TrimSpace(record[0])
		key := strings.TrimSpace(record[1])
		if path == "" {
			return nil, fmt.Errorf("row %d: path is required", line)
		}
		if key == "" {
			return nil, fmt.Errorf("row %d: key is required", line)
		}
		rows = append(rows, secretCSVRow{
			Path:  path,
			Key:   key,
			Value: record[2],
			Row:   line,
		})
	}
	return rows, nil
}

func isSecretCSVHeader(record []string) bool {
	if len(record) < 3 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(record[0]), "path") &&
		strings.EqualFold(strings.TrimSpace(record[1]), "key") &&
		strings.EqualFold(strings.TrimSpace(record[2]), "value")
}

func secretImportConflicts(recon *reconciler.DefaultReconciler, rows []secretCSVRow) ([]string, error) {
	seen := map[string]map[string]struct{}{}
	var conflicts []string
	for _, row := range rows {
		if seen[row.Path] == nil {
			seen[row.Path] = map[string]struct{}{}
		}
		if _, ok := seen[row.Path][row.Key]; ok {
			continue
		}
		seen[row.Path][row.Key] = struct{}{}

		_, err := recon.GetSecret(row.Path, row.Key)
		if err == nil {
			conflicts = append(conflicts, fmt.Sprintf("%s:%s", row.Path, row.Key))
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("check existing secret %s:%s: %w", row.Path, row.Key, err)
		}
	}
	sort.Strings(conflicts)
	return conflicts, nil
}
