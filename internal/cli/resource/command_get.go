package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var fromRepository bool
	var fromRemoteServer bool
	var showSecrets bool

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			if fromRepository && fromRemoteServer {
				return common.ValidationError("flags --repository and --remote-server cannot be used together", nil)
			}

			source := sourceRemoteServer
			if fromRepository {
				source = sourceRepository
			} else if fromRemoteServer {
				source = sourceRemoteServer
			}

			debugctx.Printf(command.Context(), "resource get requested path=%q source=%q", resolvedPath, source)

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			var value resource.Value
			switch source {
			case sourceRepository:
				value, err = orchestratorService.GetLocal(command.Context(), resolvedPath)
			case sourceRemoteServer:
				value, err = orchestratorService.GetRemote(command.Context(), resolvedPath)
			default:
				return common.ValidationError("invalid source: use --repository or --remote-server", nil)
			}
			if err != nil {
				debugctx.Printf(command.Context(), "resource get failed path=%q source=%q error=%v", resolvedPath, source, err)
				if source == sourceRepository && (isNotFoundError(err) || isRootResourceError(err)) {
					debugctx.Printf(command.Context(), "resource get treating %q as collection listing", resolvedPath)
					return renderRepositoryCollection(command, outputFormat, deps, orchestratorService, resolvedPath, showSecrets)
				}
				return err
			}

			debugctx.Printf(command.Context(), "resource get succeeded path=%q value_type=%T source=%q", resolvedPath, value, source)
			if showSecrets {
				value, err = resolveGetSecretsForOutput(command.Context(), deps, resolvedPath, value)
				if err != nil {
					return err
				}
			} else {
				value, err = maskGetSecretsForOutput(command.Context(), deps, resolvedPath, value)
				if err != nil {
					return err
				}
			}

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&fromRepository, "repository", false, "read from repository")
	command.Flags().BoolVar(&fromRemoteServer, "remote-server", false, "read from remote server (default)")
	command.Flags().BoolVar(&showSecrets, "show-secrets", false, "show plaintext values for metadata-declared secret attributes")
	return command
}

func isNotFoundError(err error) bool {
	var typedErr *faults.TypedError
	if errors.As(err, &typedErr) {
		return typedErr.Category == faults.NotFoundError
	}
	return false
}

func isRootResourceError(err error) bool {
	var typedErr *faults.TypedError
	if errors.As(err, &typedErr) {
		return typedErr.Category == faults.ValidationError && typedErr.Message == "logical path must target a resource, not root"
	}
	return false
}

func renderRepositoryCollection(
	command *cobra.Command,
	outputFormat string,
	deps common.CommandDependencies,
	orchestratorService orchestrator.Orchestrator,
	logicalPath string,
	showSecrets bool,
) error {
	items, err := orchestratorService.ListLocal(command.Context(), logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return err
	}

	if !showSecrets {
		maskedItems := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			maskedPayload, maskErr := maskGetSecretsForOutput(command.Context(), deps, item.LogicalPath, item.Payload)
			if maskErr != nil {
				return maskErr
			}
			item.Payload = maskedPayload
			maskedItems = append(maskedItems, item)
		}
		items = maskedItems
	} else {
		resolvedItems := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			resolvedPayload, resolveErr := resolveGetSecretsForOutput(command.Context(), deps, item.LogicalPath, item.Payload)
			if resolveErr != nil {
				return resolveErr
			}
			item.Payload = resolvedPayload
			resolvedItems = append(resolvedItems, item)
		}
		items = resolvedItems
	}

	payloads := make([]resource.Value, len(items))
	for idx, item := range items {
		payloads[idx] = item.Payload
	}

	return common.WriteOutput(command, outputFormat, payloads, func(w io.Writer, _ []resource.Value) error {
		for _, item := range items {
			if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}

func maskGetSecretsForOutput(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	secretAttributes, err := resolveGetSecretAttributes(ctx, deps, logicalPath)
	if err != nil {
		return nil, err
	}
	if len(secretAttributes) == 0 {
		return value, nil
	}
	return maskGetSecretsInValue(value, secretAttributes)
}

func resolveGetSecretsForOutput(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	secretProvider, secretProviderErr := common.RequireSecretProvider(deps)
	if secretProviderErr != nil {
		return secretdomain.ResolvePayloadForResource(value, normalizedPath, func(string) (string, error) {
			return "", common.ValidationError(
				"flag --show-secrets requires a configured secret provider when payload includes placeholders",
				nil,
			)
		})
	}

	return secretdomain.ResolvePayloadForResource(value, normalizedPath, func(key string) (string, error) {
		return secretProvider.Get(ctx, key)
	})
}

func resolveGetSecretAttributes(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
) ([]string, error) {
	resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, logicalPath)
	if err != nil {
		return nil, err
	}
	return dedupeAndSortSaveSecretAttributes(resolvedMetadata.SecretsFromAttributes), nil
}

func maskGetSecretsInValue(value resource.Value, secretAttributes []string) (resource.Value, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	switch typed := normalizedValue.(type) {
	case map[string]any:
		maskGetSecretsInPayload(typed, secretAttributes)
		return typed, nil
	case []any:
		items := make([]any, len(typed))
		for idx := range typed {
			entry := typed[idx]
			entryPayload, ok := entry.(map[string]any)
			if !ok {
				items[idx] = entry
				continue
			}
			maskGetSecretsInPayload(entryPayload, secretAttributes)
			items[idx] = entryPayload
		}
		return items, nil
	default:
		return normalizedValue, nil
	}
}

func maskGetSecretsInPayload(payload map[string]any, secretAttributes []string) {
	paths := resolveGetSecretAttributePaths(payload, secretAttributes)
	for _, attributePath := range paths {
		parent, leafKey, found := findAttributeParentMap(payload, attributePath)
		if !found {
			continue
		}

		currentValue := parent[leafKey]
		if currentValue == nil {
			continue
		}
		if stringValue, ok := currentValue.(string); ok && isSecretPlaceholderValue(stringValue) {
			continue
		}
		parent[leafKey] = secretPlaceholderValue()
	}
}

func resolveGetSecretAttributePaths(payload map[string]any, secretAttributes []string) []string {
	resolvedPaths := make(map[string]struct{})
	for _, rawAttribute := range secretAttributes {
		attribute := strings.TrimSpace(rawAttribute)
		if attribute == "" {
			continue
		}
		if strings.Contains(attribute, ".") {
			if _, _, found := findAttributeParentMap(payload, attribute); found {
				resolvedPaths[attribute] = struct{}{}
			}
			continue
		}
		collectGetSecretAttributePaths(payload, "", attribute, resolvedPaths)
	}

	paths := make([]string, 0, len(resolvedPaths))
	for attributePath := range resolvedPaths {
		paths = append(paths, attributePath)
	}
	sort.Strings(paths)
	return paths
}

func collectGetSecretAttributePaths(
	value any,
	prefix string,
	attribute string,
	paths map[string]struct{},
) {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			currentPath := key
			if prefix != "" {
				currentPath = prefix + "." + key
			}
			if key == attribute {
				paths[currentPath] = struct{}{}
			}
			collectGetSecretAttributePaths(field, currentPath, attribute, paths)
		}
	case []any:
		// Arrays are intentionally ignored because metadata secret paths are map-path based.
		return
	}
}
