package resource

import (
	"fmt"
	"io"
	"strings"

	"github.com/crmarques/declarest/internal/cli/shared"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	identitysupport "github.com/crmarques/declarest/resource/identity"
	"github.com/spf13/cobra"
)

func newListCommand(deps shared.CommandDependencies, globalFlags *shared.GlobalFlags) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var fromRepository bool
	var fromRemoteServer bool
	var recursive bool
	var httpMethod string

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List resources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := shared.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			source, err := normalizeReadSourceSelection(sourceFlag, fromRepository, fromRemoteServer)
			if err != nil {
				return err
			}
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && source == sourceRepository {
				return shared.ValidationError("flag --http-method requires remote-server source", nil)
			}

			outputFormat, err := shared.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if globalFlags != nil && globalFlags.Output == shared.OutputAuto {
				outputFormat = shared.OutputAuto
			}

			orchestratorService, err := shared.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			runCtx := command.Context()
			if source == sourceRemoteServer {
				runCtx, _, err = applyHTTPMethodOverride(runCtx, httpMethod, metadata.OperationList)
				if err != nil {
					return err
				}
			}

			var items []resource.Resource
			switch source {
			case sourceRepository:
				items, err = orchestratorService.ListLocal(runCtx, resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			case sourceRemoteServer:
				fallthrough
			default:
				items, err = orchestratorService.ListRemote(runCtx, resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			}
			if err != nil {
				return err
			}

			payloads := make([]resource.Value, 0, len(items))
			for _, item := range items {
				payloads = append(payloads, item.Payload)
			}

			return shared.WriteOutput(command, outputFormat, payloads, func(w io.Writer, _ []resource.Value) error {
				return renderListText(w, items)
			})
		},
	}

	shared.BindPathFlag(command, &pathFlag)
	shared.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = shared.SinglePathArgCompletionFunc(deps)
	bindReadSourceFlags(command, &sourceFlag, &fromRepository, &fromRemoteServer)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "list recursively")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}

func renderListText(w io.Writer, items []resource.Resource) error {
	for _, item := range items {
		alias := strings.TrimSpace(item.LocalAlias)
		remoteID := strings.TrimSpace(item.RemoteID)
		if alias == "" || remoteID == "" {
			resolvedAlias, resolvedRemoteID, err := identitysupport.ResolveAliasAndRemoteID(item.LogicalPath, item.Metadata, item.Payload)
			if err == nil {
				if alias == "" {
					alias = strings.TrimSpace(resolvedAlias)
				}
				if remoteID == "" {
					remoteID = strings.TrimSpace(resolvedRemoteID)
				}
			}
		}
		if alias == "" {
			alias = strings.TrimSpace(item.LogicalPath)
		}
		if remoteID == "" {
			remoteID = alias
		}
		if _, err := fmt.Fprintf(w, "%s (%s)\n", alias, remoteID); err != nil {
			return err
		}
	}
	return nil
}
