package config

import (
	"bytes"
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func newEditCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var editor string

	command := &cobra.Command{
		Use:   "edit [name]",
		Short: "Edit the context catalog or one context in an editor",
		Long: strings.Join([]string{
			"Edit the full contexts catalog by default.",
			"When a context name (or global --context) is provided, only that context object is opened for editing.",
			"Changes are validated and persisted only after the editor exits successfully.",
		}, " "),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			editorService, ok := contexts.(configdomain.ContextCatalogEditor)
			if !ok {
				return common.ValidationError("config edit requires a file-backed context catalog editor service", nil)
			}

			targetName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			catalog, err := editorService.GetCatalog(command.Context())
			if err != nil {
				return err
			}

			resolvedEditor := common.ResolveEditorCommand(command.Context(), deps, editor)
			if strings.TrimSpace(targetName) == "" {
				return editContextCatalog(command, editorService, resolvedEditor, catalog)
			}
			return editSingleContext(command, editorService, resolvedEditor, catalog, targetName)
		},
	}

	common.BindEditorFlag(command, &editor)
	registerSingleContextArgCompletion(command, deps)
	return command
}

func editContextCatalog(
	command *cobra.Command,
	editorService configdomain.ContextCatalogEditor,
	editor string,
	catalog configdomain.ContextCatalog,
) error {
	encoded, err := yaml.Marshal(catalog)
	if err != nil {
		return common.ValidationError("failed to encode context catalog for editing", err)
	}

	edited, err := common.EditTempFile(command, editor, "contexts.yaml", encoded)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(edited)) == 0 {
		return common.ValidationError("context catalog edit is empty", nil)
	}

	decoded, err := decodeContextCatalogStrictFromData(edited, common.OutputYAML)
	if err != nil {
		return err
	}

	return editorService.ReplaceCatalog(command.Context(), decoded)
}

func editSingleContext(
	command *cobra.Command,
	editorService configdomain.ContextCatalogEditor,
	editor string,
	catalog configdomain.ContextCatalog,
	name string,
) error {
	idx := -1
	for i, item := range catalog.Contexts {
		if item.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("context %q not found", name), nil)
	}

	encoded, err := yaml.Marshal(catalog.Contexts[idx])
	if err != nil {
		return common.ValidationError("failed to encode context for editing", err)
	}

	edited, err := common.EditTempFile(command, editor, fmt.Sprintf("%s.yaml", name), encoded)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(edited)) == 0 {
		return common.ValidationError("context edit is empty", nil)
	}

	decoded, err := decodeContextStrictFromData(edited, common.OutputYAML)
	if err != nil {
		return err
	}

	oldName := catalog.Contexts[idx].Name
	catalog.Contexts[idx] = decoded
	if catalog.CurrentCtx == oldName && decoded.Name != "" {
		catalog.CurrentCtx = decoded.Name
	}

	return editorService.ReplaceCatalog(command.Context(), catalog)
}
