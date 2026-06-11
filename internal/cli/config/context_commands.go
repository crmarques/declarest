// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

type addContextSelection struct {
	Contexts       []configdomain.Context
	CurrentContext string
}

func newAddCommand(
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	var input cliutil.InputFlags
	var contextName string
	var setCurrent bool

	command := &cobra.Command{
		Use:   "add [new-context-name]",
		Short: "Add contexts from input or create one interactively",
		Example: strings.Join([]string{
			"  declarest context add --payload context.yaml",
			"  declarest context add --payload contexts.yaml --context-name prod",
			"  cat contexts.yaml | declarest context add --set-current",
			"  declarest context add dev",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			contextArgName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			effectiveImportContextName := strings.TrimSpace(contextName)
			if effectiveImportContextName != "" && contextArgName != "" && effectiveImportContextName != contextArgName {
				return cliutil.ValidationError(
					fmt.Sprintf(
						"context name conflict: positional/--context %q differs from --context-name %q",
						contextArgName,
						effectiveImportContextName,
					),
					nil,
				)
			}
			if effectiveImportContextName == "" {
				effectiveImportContextName = contextArgName
			}

			if shouldUseInteractiveCreate(command, input, prompter) {
				cfg, err := resolveCreateContextInput(command, input, prompter, effectiveImportContextName)
				if err != nil {
					return err
				}
				if err := contexts.Create(command.Context(), cfg); err != nil {
					return err
				}
				if setCurrent {
					return contexts.SetCurrent(command.Context(), cfg.Name)
				}
				return nil
			}

			decoded, err := decodeContextImportInputStrict(command, input)
			if err != nil {
				return err
			}

			selection, err := selectContextsForAdd(decoded, effectiveImportContextName)
			if err != nil {
				return err
			}

			currentName := ""
			if setCurrent {
				currentName, err = resolveSetCurrentContext(selection)
				if err != nil {
					return err
				}
			}

			if err := validateAddTargets(command, contexts, selection.Contexts); err != nil {
				return err
			}

			for _, cfg := range selection.Contexts {
				if err := contexts.Create(command.Context(), cfg); err != nil {
					return err
				}
			}

			if !setCurrent {
				return nil
			}

			return contexts.SetCurrent(command.Context(), currentName)
		},
	}

	cliutil.BindInputFlags(command, &input)
	command.Flags().StringVar(&contextName, "context-name", "", "context name to import (catalog) or assign (single context)")
	command.Flags().BoolVar(&setCurrent, "set-current", false, "set imported context as current")
	return command
}

func selectContextsForAdd(input contextImportInput, contextName string) (addContextSelection, error) {
	trimmedContextName := strings.TrimSpace(contextName)
	switch input.Kind {
	case contextImportInputContext:
		cfg := input.Context
		if trimmedContextName != "" {
			cfg.Name = trimmedContextName
		}
		return addContextSelection{
			Contexts: []configdomain.Context{cfg},
		}, nil
	case contextImportInputCatalog:
		if len(input.Catalog.Contexts) == 0 {
			return addContextSelection{}, cliutil.ValidationError("input context catalog has no contexts", nil)
		}

		if trimmedContextName == "" {
			contexts := make([]configdomain.Context, len(input.Catalog.Contexts))
			copy(contexts, input.Catalog.Contexts)
			return addContextSelection{
				Contexts:       contexts,
				CurrentContext: strings.TrimSpace(input.Catalog.CurrentContext),
			}, nil
		}

		for _, item := range input.Catalog.Contexts {
			if item.Name == trimmedContextName {
				return addContextSelection{
					Contexts: []configdomain.Context{item},
				}, nil
			}
		}

		return addContextSelection{}, cliutil.ValidationError(
			fmt.Sprintf("context %q not found in input catalog", trimmedContextName),
			nil,
		)
	default:
		return addContextSelection{}, cliutil.ValidationError("unsupported config input shape", nil)
	}
}

func resolveSetCurrentContext(selection addContextSelection) (string, error) {
	if len(selection.Contexts) == 1 {
		return selection.Contexts[0].Name, nil
	}

	if selection.CurrentContext != "" {
		for _, item := range selection.Contexts {
			if item.Name == selection.CurrentContext {
				return selection.CurrentContext, nil
			}
		}
		return "", cliutil.ValidationError(
			fmt.Sprintf("input currentContext %q is not present in imported contexts", selection.CurrentContext),
			nil,
		)
	}

	return "", cliutil.ValidationError(
		"set-current requires a single imported context or a catalog currentContext value",
		nil,
	)
}

func resolveCreateContextName(args []string, contextFlagName string) (string, error) {
	positionalName := ""
	if len(args) > 0 {
		positionalName = strings.TrimSpace(args[0])
	}

	flagName := strings.TrimSpace(contextFlagName)
	if positionalName != "" && flagName != "" && positionalName != flagName {
		return "", cliutil.ValidationError(
			fmt.Sprintf("context name conflict: positional %q differs from --context %q", positionalName, flagName),
			nil,
		)
	}

	if positionalName != "" {
		return positionalName, nil
	}
	return flagName, nil
}

func validateAddTargets(command *cobra.Command, contexts configdomain.ContextService, items []configdomain.Context) error {
	if len(items) == 0 {
		return cliutil.ValidationError("no contexts found in input", nil)
	}

	existing, err := contexts.List(command.Context())
	if err != nil {
		return err
	}

	existingNames := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		existingNames[item.Name] = struct{}{}
	}

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return cliutil.ValidationError("context name is required", nil)
		}
		if _, duplicated := seen[name]; duplicated {
			return cliutil.ValidationError(fmt.Sprintf("input contains duplicate context %q", name), nil)
		}
		if _, exists := existingNames[name]; exists {
			return cliutil.ValidationError(fmt.Sprintf("context %q already exists", name), nil)
		}
		seen[name] = struct{}{}
	}

	return nil
}

func newUpdateCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "update",
		Short: "Update a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := cliutil.RequireContexts(deps)
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

	cliutil.BindInputFlags(command, &input)
	return command
}

func newDeleteCommand(deps cliutil.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}

			var name string
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
					return cliutil.WriteText(command, cliutil.OutputText, "delete canceled")
				}
				name = selected
			}
			return contexts.Delete(command.Context(), name)
		},
	}
	registerSingleContextArgCompletion(command, deps)
	return command
}

func newRenameCommand(deps cliutil.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "rename [from] [to]",
		Short: "Rename a context (interactive when args are omitted)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}

			var fromName, toName string
			switch len(args) {
			case 2:
				fromName = args[0]
				toName = args[1]
			case 1:
				fromName = args[0]
				if !prompter.IsInteractive(command) {
					return cliutil.ValidationError("new context name is required", nil)
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
	registerRenameFromArgCompletion(command, deps)
	return command
}

func newListCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List contexts",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			items, err := contexts.List(command.Context())
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, globalFlags.Output, items, func(w io.Writer, value []configdomain.Context) error {
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

func newUseCommand(deps cliutil.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "use [name]",
		Short: "Set current context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}

			var name string
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
	registerSingleContextArgCompletion(command, deps)
	return command
}

func newShowCommand(
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "show [name]",
		Short: "Show a context from --context or interactive selection",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}

			name, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}
			if name == "" {
				name, err = selectContextForAction(command, contexts, prompter, "show --context")
				if err != nil {
					return err
				}
			}

			if editorService, ok := contexts.(configdomain.ContextCatalogEditor); ok {
				catalog, err := editorService.GetCatalog(command.Context())
				if err != nil {
					return err
				}

				shown, err := selectContextCatalogForShow(catalog, name)
				if err != nil {
					return err
				}

				return cliutil.WriteOutput(
					command,
					cliutil.ResolveCommandOutputFormat(command, globalFlags),
					shown,
					renderContextCatalogText,
				)
			}

			items, err := contexts.List(command.Context())
			if err != nil {
				return err
			}

			shown, _, err := selectContextForView(items, name)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(
				command,
				cliutil.ResolveCommandOutputFormat(command, globalFlags),
				shown,
				renderContextText,
			)
		},
	}

	registerSingleContextArgCompletion(command, deps)
	return command
}

func newCurrentCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Get current context",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			current, err := contexts.GetCurrent(command.Context())
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, globalFlags.Output, current, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}
}
