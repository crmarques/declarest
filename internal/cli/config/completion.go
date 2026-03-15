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
	"context"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

func registerSingleContextArgCompletion(command *cobra.Command, deps cliutil.CommandDependencies) {
	command.ValidArgsFunction = func(
		_ *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContextNames(deps, toComplete)
	}
}

func registerRenameFromArgCompletion(command *cobra.Command, deps cliutil.CommandDependencies) {
	command.ValidArgsFunction = func(
		_ *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContextNames(deps, toComplete)
	}
}

func completeContextNames(deps cliutil.CommandDependencies, toComplete string) ([]string, cobra.ShellCompDirective) {
	service, err := cliutil.RequireContexts(deps)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	items, err := service.List(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return cliutil.CompleteValues(names, toComplete)
}
