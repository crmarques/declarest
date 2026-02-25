package common

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/spf13/cobra"
)

const DefaultEditorCommand = "vi"

func BindEditorFlag(command *cobra.Command, editor *string) {
	command.Flags().StringVar(editor, "editor", "", "editor command override (default: context catalog default-editor or vi)")
}

func ResolveEditorCommand(ctx context.Context, deps CommandDependencies, explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}

	contexts, err := RequireContexts(deps)
	if err == nil {
		if editorService, ok := contexts.(configdomain.ContextCatalogEditor); ok {
			catalog, catalogErr := editorService.GetCatalog(ctx)
			if catalogErr == nil {
				if value := strings.TrimSpace(catalog.DefaultEditor); value != "" {
					return value
				}
			}
		}
	}

	return DefaultEditorCommand
}

func EditTempFile(command *cobra.Command, editorCommand string, filename string, initial []byte) ([]byte, error) {
	if !IsInteractiveTerminal(command) {
		return nil, ValidationError("interactive editor requires a terminal", nil)
	}

	baseName := strings.TrimSpace(filename)
	if baseName == "" {
		baseName = "declarest-edit.tmp"
	}

	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "declarest-edit-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if strings.TrimSpace(filepath.Ext(baseName)) != "" {
		renamedPath := tmpPath + filepath.Ext(baseName)
		if renameErr := os.Rename(tmpPath, renamedPath); renameErr == nil {
			tmpPath = renamedPath
		}
	}

	if len(initial) > 0 {
		if err := os.WriteFile(tmpPath, initial, 0o600); err != nil {
			return nil, err
		}
	}

	if err := runEditor(command, editorCommand, tmpPath); err != nil {
		return nil, err
	}

	return os.ReadFile(tmpPath)
}

func runEditor(command *cobra.Command, editorCommand string, filePath string) error {
	trimmed := strings.TrimSpace(editorCommand)
	if trimmed == "" {
		trimmed = DefaultEditorCommand
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ValidationError("editor command is empty", nil)
	}

	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = command.InOrStdin()
	cmd.Stdout = command.OutOrStdout()
	cmd.Stderr = command.ErrOrStderr()
	return cmd.Run()
}
