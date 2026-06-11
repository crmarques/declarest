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

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/promptauth"
	"github.com/spf13/cobra"
)

func newPrintTemplateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "print-template",
		Short: "Print a full context YAML template with guidance comments",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := io.WriteString(command.OutOrStdout(), contextTemplateYAML)
			return err
		},
	}
}

func newMigrateCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Rewrite the context catalog using the canonical format",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			editorService, ok := contexts.(configdomain.ContextCatalogEditor)
			if !ok {
				return cliutil.ValidationError("context migrate requires a file-backed context catalog editor service", nil)
			}

			catalog, err := editorService.GetCatalog(command.Context())
			if err != nil {
				return err
			}
			if err := editorService.ReplaceCatalog(command.Context(), catalog); err != nil {
				return err
			}
			return cliutil.WriteText(command, cliutil.OutputText, "context catalog migrated")
		},
	}
}

func newInitCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize repository and metadata dependencies",
		Example: strings.Join([]string{
			"  declarest context init",
			"  declarest context init prod",
			"  declarest context init --context prod",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contextName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			if _, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{Name: contextName}); err != nil {
				return err
			}

			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			if err := repositoryService.Init(command.Context()); err != nil {
				return err
			}

			metadataService, err := cliutil.RequireMetadataService(deps)
			if err != nil {
				return err
			}
			_, err = metadataService.ResolveForPath(command.Context(), "/")
			return err
		},
	}

	registerSingleContextArgCompletion(command, deps)
	return command
}

func newCleanCommand() *cobra.Command {
	var credentialsInSession bool

	command := &cobra.Command{
		Use:   "clean",
		Short: "Clean cached context session data",
		Example: strings.Join([]string{
			"  declarest context clean --credentials-in-session",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !credentialsInSession {
				return cliutil.ValidationError("at least one clean target flag is required", nil)
			}

			removed := 0
			if credentialsInSession {
				cleared, err := promptauth.ClearSessionCredentials()
				if err != nil {
					return err
				}
				removed += cleared
			}

			return cliutil.WriteText(
				command,
				cliutil.OutputText,
				fmt.Sprintf("removed %d prompt credential session cache files", removed),
			)
		},
	}

	command.Flags().BoolVar(
		&credentialsInSession,
		"credentials-in-session",
		false,
		"remove prompt-backed credential session cache files",
	)

	return command
}

func newSessionHookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "session-hook <bash|zsh>",
		Short: "Print shell hook code for prompt credential session cleanup",
		Args:  cobra.ExactArgs(1),
		Example: strings.Join([]string{
			`  eval "$(declarest context session-hook bash)"`,
			`  eval "$(declarest context session-hook zsh)"`,
		}, "\n"),
		RunE: func(command *cobra.Command, args []string) error {
			hook, err := promptauth.RenderSessionHook(args[0])
			if err != nil {
				return err
			}
			return cliutil.WriteText(command, cliutil.OutputText, strings.TrimRight(hook, "\n"))
		},
	}
}

func newResolveCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var overrides []string

	command := &cobra.Command{
		Use:   "resolve [name]",
		Short: "Resolve active context with overrides",
		Example: strings.Join([]string{
			"  declarest context resolve",
			"  declarest context resolve prod",
			"  declarest context resolve --context prod",
			"  declarest context resolve --set managedService.http.url=https://api.example.com",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			contextName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			overridesMap, err := parseOverrides(overrides)
			if err != nil {
				return err
			}

			resolved, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{
				Name:      contextName,
				Overrides: overridesMap,
			})
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, globalFlags.Output, resolved, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}

	command.Flags().StringArrayVarP(&overrides, "set", "s", nil, "override key=value (repeatable)")
	registerSingleContextArgCompletion(command, deps)
	return command
}

func newValidateCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate a context from input",
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
			return contexts.Validate(command.Context(), cfg)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}
