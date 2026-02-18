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

const handleSecretsAllSentinel = "__all__"

func newSaveCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var asItems bool
	var asOneResource bool
	var ignore bool
	var handleSecrets string

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

			handleSecretsEnabled, requestedSecretCandidates, err := parseSaveHandleSecretsFlag(command, handleSecrets)
			if err != nil {
				return err
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
				value = remoteValue
				hasInput = true
			}

			items, isListPayload, err := extractSaveListItems(value)
			if err != nil {
				return err
			}

			if asOneResource || (!asItems && !isListPayload) {
				if handleSecretsEnabled {
					value, unhandled, err := handleSaveSecrets(
						command.Context(),
						deps,
						resolvedPath,
						value,
						"",
						requestedSecretCandidates,
					)
					if err != nil {
						return err
					}
					if len(unhandled) > 0 && !ignore {
						return saveSecretSafetyError(resolvedPath, unhandled)
					}
					return orchestratorService.Save(command.Context(), resolvedPath, value)
				} else if err := enforceSaveSecretSafety(command.Context(), deps, resolvedPath, value, ignore); err != nil {
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
			collectionCandidates := make([]string, 0)
			if handleSecretsEnabled || !ignore {
				collectionCandidates, err = detectSaveSecretCandidatesForCollection(command.Context(), deps, resolvedPath, entries)
				if err != nil {
					return err
				}
			}
			if handleSecretsEnabled {
				selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(collectionCandidates, requestedSecretCandidates)
				if err != nil {
					return err
				}

				if len(selectedCandidates) > 0 {
					secretProvider, err := common.RequireSecretProvider(deps)
					if err != nil {
						return err
					}

					updatedEntries := make([]saveEntry, 0, len(entries))
					for _, entry := range entries {
						processedPayload, _, err := applySaveSecretCandidates(
							command.Context(),
							secretProvider,
							entry.LogicalPath,
							entry.Payload,
							selectedCandidates,
						)
						if err != nil {
							return err
						}
						updatedEntries = append(updatedEntries, saveEntry{
							LogicalPath: entry.LogicalPath,
							Payload:     processedPayload,
						})
					}
					entries = updatedEntries

					if err := persistSaveSecretAttributes(
						command.Context(),
						deps,
						saveSecretMetadataPathForCollection(resolvedPath),
						selectedCandidates,
					); err != nil {
						return err
					}
				}

				if len(unhandledCandidates) > 0 && !ignore {
					return saveSecretSafetyError(resolvedPath, unhandledCandidates)
				}
			} else if len(collectionCandidates) > 0 && !ignore {
				return saveSecretSafetyError(resolvedPath, collectionCandidates)
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
	command.Flags().StringVar(&handleSecrets, "handle-secrets", "", "detect, store, and mask plaintext secrets while saving (optional comma-separated attributes)")
	handleSecretsFlag := command.Flags().Lookup("handle-secrets")
	handleSecretsFlag.NoOptDefVal = handleSecretsAllSentinel
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

			alias, err := resolveSaveListItemAlias(itemMap, resolvedMetadata)
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

func resolveSaveListItemAlias(
	payload map[string]any,
	md metadatadomain.ResourceMetadata,
) (string, error) {
	alias, _, err := identity.ResolveAliasAndRemoteIDForListItem(payload, md)
	if err == nil && strings.TrimSpace(alias) != "" {
		return strings.TrimSpace(alias), nil
	}

	// Fallback keeps list save usable when metadata identity attributes are absent.
	for _, candidate := range []string{"clientId", "id", "name", "alias", "key", "uuid", "uid"} {
		value, found := identity.LookupScalarAttribute(payload, candidate)
		if !found || strings.TrimSpace(value) == "" {
			continue
		}
		return strings.TrimSpace(value), nil
	}

	return "", err
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

func saveSecretMetadataPathForCollection(collectionPath string) string {
	normalizedCollectionPath, err := resource.NormalizeLogicalPath(collectionPath)
	if err != nil {
		return strings.TrimSpace(collectionPath)
	}

	segments := strings.Split(strings.TrimPrefix(normalizedCollectionPath, "/"), "/")
	for idx := 0; idx < len(segments)-1; idx++ {
		if segments[idx] != "realms" {
			continue
		}
		next := strings.TrimSpace(segments[idx+1])
		if next != "" && next != "*" {
			segments[idx+1] = "*"
		}
		break
	}

	if len(segments) == 1 && segments[0] == "" {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
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

	return saveSecretSafetyError(logicalPath, candidates)
}

func handleSaveSecrets(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	value resource.Value,
	metadataPath string,
	requestedCandidates []string,
) (resource.Value, []string, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, nil, err
	}

	candidates, err := detectSaveSecretCandidates(ctx, deps, logicalPath, normalizedValue)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) == 0 {
		return normalizedValue, nil, nil
	}

	selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(candidates, requestedCandidates)
	if err != nil {
		return nil, nil, err
	}

	payload, ok := normalizedValue.(map[string]any)
	if !ok {
		return nil, nil, common.ValidationError("--handle-secrets requires object payloads", nil)
	}

	secretProvider, err := common.RequireSecretProvider(deps)
	if err != nil {
		return nil, nil, err
	}
	processedPayload, handledAttributes, err := applySaveSecretCandidates(ctx, secretProvider, logicalPath, payload, selectedCandidates)
	if err != nil {
		return nil, nil, err
	}

	targetMetadataPath := strings.TrimSpace(metadataPath)
	if targetMetadataPath == "" {
		targetMetadataPath = logicalPath
	}
	if err := persistSaveSecretAttributes(ctx, deps, targetMetadataPath, handledAttributes); err != nil {
		return nil, nil, err
	}

	return processedPayload, unhandledCandidates, nil
}

func applySaveSecretCandidates(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	logicalPath string,
	value resource.Value,
	selectedCandidates []string,
) (resource.Value, []string, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, nil, err
	}

	payload, ok := normalizedValue.(map[string]any)
	if !ok {
		return nil, nil, common.ValidationError("--handle-secrets requires object payloads", nil)
	}

	attributes := resolveSaveSecretAttributes(payload, selectedCandidates)
	for _, attribute := range attributes {
		if err := storeAndMaskAttribute(ctx, secretProvider, payload, logicalPath, attribute); err != nil {
			return nil, nil, err
		}
	}

	return payload, attributes, nil
}

func detectSaveSecretCandidatesForCollection(
	ctx context.Context,
	deps common.CommandDependencies,
	collectionPath string,
	entries []saveEntry,
) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, collectionPath)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]struct{})
	for _, entry := range entries {
		normalizedValue, err := resource.Normalize(entry.Payload)
		if err != nil {
			return nil, err
		}

		heuristicCandidates, err := detectHeuristicSecretCandidates(ctx, deps, normalizedValue)
		if err != nil {
			return nil, err
		}
		for _, candidate := range heuristicCandidates {
			candidates[candidate] = struct{}{}
		}

		for _, candidate := range detectMetadataSecretCandidates(normalizedValue, resolvedMetadata.SecretsFromAttributes) {
			candidates[candidate] = struct{}{}
		}
	}

	result := make([]string, 0, len(candidates))
	for candidate := range candidates {
		result = append(result, candidate)
	}
	sort.Strings(result)
	return result, nil
}

func parseSaveHandleSecretsFlag(command *cobra.Command, rawValue string) (bool, []string, error) {
	flag := command.Flags().Lookup("handle-secrets")
	if flag == nil || !flag.Changed {
		return false, nil, nil
	}

	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" || trimmed == handleSecretsAllSentinel {
		return true, nil, nil
	}

	items := strings.Split(trimmed, ",")
	requested := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			return false, nil, common.ValidationError("--handle-secrets contains an empty attribute value", nil)
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		requested = append(requested, value)
	}
	sort.Strings(requested)

	return true, requested, nil
}

func selectSaveSecretCandidates(candidates []string, requested []string) ([]string, []string, error) {
	normalizedCandidates := dedupeAndSortSaveSecretAttributes(candidates)
	if len(requested) == 0 {
		return normalizedCandidates, nil, nil
	}

	candidateSet := make(map[string]struct{}, len(normalizedCandidates))
	for _, candidate := range normalizedCandidates {
		candidateSet[candidate] = struct{}{}
	}

	selected := make([]string, 0, len(requested))
	selectedSet := make(map[string]struct{}, len(requested))
	for _, requestedCandidate := range dedupeAndSortSaveSecretAttributes(requested) {
		if _, found := candidateSet[requestedCandidate]; !found {
			return nil, nil, common.ValidationError(
				fmt.Sprintf("requested --handle-secrets attribute %q was not detected", requestedCandidate),
				nil,
			)
		}
		if _, found := selectedSet[requestedCandidate]; found {
			continue
		}
		selectedSet[requestedCandidate] = struct{}{}
		selected = append(selected, requestedCandidate)
	}

	unhandled := make([]string, 0, len(normalizedCandidates))
	for _, candidate := range normalizedCandidates {
		if _, found := selectedSet[candidate]; found {
			continue
		}
		unhandled = append(unhandled, candidate)
	}

	return selected, unhandled, nil
}

func saveSecretSafetyError(logicalPath string, candidates []string) error {
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

func resolveSaveSecretAttributes(payload map[string]any, candidates []string) []string {
	attributes := make(map[string]struct{})
	for _, rawCandidate := range candidates {
		candidate := strings.TrimSpace(rawCandidate)
		if candidate == "" {
			continue
		}

		if strings.Contains(candidate, ".") {
			fieldValue, found := identity.LookupScalarAttribute(payload, candidate)
			if found && strings.TrimSpace(fieldValue) != "" && !isSecretPlaceholderValue(fieldValue) {
				attributes[candidate] = struct{}{}
			}
			continue
		}

		collectCandidateAttributePaths(payload, "", candidate, attributes)
	}

	result := make([]string, 0, len(attributes))
	for attribute := range attributes {
		result = append(result, attribute)
	}
	sort.Strings(result)
	return result
}

func collectCandidateAttributePaths(
	value any,
	prefix string,
	candidate string,
	attributes map[string]struct{},
) {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			attribute := key
			if prefix != "" {
				attribute = prefix + "." + key
			}
			if key == candidate {
				fieldValue, ok := field.(string)
				if ok && strings.TrimSpace(fieldValue) != "" && !isSecretPlaceholderValue(fieldValue) {
					attributes[attribute] = struct{}{}
				}
			}
			collectCandidateAttributePaths(field, attribute, candidate, attributes)
		}
	case []any:
		// Arrays are intentionally skipped because metadata attributes are map-path based.
		return
	}
}

func persistSaveSecretAttributes(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	detected []string,
) error {
	attributes := dedupeAndSortSaveSecretAttributes(detected)
	if len(attributes) == 0 {
		return nil
	}

	metadataService, err := common.RequireMetadataService(deps)
	if err != nil {
		return err
	}

	currentMetadata, err := metadataService.Get(ctx, logicalPath)
	if err != nil {
		if !isTypedErrorCategory(err, faults.NotFoundError) {
			return err
		}
		currentMetadata = metadatadomain.ResourceMetadata{}
	}

	currentMetadata.SecretsFromAttributes = mergeSaveSecretAttributes(
		currentMetadata.SecretsFromAttributes,
		attributes,
	)

	return metadataService.Set(ctx, logicalPath, currentMetadata)
}

func mergeSaveSecretAttributes(existing []string, detected []string) []string {
	merged := make([]string, 0, len(existing)+len(detected))
	seen := make(map[string]struct{}, len(existing)+len(detected))

	for _, raw := range existing {
		attribute := strings.TrimSpace(raw)
		if attribute == "" {
			continue
		}
		if _, found := seen[attribute]; found {
			continue
		}
		seen[attribute] = struct{}{}
		merged = append(merged, attribute)
	}
	for _, raw := range detected {
		attribute := strings.TrimSpace(raw)
		if attribute == "" {
			continue
		}
		if _, found := seen[attribute]; found {
			continue
		}
		seen[attribute] = struct{}{}
		merged = append(merged, attribute)
	}

	sort.Strings(merged)
	return merged
}

func dedupeAndSortSaveSecretAttributes(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func storeAndMaskAttribute(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	payload map[string]any,
	logicalPath string,
	attribute string,
) error {
	secretValue, found := identity.LookupScalarAttribute(payload, attribute)
	if !found || strings.TrimSpace(secretValue) == "" {
		return nil
	}
	if isSecretPlaceholderValue(secretValue) {
		return nil
	}

	parent, leafKey, found := findAttributeParentMap(payload, attribute)
	if !found {
		return nil
	}

	secretKey := buildSaveSecretKey(logicalPath, attribute)
	if err := secretProvider.Store(ctx, secretKey, secretValue); err != nil {
		return err
	}

	parent[leafKey] = secretPlaceholderValue(secretKey)
	return nil
}

func findAttributeParentMap(payload map[string]any, attribute string) (map[string]any, string, bool) {
	segments := strings.Split(strings.TrimSpace(attribute), ".")
	if len(segments) == 0 {
		return nil, "", false
	}

	current := payload
	for idx := 0; idx < len(segments)-1; idx++ {
		segment := strings.TrimSpace(segments[idx])
		if segment == "" {
			return nil, "", false
		}

		nextRaw, exists := current[segment]
		if !exists {
			return nil, "", false
		}
		next, ok := nextRaw.(map[string]any)
		if !ok {
			return nil, "", false
		}
		current = next
	}

	leafKey := strings.TrimSpace(segments[len(segments)-1])
	if leafKey == "" {
		return nil, "", false
	}
	if _, exists := current[leafKey]; !exists {
		return nil, "", false
	}

	return current, leafKey, true
}

func secretPlaceholderValue(key string) string {
	return "{{secret " + strconv.Quote(key) + "}}"
}

func buildSaveSecretKey(logicalPath string, attribute string) string {
	return strings.TrimSpace(logicalPath) + ":" + strings.TrimSpace(attribute)
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
