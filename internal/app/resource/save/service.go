package save

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
	Repository   repository.ResourceStore
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
}

type ExecuteOptions struct {
	AsItems       bool
	AsOneResource bool
	Ignore        bool
	Force         bool

	HandleSecretsEnabled      bool
	RequestedSecretCandidates []string
}

type saveRemoteReader interface {
	GetRemote(ctx context.Context, logicalPath string) (resource.Value, error)
	ListRemote(ctx context.Context, logicalPath string, policy orchestratordomain.ListPolicy) ([]resource.Resource, error)
}

func Execute(
	ctx context.Context,
	deps Dependencies,
	resolvedPath string,
	value resource.Value,
	hasInput bool,
	options ExecuteOptions,
) error {
	normalizedPath, hasWildcard, explicitCollectionTarget, err := normalizeSavePathPattern(resolvedPath)
	if err != nil {
		return err
	}
	if options.AsItems && options.AsOneResource {
		return validationError("flags --as-items and --as-one-resource cannot be used together", nil)
	}

	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return err
	}
	repositoryService, err := requireResourceStore(deps)
	if err != nil {
		return err
	}

	if hasWildcard {
		if hasInput {
			return validationError("wildcard save paths are supported only when reading from remote server", nil)
		}

		targets, err := expandSaveWildcardPaths(ctx, orchestratorService, normalizedPath)
		if err != nil {
			return err
		}

		matchedCount := 0
		for _, targetPath := range targets {
			remoteValue, err := orchestratorService.GetRemote(ctx, targetPath)
			if err != nil {
				if isTypedErrorCategory(err, faults.NotFoundError) {
					continue
				}
				return err
			}
			matchedCount++

			if err := saveResolvedPathPayload(
				ctx,
				deps,
				orchestratorService,
				repositoryService,
				targetPath,
				remoteValue,
				options.AsItems,
				options.AsOneResource,
				options.Ignore,
				options.HandleSecretsEnabled,
				options.RequestedSecretCandidates,
				options.Force,
			); err != nil {
				return err
			}
		}

		if matchedCount == 0 {
			return faults.NewTypedError(
				faults.NotFoundError,
				fmt.Sprintf("no remote resources matched wildcard path %q", normalizedPath),
				nil,
			)
		}
		return nil
	}

	if !hasInput {
		remoteValue, err := resolveSaveRemoteValue(
			ctx,
			orchestratorService,
			deps.Metadata,
			normalizedPath,
			explicitCollectionTarget,
		)
		if err != nil {
			return err
		}
		value = remoteValue
	}

	return saveResolvedPathPayload(
		ctx,
		deps,
		orchestratorService,
		repositoryService,
		normalizedPath,
		value,
		options.AsItems,
		options.AsOneResource,
		options.Ignore,
		options.HandleSecretsEnabled,
		options.RequestedSecretCandidates,
		options.Force,
	)
}
func saveResolvedPathPayload(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestratordomain.Orchestrator,
	repositoryService repository.ResourceStore,
	resolvedPath string,
	value resource.Value,
	asItems bool,
	asOneResource bool,
	ignore bool,
	handleSecretsEnabled bool,
	requestedSecretCandidates []string,
	force bool,
) error {
	items, isListPayload, err := extractSaveListItems(value)
	if err != nil {
		return err
	}

	if asOneResource || (!asItems && !isListPayload) {
		if err := ensureSaveTargetAllowed(ctx, repositoryService, resolvedPath, force); err != nil {
			return err
		}
		if handleSecretsEnabled {
			value, unhandled, err := handleSaveSecrets(
				ctx,
				deps,
				resolvedPath,
				value,
				"",
				requestedSecretCandidates,
			)
			if err != nil {
				return err
			}
			declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
			if err != nil {
				return err
			}
			blockingCandidates := filterSaveSecretCandidatesForSafety(unhandled, declaredCandidates, ignore)
			if len(blockingCandidates) > 0 {
				return saveSecretSafetyError(resolvedPath, blockingCandidates)
			}
			return orchestratorService.Save(ctx, resolvedPath, value)
		}
		value, err = autoHandleDeclaredSaveSecrets(ctx, deps, resolvedPath, value)
		if err != nil {
			return err
		}
		if err := enforceSaveSecretSafety(ctx, deps, resolvedPath, value, ignore); err != nil {
			return err
		}
		return orchestratorService.Save(ctx, resolvedPath, value)
	}
	if !isListPayload {
		return validationError("input payload is not a list; use --as-one-resource to save a single resource", nil)
	}

	entries, err := resolveSaveEntriesForItems(ctx, deps, resolvedPath, items)
	if err != nil {
		return err
	}
	if err := ensureSaveEntriesWritable(ctx, repositoryService, entries, force); err != nil {
		return err
	}
	collectionCandidates, err := detectSaveSecretCandidatesForCollection(ctx, deps, resolvedPath, entries)
	if err != nil {
		return err
	}
	if handleSecretsEnabled {
		selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(
			collectionCandidates,
			requestedSecretCandidates,
			true,
		)
		if err != nil {
			return err
		}

		if len(selectedCandidates) > 0 {
			secretProvider, err := requireSecretProvider(deps)
			if err != nil {
				return err
			}

			entries, err = applySaveSecretCandidatesToEntries(ctx, secretProvider, entries, selectedCandidates)
			if err != nil {
				return err
			}

			if err := persistSaveSecretAttributes(
				ctx,
				deps,
				saveSecretMetadataPathForCollection(resolvedPath),
				selectedCandidates,
			); err != nil {
				return err
			}
		}

		declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
		if err != nil {
			return err
		}

		blockingCandidates := filterSaveSecretCandidatesForSafety(unhandledCandidates, declaredCandidates, ignore)
		if len(blockingCandidates) > 0 {
			return saveSecretSafetyError(resolvedPath, blockingCandidates)
		}
	} else {
		declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
		if err != nil {
			return err
		}
		entries, err = autoHandleDeclaredSaveSecretsForEntries(
			ctx,
			deps,
			entries,
			collectionCandidates,
			declaredCandidates,
		)
		if err != nil {
			return err
		}

		blockingCandidates := filterSaveSecretCandidatesForSafety(collectionCandidates, declaredCandidates, ignore)
		if len(blockingCandidates) > 0 {
			return saveSecretSafetyError(resolvedPath, blockingCandidates)
		}
	}
	for _, entry := range entries {
		if err := orchestratorService.Save(ctx, entry.LogicalPath, entry.Payload); err != nil {
			return err
		}
	}

	return nil
}

func normalizeSavePathPattern(rawPath string) (string, bool, bool, error) {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return "", false, false, validationError("path is required", nil)
	}
	explicitCollectionTarget := trimmedPath != "/" && strings.HasSuffix(trimmedPath, "/")

	normalizedInput := strings.ReplaceAll(trimmedPath, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", false, false, validationError("logical path must be absolute", nil)
	}

	for _, segment := range strings.Split(normalizedInput, "/") {
		if segment == ".." {
			return "", false, false, validationError("logical path must not contain traversal segments", nil)
		}
	}

	normalizedPath := path.Clean(normalizedInput)
	if !strings.HasPrefix(normalizedPath, "/") {
		return "", false, false, validationError("logical path must be absolute", nil)
	}
	if normalizedPath != "/" {
		normalizedPath = strings.TrimSuffix(normalizedPath, "/")
	}

	hasWildcard := false
	for _, segment := range splitSavePathSegments(normalizedPath) {
		if segment == "_" {
			hasWildcard = true
			break
		}
	}

	return normalizedPath, hasWildcard, explicitCollectionTarget, nil
}

func resolveSaveRemoteValue(
	ctx context.Context,
	remoteReader saveRemoteReader,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	explicitCollectionTarget bool,
) (resource.Value, error) {
	if explicitCollectionTarget {
		items, err := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
		if err == nil {
			return saveListPayloadFromResources(items), nil
		}
		if !isCollectionListShapeError(err) {
			return nil, err
		}
	}

	remoteValue, err := remoteReader.GetRemote(ctx, logicalPath)
	if err == nil {
		return remoteValue, nil
	}
	if !isTypedErrorCategory(err, faults.NotFoundError) {
		return nil, err
	}

	items, listErr := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
	if listErr != nil {
		return nil, err
	}
	if !explicitCollectionTarget && !shouldUseSaveCollectionFallback(ctx, metadataService, logicalPath, items) {
		return nil, err
	}

	return saveListPayloadFromResources(items), nil
}

func saveListPayloadFromResources(items []resource.Resource) resource.Value {
	if len(items) == 0 {
		return []any{}
	}

	sorted := make([]resource.Resource, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].LogicalPath < sorted[j].LogicalPath
	})

	payload := make([]any, 0, len(sorted))
	for _, item := range sorted {
		payload = append(payload, item.Payload)
	}
	return payload
}

func shouldUseSaveCollectionFallback(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	items []resource.Resource,
) bool {
	if len(items) == 0 {
		return true
	}

	collectionChildrenResolver, ok := metadataService.(metadatadomain.CollectionChildrenResolver)
	if !ok {
		return false
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil || normalizedPath == "/" {
		return false
	}

	parentPath := path.Dir(normalizedPath)
	if parentPath == "." || parentPath == "" {
		parentPath = "/"
	}
	requestedSegment := path.Base(normalizedPath)
	if strings.TrimSpace(requestedSegment) == "" || requestedSegment == "/" {
		return false
	}

	children, err := collectionChildrenResolver.ResolveCollectionChildren(ctx, parentPath)
	if err != nil {
		return false
	}
	for _, child := range children {
		if strings.TrimSpace(child) == requestedSegment {
			return true
		}
	}
	return false
}

func isCollectionListShapeError(err error) bool {
	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	if typedErr.Category != faults.ValidationError {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(typedErr.Message))
	return strings.HasPrefix(message, "list response ") || strings.HasPrefix(message, "list payload ")
}

func splitSavePathSegments(logicalPath string) []string {
	trimmed := strings.Trim(strings.TrimSpace(logicalPath), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func expandSaveWildcardPaths(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	wildcardPath string,
) ([]string, error) {
	segments := splitSavePathSegments(wildcardPath)
	if len(segments) == 0 {
		return nil, validationError("wildcard save path must target a collection or resource", nil)
	}

	currentPaths := []string{"/"}
	for _, segment := range segments {
		nextPaths := make(map[string]struct{})

		if segment == "_" {
			for _, parentPath := range currentPaths {
				items, err := orchestratorService.ListRemote(ctx, parentPath, orchestratordomain.ListPolicy{Recursive: false})
				if err != nil {
					return nil, err
				}

				for _, item := range items {
					childSegment, ok := directChildSegment(parentPath, item.LogicalPath)
					if !ok {
						continue
					}
					childPath, err := appendSavePathSegment(parentPath, childSegment)
					if err != nil {
						return nil, err
					}
					nextPaths[childPath] = struct{}{}
				}
			}
		} else {
			for _, parentPath := range currentPaths {
				childPath, err := appendSavePathSegment(parentPath, segment)
				if err != nil {
					return nil, err
				}
				nextPaths[childPath] = struct{}{}
			}
		}

		if len(nextPaths) == 0 {
			return nil, faults.NewTypedError(
				faults.NotFoundError,
				fmt.Sprintf("no remote resources matched wildcard path %q", wildcardPath),
				nil,
			)
		}

		currentPaths = sortedPathKeys(nextPaths)
	}

	return currentPaths, nil
}

func appendSavePathSegment(parentPath string, segment string) (string, error) {
	trimmedSegment := strings.TrimSpace(segment)
	if trimmedSegment == "" {
		return "", validationError("wildcard path contains an empty segment", nil)
	}

	joined := path.Join(parentPath, trimmedSegment)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	return resource.NormalizeLogicalPath(joined)
}

func directChildSegment(parentPath string, candidatePath string) (string, bool) {
	normalizedParentPath, err := resource.NormalizeLogicalPath(parentPath)
	if err != nil {
		return "", false
	}
	normalizedCandidatePath, err := resource.NormalizeLogicalPath(candidatePath)
	if err != nil {
		return "", false
	}

	if normalizedParentPath == "/" {
		remaining := strings.TrimPrefix(normalizedCandidatePath, "/")
		if remaining == "" || strings.Contains(remaining, "/") {
			return "", false
		}
		return remaining, true
	}

	parentPrefix := strings.TrimSuffix(normalizedParentPath, "/")
	if !strings.HasPrefix(normalizedCandidatePath, parentPrefix+"/") {
		return "", false
	}

	remaining := strings.TrimPrefix(normalizedCandidatePath, parentPrefix+"/")
	if remaining == "" || strings.Contains(remaining, "/") {
		return "", false
	}

	return remaining, true
}

func sortedPathKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
			return nil, false, validationError(`list payload "items" must be an array`, nil)
		}
		return items, true, nil
	default:
		return nil, false, nil
	}
}

func resolveSaveEntriesForItems(
	ctx context.Context,
	deps Dependencies,
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
			return nil, validationError("list payload entries must be JSON objects", nil)
		}

		entry, usedResourceEntryShape, err := resolveSaveEntryFromResourceShape(itemMap)
		if err != nil {
			return nil, err
		}
		if !usedResourceEntryShape {
			if !metadataResolved {
				metadataService, metadataErr := requireMetadataService(deps)
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
				return nil, validationError(
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
			return nil, validationError(
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
		return saveEntry{}, false, validationError(
			`resource list entry must include both "LogicalPath" and "Payload"`,
			nil,
		)
	}

	logicalPath, ok := logicalPathValue.(string)
	if !ok || strings.TrimSpace(logicalPath) == "" {
		return saveEntry{}, false, validationError(`resource list entry "LogicalPath" must be a non-empty string`, nil)
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
		if next != "" && next != "_" {
			segments[idx+1] = "_"
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
	deps Dependencies,
	logicalPath string,
	value resource.Value,
	ignore bool,
) error {
	candidates, err := detectSaveSecretCandidates(ctx, deps, logicalPath, value)
	if err != nil {
		return err
	}

	declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, logicalPath)
	if err != nil {
		return err
	}

	blockingCandidates := filterSaveSecretCandidatesForSafety(candidates, declaredCandidates, ignore)
	if len(blockingCandidates) == 0 {
		return nil
	}

	return saveSecretSafetyError(logicalPath, blockingCandidates)
}

func handleSaveSecrets(
	ctx context.Context,
	deps Dependencies,
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

	selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(candidates, requestedCandidates, false)
	if err != nil {
		return nil, nil, err
	}

	payload, ok := normalizedValue.(map[string]any)
	if !ok {
		return nil, nil, validationError("--handle-secrets requires object payloads", nil)
	}

	secretProvider, err := requireSecretProvider(deps)
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

func autoHandleDeclaredSaveSecrets(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates, err := detectSaveSecretCandidates(ctx, deps, logicalPath, normalizedValue)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return normalizedValue, nil
	}

	declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, logicalPath)
	if err != nil {
		return nil, err
	}

	metadataDeclaredCandidates := intersectSaveSecretCandidates(candidates, declaredCandidates)
	if len(metadataDeclaredCandidates) == 0 {
		return normalizedValue, nil
	}

	secretProvider, err := requireSecretProvider(deps)
	if err != nil {
		return nil, err
	}

	processedPayload, _, err := applySaveSecretCandidates(
		ctx,
		secretProvider,
		logicalPath,
		normalizedValue,
		metadataDeclaredCandidates,
	)
	if err != nil {
		return nil, err
	}
	return processedPayload, nil
}

func autoHandleDeclaredSaveSecretsForEntries(
	ctx context.Context,
	deps Dependencies,
	entries []saveEntry,
	detectedCandidates []string,
	declaredCandidates []string,
) ([]saveEntry, error) {
	metadataDeclaredCandidates := intersectSaveSecretCandidates(detectedCandidates, declaredCandidates)
	if len(metadataDeclaredCandidates) == 0 {
		return entries, nil
	}

	secretProvider, err := requireSecretProvider(deps)
	if err != nil {
		return nil, err
	}

	return applySaveSecretCandidatesToEntries(ctx, secretProvider, entries, metadataDeclaredCandidates)
}

func applySaveSecretCandidatesToEntries(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	entries []saveEntry,
	selectedCandidates []string,
) ([]saveEntry, error) {
	if len(entries) == 0 || len(selectedCandidates) == 0 {
		return entries, nil
	}

	updatedEntries := make([]saveEntry, 0, len(entries))
	for _, entry := range entries {
		processedPayload, _, err := applySaveSecretCandidates(
			ctx,
			secretProvider,
			entry.LogicalPath,
			entry.Payload,
			selectedCandidates,
		)
		if err != nil {
			return nil, err
		}
		updatedEntries = append(updatedEntries, saveEntry{
			LogicalPath: entry.LogicalPath,
			Payload:     processedPayload,
		})
	}
	return updatedEntries, nil
}

func intersectSaveSecretCandidates(candidates []string, declared []string) []string {
	normalizedCandidates := dedupeAndSortSaveSecretAttributes(candidates)
	if len(normalizedCandidates) == 0 || len(declared) == 0 {
		return nil
	}

	declaredSet := make(map[string]struct{}, len(declared))
	for _, candidate := range dedupeAndSortSaveSecretAttributes(declared) {
		declaredSet[candidate] = struct{}{}
	}

	intersections := make([]string, 0, len(normalizedCandidates))
	for _, candidate := range normalizedCandidates {
		if _, found := declaredSet[candidate]; !found {
			continue
		}
		intersections = append(intersections, candidate)
	}
	return intersections
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
		return nil, nil, validationError("--handle-secrets requires object payloads", nil)
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
	deps Dependencies,
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

func selectSaveSecretCandidates(candidates []string, requested []string, allowMissingRequested bool) ([]string, []string, error) {
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
			if allowMissingRequested {
				continue
			}
			return nil, nil, validationError(
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

func resolveDeclaredSaveSecretAttributes(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
) ([]string, error) {
	resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, logicalPath)
	if err != nil {
		return nil, err
	}

	return dedupeAndSortSaveSecretAttributes(resolvedMetadata.SecretsFromAttributes), nil
}

func filterSaveSecretCandidatesForSafety(candidates []string, declared []string, ignore bool) []string {
	normalizedCandidates := dedupeAndSortSaveSecretAttributes(candidates)
	if len(normalizedCandidates) == 0 {
		return nil
	}
	if ignore {
		return nil
	}

	declaredSet := make(map[string]struct{}, len(declared))
	for _, candidate := range dedupeAndSortSaveSecretAttributes(declared) {
		declaredSet[candidate] = struct{}{}
	}

	filtered := make([]string, 0, len(normalizedCandidates))
	for _, candidate := range normalizedCandidates {
		if _, found := declaredSet[candidate]; found {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func saveSecretSafetyError(logicalPath string, candidates []string) error {
	return validationError(
		fmt.Sprintf(
			"warning: potential plaintext secrets detected for %q at attributes [%s]; refusing to save without --ignore",
			logicalPath,
			strings.Join(candidates, ", "),
		),
		nil,
	)
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func requireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, validationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func requireResourceStore(deps Dependencies) (repository.ResourceStore, error) {
	if deps.Repository == nil {
		return nil, validationError("resource repository is not configured", nil)
	}
	return deps.Repository, nil
}

func requireMetadataService(deps Dependencies) (metadatadomain.MetadataService, error) {
	if deps.Metadata == nil {
		return nil, validationError("metadata service is not configured", nil)
	}
	return deps.Metadata, nil
}

func requireSecretProvider(deps Dependencies) (secretdomain.SecretProvider, error) {
	if deps.Secrets == nil {
		return nil, validationError("secret provider is not configured", nil)
	}
	return deps.Secrets, nil
}

func detectSaveSecretCandidates(
	ctx context.Context,
	deps Dependencies,
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
	deps Dependencies,
	value resource.Value,
) ([]string, error) {
	if deps.Secrets != nil {
		return deps.Secrets.DetectSecretCandidates(ctx, value)
	}

	return secretdomain.DetectSecretCandidates(value)
}

func resolveMetadataForSecretCheck(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
) (metadatadomain.ResourceMetadata, error) {
	return secretworkflow.ResolveMetadataForSecretCheck(ctx, deps.Metadata, logicalPath)
}

func detectMetadataSecretCandidates(value resource.Value, attributes []string) []string {
	return secretworkflow.DetectMetadataSecretCandidates(value, attributes)
}

func resolveSaveSecretAttributes(payload map[string]any, candidates []string) []string {
	return secretworkflow.ResolveAttributePathsForCandidates(payload, candidates)
}

func persistSaveSecretAttributes(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	detected []string,
) error {
	attributes := dedupeAndSortSaveSecretAttributes(detected)
	if len(attributes) == 0 {
		return nil
	}

	metadataService, err := requireMetadataService(deps)
	if err != nil {
		return err
	}

	return secretworkflow.PersistDetectedAttributes(ctx, metadataService, logicalPath, attributes)
}

func ensureSaveTargetAllowed(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	logicalPath string,
	force bool,
) error {
	if force {
		return nil
	}

	exists, err := resourceExists(ctx, repositoryService, logicalPath)
	if err != nil {
		return err
	}
	if exists {
		return validationError(
			fmt.Sprintf("resource %q already exists; rerun with --force to override", logicalPath),
			nil,
		)
	}
	return nil
}

func ensureSaveEntriesWritable(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	entries []saveEntry,
	force bool,
) error {
	if force {
		return nil
	}
	for _, entry := range entries {
		if err := ensureSaveTargetAllowed(ctx, repositoryService, entry.LogicalPath, false); err != nil {
			return err
		}
	}
	return nil
}

func resourceExists(
	ctx context.Context,
	repositoryService repository.ResourceStore,
	logicalPath string,
) (bool, error) {
	_, err := repositoryService.Get(ctx, logicalPath)
	if err == nil {
		return true, nil
	}
	if isTypedErrorCategory(err, faults.NotFoundError) {
		return false, nil
	}
	return false, err
}

func dedupeAndSortSaveSecretAttributes(values []string) []string {
	return secretworkflow.DedupeAndSortAttributes(values)
}

func storeAndMaskAttribute(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	payload map[string]any,
	logicalPath string,
	attribute string,
) error {
	return secretworkflow.StoreAndMaskAttribute(ctx, secretProvider, payload, logicalPath, attribute)
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
