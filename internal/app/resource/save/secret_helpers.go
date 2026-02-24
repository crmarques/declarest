package save

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

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
	return faults.IsCategory(err, category)
}
