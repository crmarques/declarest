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

package resource

import (
	"fmt"
	"io"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newListCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var excludeItemsFlag []string
	var recursive bool
	var httpMethod string

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List resources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			source, err := normalizeReadSourceSelection(sourceFlag)
			if err != nil {
				return err
			}
			excludeItems, err := parseExcludeFlag(command, excludeItemsFlag)
			if err != nil {
				return err
			}
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && source == sourceRepository {
				return cliutil.ValidationError("flag --http-method requires managed-server source", nil)
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if globalFlags != nil && globalFlags.Output == cliutil.OutputAuto {
				outputFormat = cliutil.OutputAuto
			}

			orchestratorService, err := cliutil.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			runCtx := command.Context()
			if source == sourceManagedServer {
				runCtx, _, err = applyHTTPMethodOverride(runCtx, httpMethod, metadata.OperationList)
				if err != nil {
					return err
				}
			}

			var items []resource.Resource
			switch source {
			case sourceRepository:
				items, err = orchestratorService.ListLocal(runCtx, resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			case sourceManagedServer:
				fallthrough
			default:
				items, err = orchestratorService.ListRemote(runCtx, resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			}
			if err != nil {
				return err
			}
			items = resource.FilterCollectionItems(resolvedPath, items, excludeItems)

			payloads := make([]resource.Value, 0, len(items))
			for _, item := range items {
				payloads = append(payloads, item.Payload)
			}

			return cliutil.WriteOutput(command, outputFormat, payloads, func(w io.Writer, _ []resource.Value) error {
				return renderListText(w, items)
			})
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	bindReadSourceFlags(command, &sourceFlag)
	bindExcludeFlag(command, &excludeItemsFlag)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "list recursively")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}

func renderListText(w io.Writer, items []resource.Resource) error {
	type listEntry struct {
		alias    string
		remoteID string
	}

	entries := make([]listEntry, 0, len(items))
	maxAliasWidth := 0
	for _, item := range items {
		alias := strings.TrimSpace(item.LocalAlias)
		remoteID := strings.TrimSpace(item.RemoteID)
		if alias == "" {
			alias = strings.TrimSpace(item.LogicalPath)
		}
		if remoteID == "" {
			remoteID = alias
		}
		entries = append(entries, listEntry{alias: alias, remoteID: remoteID})
		if len(alias) > maxAliasWidth {
			maxAliasWidth = len(alias)
		}
	}

	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "%-*s (%s)\n", maxAliasWidth, entry.alias, entry.remoteID); err != nil {
			return err
		}
	}
	return nil
}
