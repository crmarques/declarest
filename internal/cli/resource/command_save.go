package resource

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/internal/support/identity"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func newSaveCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var asItems bool
	var asOneResource bool
	var ignore bool

	command := &cobra.Command{
		Use:   "save [path]",
		Short: "Save resource value into repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			if asItems && asOneResource {
				return common.ValidationError("flags --as-items and --as-one-resource cannot be used together", nil)
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			value, hasInput, err := decodeOptionalResourceInput(command, input)
			if err != nil {
				return err
			}
			if !hasInput {
				remoteValue, err := orchestratorService.GetRemote(command.Context(), resolvedPath)
				if err != nil {
					return err
				}
				if err := enforceSaveSecretSafety(command.Context(), deps, resolvedPath, remoteValue, ignore); err != nil {
					return err
				}
				return orchestratorService.Save(command.Context(), resolvedPath, remoteValue)
			}

			items, isListPayload, err := extractSaveListItems(value)
			if err != nil {
				return err
			}

			if asOneResource || (!asItems && !isListPayload) {
				if err := enforceSaveSecretSafety(command.Context(), deps, resolvedPath, value, ignore); err != nil {
					return err
				}
				return orchestratorService.Save(command.Context(), resolvedPath, value)
			}
			if !isListPayload {
				return common.ValidationError("input payload is not a list; use --as-one-resource to save a single resource", nil)
			}

			entries, err := resolveSaveEntriesForItems(command.Context(), deps, resolvedPath, items)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := enforceSaveSecretSafety(command.Context(), deps, entry.LogicalPath, entry.Payload, ignore); err != nil {
					return err
				}
			}
			for _, entry := range entries {
				if err := orchestratorService.Save(command.Context(), entry.LogicalPath, entry.Payload); err != nil {
					return err
				}
			}

			return nil
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	command.Flags().BoolVar(&asItems, "as-items", false, "save list payload entries as individual resources")
	command.Flags().BoolVar(&asOneResource, "as-one-resource", false, "save payload as one resource file")
	command.Flags().BoolVar(&ignore, "ignore", false, "ignore plaintext-secret safety validation when saving")
	return command
}

type saveEntry struct {
	LogicalPath string
	Payload     resource.Value
}

func extractSaveListItems(value resource.Value) ([]any, bool, error) {
	switch typed := value.(type) {
	case []any:
		return typed, true, nil
	case map[string]any:
		itemsValue, hasItems := typed["items"]
		if !hasItems {
			return nil, false, nil
		}
		items, ok := itemsValue.([]any)
		if !ok {
			return nil, false, common.ValidationError(`list payload "items" must be an array`, nil)
		}
		return items, true, nil
	default:
		return nil, false, nil
	}
}

func resolveSaveEntriesForItems(
	ctx context.Context,
	deps common.CommandDependencies,
	collectionPath string,
	items []any,
) ([]saveEntry, error) {
	normalizedCollectionPath, err := resource.NormalizeLogicalPath(collectionPath)
	if err != nil {
		return nil, err
	}

	entries := make([]saveEntry, 0, len(items))
	seenPaths := make(map[string]struct{}, len(items))

	var metadataResolved bool
	var resolvedMetadata metadatadomain.ResourceMetadata

	for _, rawItem := range items {
		normalizedItem, err := resource.Normalize(rawItem)
		if err != nil {
			return nil, err
		}

		itemMap, ok := normalizedItem.(map[string]any)
		if !ok {
			return nil, common.ValidationError("list payload entries must be JSON objects", nil)
		}

		entry, usedResourceEntryShape, err := resolveSaveEntryFromResourceShape(itemMap)
		if err != nil {
			return nil, err
		}
		if !usedResourceEntryShape {
			if !metadataResolved {
				metadataService, metadataErr := common.RequireMetadataService(deps)
				if metadataErr != nil {
					return nil, metadataErr
				}
				resolvedMetadata, metadataErr = metadataService.ResolveForPath(ctx, normalizedCollectionPath)
				if metadataErr != nil {
					if !isTypedErrorCategory(metadataErr, faults.NotFoundError) {
						return nil, metadataErr
					}
					resolvedMetadata = metadatadomain.ResourceMetadata{}
				}
				metadataResolved = true
			}

			alias, _, err := identity.ResolveAliasAndRemoteIDForListItem(itemMap, resolvedMetadata)
			if err != nil {
				return nil, common.ValidationError(
					"list item alias could not be resolved; configure metadata alias/id attributes or use --as-one-resource",
					err,
				)
			}

			logicalPath, err := buildLogicalPathForSave(normalizedCollectionPath, alias)
			if err != nil {
				return nil, err
			}
			entry = saveEntry{
				LogicalPath: logicalPath,
				Payload:     itemMap,
			}
		}

		if _, exists := seenPaths[entry.LogicalPath]; exists {
			return nil, common.ValidationError(
				fmt.Sprintf("list payload contains duplicate resource path %q", entry.LogicalPath),
				nil,
			)
		}
		seenPaths[entry.LogicalPath] = struct{}{}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].LogicalPath < entries[j].LogicalPath
	})
	return entries, nil
}

func resolveSaveEntryFromResourceShape(item map[string]any) (saveEntry, bool, error) {
	logicalPathValue, hasLogicalPath := item["LogicalPath"]
	payloadValue, hasPayload := item["Payload"]
	if !hasLogicalPath && !hasPayload {
		return saveEntry{}, false, nil
	}
	if !hasLogicalPath || !hasPayload {
		return saveEntry{}, false, common.ValidationError(
			`resource list entry must include both "LogicalPath" and "Payload"`,
			nil,
		)
	}

	logicalPath, ok := logicalPathValue.(string)
	if !ok || strings.TrimSpace(logicalPath) == "" {
		return saveEntry{}, false, common.ValidationError(`resource list entry "LogicalPath" must be a non-empty string`, nil)
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return saveEntry{}, false, err
	}
	// payloadValue was already normalized by resolveSaveEntriesForItems.
	normalizedPayload := payloadValue

	return saveEntry{
		LogicalPath: normalizedPath,
		Payload:     normalizedPayload,
	}, true, nil
}

func buildLogicalPathForSave(collectionPath string, alias string) (string, error) {
	joined := path.Join(collectionPath, alias)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	return resource.NormalizeLogicalPath(joined)
}

func enforceSaveSecretSafety(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	value resource.Value,
	ignore bool,
) error {
	if ignore {
		return nil
	}

	candidates, err := detectSaveSecretCandidates(ctx, deps, logicalPath, value)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	return common.ValidationError(
		fmt.Sprintf(
			"warning: potential plaintext secrets detected for %q at attributes [%s]; refusing to save without --ignore",
			logicalPath,
			strings.Join(candidates, ", "),
		),
		nil,
	)
}

func detectSaveSecretCandidates(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	value resource.Value,
) ([]string, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]struct{})

	heuristicCandidates, err := detectHeuristicSecretCandidates(ctx, deps, normalizedValue)
	if err != nil {
		return nil, err
	}
	for _, candidate := range heuristicCandidates {
		candidates[candidate] = struct{}{}
	}

	resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, logicalPath)
	if err != nil {
		return nil, err
	}
	for _, candidate := range detectMetadataSecretCandidates(normalizedValue, resolvedMetadata.SecretsFromAttributes) {
		candidates[candidate] = struct{}{}
	}

	result := make([]string, 0, len(candidates))
	for candidate := range candidates {
		result = append(result, candidate)
	}
	sort.Strings(result)
	return result, nil
}

func detectHeuristicSecretCandidates(
	ctx context.Context,
	deps common.CommandDependencies,
	value resource.Value,
) ([]string, error) {
	if deps.Secrets != nil {
		return deps.Secrets.DetectSecretCandidates(ctx, value)
	}

	return secretdomain.DetectSecretCandidates(value)
}

func resolveMetadataForSecretCheck(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
) (metadatadomain.ResourceMetadata, error) {
	if deps.Metadata == nil {
		return metadatadomain.ResourceMetadata{}, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	resolvedMetadata, err := deps.Metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if isTypedErrorCategory(err, faults.NotFoundError) {
			return metadatadomain.ResourceMetadata{}, nil
		}
		return metadatadomain.ResourceMetadata{}, err
	}
	return resolvedMetadata, nil
}

func detectMetadataSecretCandidates(value resource.Value, attributes []string) []string {
	payload, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	candidates := make([]string, 0)
	seenAttributes := make(map[string]struct{})
	for _, rawAttribute := range attributes {
		attribute := strings.TrimSpace(rawAttribute)
		if attribute == "" {
			continue
		}
		if _, seen := seenAttributes[attribute]; seen {
			continue
		}
		seenAttributes[attribute] = struct{}{}

		fieldValue, found := identity.LookupScalarAttribute(payload, attribute)
		if !found || strings.TrimSpace(fieldValue) == "" {
			continue
		}
		if isSecretPlaceholderValue(fieldValue) {
			continue
		}
		candidates = append(candidates, attribute)
	}

	sort.Strings(candidates)
	return candidates
}

func isSecretPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}")
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "secret") {
		return false
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "secret"))
	if argument == "." {
		return true
	}
	if !strings.HasPrefix(argument, "\"") {
		return false
	}

	parsed, err := strconv.Unquote(argument)
	if err != nil {
		return false
	}
	return strings.TrimSpace(parsed) != ""
}

func isTypedErrorCategory(err error, category faults.ErrorCategory) bool {
	if err == nil {
		return false
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	return typedErr.Category == category
}
