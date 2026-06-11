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
	"errors"
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	return newCommandWithPrompter(deps, globalFlags, terminalPrompter{})
}

func newCommandWithPrompter(
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "context",
		Short: "Manage contexts",
		Args:  cobra.NoArgs,
	}

	printTemplateCommand := newPrintTemplateCommand()
	migrateCommand := newMigrateCommand(deps)
	initCommand := newInitCommand(deps, globalFlags)
	addCommand := newAddCommand(deps, globalFlags, prompter)
	editCommand := newEditCommand(deps, globalFlags)
	updateCommand := newUpdateCommand(deps)
	deleteCommand := newDeleteCommand(deps, prompter)
	renameCommand := newRenameCommand(deps, prompter)
	listCommand := newListCommand(deps, globalFlags)
	useCommand := newUseCommand(deps, prompter)
	showCommand := newShowCommand(deps, globalFlags, prompter)
	currentCommand := newCurrentCommand(deps, globalFlags)
	cleanCommand := newCleanCommand()
	sessionHookCommand := newSessionHookCommand()
	resolveCommand := newResolveCommand(deps, globalFlags)
	checkCommand := newCheckCommand(deps, globalFlags)
	validateCommand := newValidateCommand(deps)

	commandmeta.MarkTextOnlyOutput(printTemplateCommand)
	commandmeta.MarkTextOnlyOutput(cleanCommand)
	commandmeta.MarkTextOnlyOutput(sessionHookCommand)
	commandmeta.MarkRequiresContextBootstrap(initCommand)
	commandmeta.MarkYAMLDefaultTextOrYAMLOutput(showCommand)
	commandmeta.MarkRequiresContextBootstrap(checkCommand)
	commandmeta.MarkTextDefaultStructuredOutput(checkCommand)

	command.AddCommand(
		printTemplateCommand,
		migrateCommand,
		initCommand,
		addCommand,
		editCommand,
		updateCommand,
		deleteCommand,
		renameCommand,
		listCommand,
		useCommand,
		showCommand,
		currentCommand,
		cleanCommand,
		sessionHookCommand,
		resolveCommand,
		checkCommand,
		validateCommand,
	)

	return command
}

func renderContextText(writer io.Writer, value configdomain.Context) error {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(writer, string(encoded))
	return err
}

func renderContextCatalogText(writer io.Writer, value configdomain.ContextCatalog) error {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(writer, string(encoded))
	return err
}

func selectedContextName(globalFlags *cliutil.GlobalFlags) string {
	if globalFlags == nil {
		return ""
	}
	return strings.TrimSpace(globalFlags.Context)
}

func typedCategory(err error) faults.ErrorCategory {
	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return ""
	}
	return typedErr.Category
}

func parseOverrides(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, cliutil.ValidationError("invalid override: expected key=value", nil)
		}
		parsed[strings.TrimSpace(parts[0])] = parts[1]
	}

	return parsed, nil
}
