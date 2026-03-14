package defaults

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies = appdeps.Dependencies

type InferRequest struct {
	ManagedServer bool
	Wait          time.Duration
}

type CheckRequest struct {
	ManagedServer bool
	Wait          time.Duration
}

type Result struct {
	ResolvedPath string
	Content      resource.Content
}

type CheckResult struct {
	ResolvedPath    string
	InferredContent resource.Content
	CurrentContent  resource.Content
	Matches         bool
}

type target struct {
	scopePath         string
	metadataPath      string
	logicalPath       string
	resourceContent   resource.Content
	defaultsContent   resource.Content
	defaultsFound     bool
	payloadDescriptor resource.PayloadDescriptor
}

type localResourceResolver interface {
	ResolveLocalResource(ctx context.Context, logicalPath string) (resource.Resource, error)
}

type managedServerAuthCacheInvalidator interface {
	InvalidateAuthCache()
}

const (
	managedServerProbeReadAttempts    = 8
	managedServerProbeReadMinAttempts = 4
	managedServerProbeReadDelay       = 250 * time.Millisecond
)

func Get(ctx context.Context, deps Dependencies, logicalPath string) (Result, error) {
	resolvedTarget, err := resolveTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	content := resolvedTarget.defaultsContent
	if !resolvedTarget.defaultsFound {
		content = resource.Content{
			Value:      map[string]any{},
			Descriptor: resolvedTarget.payloadDescriptor,
		}
	}

	return Result{
		ResolvedPath: resolvedTarget.scopePath,
		Content:      content,
	}, nil
}

func Save(ctx context.Context, deps Dependencies, logicalPath string, content resource.Content) (Result, error) {
	return saveBaseline(ctx, deps, logicalPath, content)
}

func Infer(ctx context.Context, deps Dependencies, logicalPath string, request InferRequest) (Result, error) {
	resolvedTarget, err := resolveInferTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	var inferred resource.Value
	if request.ManagedServer {
		inferred, err = inferFromManagedServer(ctx, deps, resolvedTarget, request)
	} else {
		inferred, err = inferFromRepository(ctx, deps, resolvedTarget)
	}
	if err != nil {
		return Result{}, err
	}

	return Result{
		ResolvedPath: resolvedTarget.scopePath,
		Content: resource.Content{
			Value:      inferred,
			Descriptor: resolvedTarget.payloadDescriptor,
		},
	}, nil
}

func Check(ctx context.Context, deps Dependencies, logicalPath string, request CheckRequest) (CheckResult, error) {
	inferred, err := Infer(ctx, deps, logicalPath, InferRequest{
		ManagedServer: request.ManagedServer,
		Wait:          request.Wait,
	})
	if err != nil {
		return CheckResult{}, err
	}

	currentTarget, err := resolveInferTarget(ctx, deps, logicalPath)
	if err != nil {
		return CheckResult{}, err
	}
	current := Result{
		ResolvedPath: currentTarget.scopePath,
		Content:      currentTarget.defaultsContent,
	}

	inferredValue := normalizeEmptyDefaultsValue(inferred.Content.Value)
	currentValue := normalizeEmptyDefaultsValue(current.Content.Value)

	inferredNormalized, err := resource.Normalize(inferredValue)
	if err != nil {
		return CheckResult{}, err
	}
	currentNormalized, err := resource.Normalize(currentValue)
	if err != nil {
		return CheckResult{}, err
	}

	inferred.Content.Value = inferredNormalized
	current.Content.Value = currentNormalized

	return CheckResult{
		ResolvedPath:    currentTarget.scopePath,
		InferredContent: inferred.Content,
		CurrentContent:  current.Content,
		Matches:         reflect.DeepEqual(currentNormalized, inferredNormalized),
	}, nil
}

func resolveInferTarget(ctx context.Context, deps Dependencies, logicalPath string) (target, error) {
	parsedPath, err := resource.ParseRawPathWithOptions(logicalPath, resource.RawPathParseOptions{})
	if err != nil {
		return target{}, err
	}
	if parsedPath.Normalized == "/" {
		return target{}, faults.NewValidationError("logical path must target a resource or collection, not root", nil)
	}

	pathDescriptor, err := metadata.ParsePathDescriptor(logicalPath)
	if err != nil {
		return target{}, err
	}
	collectionTargetPath := pathDescriptor.Selector
	concretePath := ""
	resourceContent := resource.Content{}

	if !parsedPath.ExplicitCollectionTarget && !pathDescriptor.Collection {
		orchestratorService, orchestratorErr := appdeps.RequireOrchestrator(deps)
		if orchestratorErr != nil {
			return target{}, orchestratorErr
		}
		resolvedResource, resolveErr := resolveResolvedLocalTarget(ctx, orchestratorService, parsedPath.Normalized)
		if resolveErr == nil {
			collectionTargetPath = collectionPathFor(resolvedResource.LogicalPath)
			concretePath = resolvedResource.LogicalPath
			resourceContent = resource.Content{
				Value:      resolvedResource.Payload,
				Descriptor: resolvedResource.PayloadDescriptor,
			}
		} else if !faults.IsCategory(resolveErr, faults.NotFoundError) {
			return target{}, resolveErr
		}
	}

	if concretePath == "" {
		concretePath, resourceContent, err = resolveFirstCollectionResource(ctx, deps, collectionTargetPath)
		if err != nil {
			return target{}, err
		}
	}

	metadataPath := collectionMetadataPath(collectionTargetPath)
	payloadDescriptor, err := resolveTargetPayloadDescriptor(ctx, deps, metadataPath, resourceContent.Descriptor)
	if err != nil {
		return target{}, err
	}
	defaultsContent, defaultsFound, err := resolveEffectiveDefaultsForPath(ctx, deps, metadataPath, payloadDescriptor)
	if err != nil {
		return target{}, err
	}

	return target{
		scopePath:         collectionTargetPath,
		metadataPath:      metadataPath,
		logicalPath:       concretePath,
		resourceContent:   resourceContent,
		defaultsContent:   defaultsContent,
		defaultsFound:     defaultsFound,
		payloadDescriptor: resource.NormalizePayloadDescriptor(resourceContent.Descriptor),
	}, nil
}

func CompactContentAgainstStoredDefaults(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	content resource.Content,
) (resource.Content, bool, error) {
	resolvedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, false, err
	}
	if resolvedPath == "/" {
		return resource.Content{}, false, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	defaultsContent, defaultsFound, err := resolveEffectiveDefaultsForPath(ctx, deps, resolvedPath, content.Descriptor)
	if err != nil {
		return resource.Content{}, false, err
	}
	if !defaultsFound {
		return content, false, nil
	}

	prunedValue, err := resource.CompactAgainstDefaults(content.Value, defaultsContent.Value)
	if err != nil {
		return resource.Content{}, false, err
	}

	descriptor := content.Descriptor
	if !resource.IsPayloadDescriptorExplicit(descriptor) && resource.IsPayloadDescriptorExplicit(defaultsContent.Descriptor) {
		descriptor = resource.NormalizePayloadDescriptor(defaultsContent.Descriptor)
	}

	return resource.Content{
		Value:      prunedValue,
		Descriptor: descriptor,
	}, true, nil
}

func resolveTarget(ctx context.Context, deps Dependencies, logicalPath string) (target, error) {
	scope, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return target{}, err
	}

	defaultsContent, defaultsFound, err := resolveEffectiveDefaultsForPath(ctx, deps, scope.metadataPath, scope.payloadDescriptor)
	if err != nil {
		return target{}, err
	}
	return target{
		scopePath:         scope.scopePath,
		metadataPath:      scope.metadataPath,
		logicalPath:       firstNonEmpty(scope.concretePath, scope.scopePath),
		resourceContent:   scope.resourceContent,
		defaultsContent:   defaultsContent,
		defaultsFound:     defaultsFound,
		payloadDescriptor: scope.payloadDescriptor,
	}, nil
}

func resolveResolvedLocalTarget(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	normalizedPath string,
) (resource.Resource, error) {
	if resolver, ok := orchestratorService.(localResourceResolver); ok {
		return resolver.ResolveLocalResource(ctx, normalizedPath)
	}
	content, err := orchestratorService.GetLocal(ctx, normalizedPath)
	if err != nil {
		return resource.Resource{}, err
	}
	return resource.Resource{
		LogicalPath:       normalizedPath,
		CollectionPath:    collectionPathFor(normalizedPath),
		Payload:           content.Value,
		PayloadDescriptor: content.Descriptor,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func inferFromRepository(ctx context.Context, deps Dependencies, resolvedTarget target) (resource.Value, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	collectionPath := collectionPathFor(resolvedTarget.logicalPath)
	items, err := orchestratorService.ListLocal(ctx, collectionPath, orchestratordomain.ListPolicy{})
	if err != nil {
		return nil, err
	}

	samples := make([]resource.Value, 0, len(items))
	targetPayloadType := resource.NormalizePayloadDescriptor(resolvedTarget.payloadDescriptor).PayloadType
	for _, item := range items {
		if item.LogicalPath == "" {
			continue
		}
		content, getErr := orchestratorService.GetLocal(ctx, item.LogicalPath)
		if getErr != nil {
			return nil, getErr
		}
		if resource.NormalizePayloadDescriptor(content.Descriptor).PayloadType != targetPayloadType {
			continue
		}
		samples = append(samples, content.Value)
	}

	return resource.InferDefaultsFromValues(samples...)
}

func inferFromManagedServer(
	ctx context.Context,
	deps Dependencies,
	resolvedTarget target,
	request InferRequest,
) (_ resource.Value, err error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return nil, err
	}

	md, err := metadataService.ResolveForPath(ctx, resolvedTarget.logicalPath)
	if err != nil {
		return nil, err
	}

	firstPayload, firstPath, err := buildManagedServerProbePayload(resolvedTarget.logicalPath, md, resolvedTarget.resourceContent, "probe-1")
	if err != nil {
		return nil, err
	}
	secondPayload, secondPath, err := buildManagedServerProbePayload(resolvedTarget.logicalPath, md, resolvedTarget.resourceContent, "probe-2")
	if err != nil {
		return nil, err
	}

	tempPaths := []string{}
	defer func() {
		var cleanupErr error
		for idx := len(tempPaths) - 1; idx >= 0; idx-- {
			deleteErr := cleanupManagedServerProbe(ctx, deps, orchestratorService, tempPaths[idx])
			if deleteErr != nil {
				cleanupErr = errors.Join(
					cleanupErr,
					faults.NewValidationError(
						fmt.Sprintf("failed to delete managed-server defaults probe %q", tempPaths[idx]),
						deleteErr,
					),
				)
			}
		}
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()

	if _, err := orchestratorService.Create(ctx, firstPath, firstPayload); err != nil {
		return nil, err
	}
	tempPaths = append(tempPaths, firstPath)

	if _, err := orchestratorService.Create(ctx, secondPath, secondPayload); err != nil {
		return nil, err
	}
	tempPaths = append(tempPaths, secondPath)

	if request.Wait > 0 {
		if err := waitForManagedServerDelay(ctx, request.Wait); err != nil {
			return nil, err
		}
	}

	invalidateManagedServerAuthCache(deps)

	firstRemote, err := readManagedServerProbeContent(ctx, orchestratorService, firstPath)
	if err != nil {
		return nil, err
	}
	secondRemote, err := readManagedServerProbeContent(ctx, orchestratorService, secondPath)
	if err != nil {
		return nil, err
	}

	inputs := []resource.Value{
		nil,
		nil,
	}
	inputs[0], err = managedServerProbeInputValue(ctx, deps, resolvedTarget, firstPath, firstPayload)
	if err != nil {
		return nil, err
	}
	inputs[1], err = managedServerProbeInputValue(ctx, deps, resolvedTarget, secondPath, secondPayload)
	if err != nil {
		return nil, err
	}
	outputs := []resource.Value{firstRemote.Value, secondRemote.Value}
	return resource.InferCreatedDefaults(inputs, outputs)
}

func managedServerProbeInputValue(
	ctx context.Context,
	deps Dependencies,
	resolvedTarget target,
	logicalPath string,
	content resource.Content,
) (resource.Value, error) {
	inputValue := resolveSecretsForManagedServerProbe(ctx, deps.SecretProvider(), logicalPath, content)
	return inputValue, nil
}

func invalidateManagedServerAuthCache(deps Dependencies) {
	if deps.Services == nil {
		return
	}
	managedServerClient := deps.Services.ManagedServerClient()
	if managedServerClient == nil {
		return
	}
	invalidator, ok := managedServerClient.(managedServerAuthCacheInvalidator)
	if !ok {
		return
	}
	invalidator.InvalidateAuthCache()
}

func readManagedServerProbeContent(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
) (resource.Content, error) {
	var (
		lastContent    resource.Content
		lastNormalized resource.Value
		stableReads    int
	)

	for attempt := 0; attempt < managedServerProbeReadAttempts; attempt++ {
		content, err := orchestratorService.GetRemote(ctx, logicalPath)
		if err != nil {
			return resource.Content{}, err
		}

		normalized, err := resource.Normalize(content.Value)
		if err != nil {
			return resource.Content{}, err
		}
		content.Value = normalized

		if attempt > 0 && reflect.DeepEqual(normalized, lastNormalized) {
			stableReads++
		} else {
			stableReads = 1
		}

		lastContent = content
		lastNormalized = normalized

		if attempt+1 >= managedServerProbeReadMinAttempts && stableReads >= 2 {
			return lastContent, nil
		}
		if attempt+1 == managedServerProbeReadAttempts {
			break
		}
		if waitErr := waitForManagedServerDelay(ctx, managedServerProbeReadDelay); waitErr != nil {
			return resource.Content{}, waitErr
		}
	}

	return lastContent, nil
}

func cleanupManagedServerProbe(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
) error {
	deleteErr := orchestratorService.Delete(ctx, logicalPath, orchestratordomain.DeletePolicy{})
	if deleteErr == nil || faults.IsCategory(deleteErr, faults.NotFoundError) {
		return nil
	}
	if !faults.IsCategory(deleteErr, faults.AuthError) {
		return deleteErr
	}

	retryErr := retryManagedServerProbeDelete(ctx, deps, logicalPath)
	if retryErr == nil || faults.IsCategory(retryErr, faults.NotFoundError) {
		return nil
	}
	return errors.Join(deleteErr, retryErr)
}

func retryManagedServerProbeDelete(ctx context.Context, deps Dependencies, logicalPath string) error {
	if deps.Services == nil {
		return faults.NewValidationError("managed-server cleanup retry requires service accessor", nil)
	}
	managedServerClient := deps.Services.ManagedServerClient()
	if managedServerClient == nil {
		return faults.NewValidationError("managed-server cleanup retry requires managed-server client", nil)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if invalidator, ok := managedServerClient.(managedServerAuthCacheInvalidator); ok {
			invalidator.InvalidateAuthCache()
		}
		_, err := managedServerClient.Request(ctx, managedserverdomain.RequestSpec{
			Method: http.MethodDelete,
			Path:   logicalPath,
		})
		if err == nil || faults.IsCategory(err, faults.NotFoundError) {
			return nil
		}
		lastErr = err
		if !faults.IsCategory(err, faults.AuthError) || attempt == 1 {
			break
		}
		if waitErr := waitForManagedServerDelay(ctx, 250*time.Millisecond); waitErr != nil {
			return errors.Join(lastErr, waitErr)
		}
	}
	return lastErr
}

func waitForManagedServerDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildManagedServerProbePayload(
	logicalPath string,
	md metadata.ResourceMetadata,
	content resource.Content,
	label string,
) (resource.Content, string, error) {
	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return resource.Content{}, "", err
	}

	payload, ok := normalizedValue.(map[string]any)
	if !ok {
		return resource.Content{}, "", faults.NewValidationError("managed-server defaults inference requires an object payload", nil)
	}

	aliasPointer, aliasOK, err := identity.SimpleAliasPointer(md)
	if err != nil {
		return resource.Content{}, "", err
	}
	if !aliasOK && strings.TrimSpace(md.Alias) == "" {
		aliasPointer = "/id"
		aliasOK = true
	}

	idPointer, idOK, err := identity.SimpleIDPointer(md)
	if err != nil {
		return resource.Content{}, "", err
	}
	if !idOK && strings.TrimSpace(md.ID) == "" {
		idPointer = "/id"
		idOK = true
	}

	if !aliasOK && !idOK {
		return resource.Content{}, "", faults.NewValidationError(
			"managed-server defaults inference requires simple resource.alias or resource.id metadata",
			nil,
		)
	}

	tempName := "declarest-defaults-" + label + "-" + strings.ToLower(uuid.NewString()[:8])
	next := resource.DeepCopyValue(payload)
	replacedPointers := map[string]struct{}{}

	if aliasOK {
		next, err = resource.SetJSONPointerValue(next, aliasPointer, tempName)
		if err != nil {
			return resource.Content{}, "", err
		}
		replacedPointers[aliasPointer] = struct{}{}
	}
	if idOK {
		next, err = resource.SetJSONPointerValue(next, idPointer, tempName)
		if err != nil {
			return resource.Content{}, "", err
		}
		replacedPointers[idPointer] = struct{}{}
	}

	nextPayload, ok := next.(map[string]any)
	if !ok {
		return resource.Content{}, "", faults.NewValidationError("managed-server defaults inference requires an object payload", nil)
	}
	nextPayload, err = applyManagedServerProbeIdentityFallback(logicalPath, payload, nextPayload, tempName, replacedPointers, aliasPointer, idPointer)
	if err != nil {
		return resource.Content{}, "", err
	}

	return resource.Content{
		Value:      nextPayload,
		Descriptor: content.Descriptor,
	}, joinLogicalPath(collectionPathFor(logicalPath), tempName), nil
}

func applyManagedServerProbeIdentityFallback(
	logicalPath string,
	originalPayload map[string]any,
	nextPayload map[string]any,
	tempName string,
	replacedPointers map[string]struct{},
	identityPointers ...string,
) (map[string]any, error) {
	identityValues, err := managedServerProbeIdentityValues(logicalPath, originalPayload, identityPointers...)
	if err != nil {
		return nil, err
	}
	if len(identityValues) == 0 {
		return nextPayload, nil
	}

	allowedKeys := managedServerProbeIdentityFieldKeys(logicalPath)
	current := nextPayload
	for key, rawValue := range originalPayload {
		value, ok := rawValue.(string)
		if !ok || !matchesManagedServerProbeIdentityValue(identityValues, value) {
			continue
		}

		pointer := resource.JSONPointerForObjectKey(key)
		if _, alreadyReplaced := replacedPointers[pointer]; alreadyReplaced {
			continue
		}
		if _, allowed := allowedKeys[canonicalManagedServerProbeFieldKey(key)]; !allowed {
			continue
		}

		updated, err := resource.SetJSONPointerValue(current, pointer, tempName)
		if err != nil {
			return nil, err
		}
		objectValue, ok := updated.(map[string]any)
		if !ok {
			return nil, faults.NewValidationError("managed-server defaults inference requires an object payload", nil)
		}
		current = objectValue
	}

	return current, nil
}

func managedServerProbeIdentityValues(
	logicalPath string,
	payload map[string]any,
	identityPointers ...string,
) (map[string]struct{}, error) {
	values := map[string]struct{}{}
	addManagedServerProbeIdentityValue(values, path.Base(strings.TrimSpace(logicalPath)))

	for _, pointer := range identityPointers {
		value, found, err := resource.LookupJSONPointerString(payload, pointer)
		if err != nil {
			return nil, err
		}
		if found {
			addManagedServerProbeIdentityValue(values, value)
		}
	}

	return values, nil
}

func addManagedServerProbeIdentityValue(values map[string]struct{}, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "/" || trimmed == "." {
		return
	}
	values[trimmed] = struct{}{}
}

func matchesManagedServerProbeIdentityValue(values map[string]struct{}, value string) bool {
	_, ok := values[strings.TrimSpace(value)]
	return ok
}

func managedServerProbeIdentityFieldKeys(logicalPath string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, candidate := range []string{"id", "name", "slug", "key", "code", "alias", "identifier", "uid"} {
		addManagedServerProbeIdentityFieldKey(keys, candidate)
	}

	collectionSegments := resource.SplitLogicalPathSegments(collectionPathFor(logicalPath))
	if len(collectionSegments) == 0 {
		return keys
	}

	collectionName := collectionSegments[len(collectionSegments)-1]
	singularName := singularManagedServerProbeIdentityField(collectionName)
	for _, candidate := range []string{
		collectionName,
		singularName,
		singularName + "id",
		singularName + "name",
	} {
		addManagedServerProbeIdentityFieldKey(keys, candidate)
	}
	return keys
}

func addManagedServerProbeIdentityFieldKey(keys map[string]struct{}, value string) {
	canonical := canonicalManagedServerProbeFieldKey(value)
	if canonical == "" {
		return
	}
	keys[canonical] = struct{}{}
}

func canonicalManagedServerProbeFieldKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(normalized)
}

func singularManagedServerProbeIdentityField(value string) string {
	trimmed := strings.TrimSpace(value)
	switch {
	case strings.HasSuffix(trimmed, "ies") && len(trimmed) > len("ies"):
		return trimmed[:len(trimmed)-len("ies")] + "y"
	case strings.HasSuffix(trimmed, "ses") && len(trimmed) > len("ses"):
		return strings.TrimSuffix(trimmed, "es")
	case strings.HasSuffix(trimmed, "s") && !strings.HasSuffix(trimmed, "ss") && len(trimmed) > 1:
		return strings.TrimSuffix(trimmed, "s")
	default:
		return trimmed
	}
}

func resolveSecretsForManagedServerProbe(
	ctx context.Context,
	provider secretdomain.SecretProvider,
	logicalPath string,
	content resource.Content,
) resource.Value {
	if provider == nil {
		return content.Value
	}

	resolved, err := secretdomain.ResolvePayloadDirectivesForResource(
		content.Value,
		logicalPath,
		content.Descriptor,
		func(key string) (string, error) {
			return provider.Get(ctx, key)
		},
	)
	if err != nil {
		return content.Value
	}
	return resolved
}

func chooseDefaultsDescriptor(candidate resource.PayloadDescriptor, fallback resource.PayloadDescriptor) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(candidate) {
		return resource.NormalizePayloadDescriptor(candidate)
	}
	return resource.NormalizePayloadDescriptor(fallback)
}

func normalizeEmptyDefaultsValue(value resource.Value) resource.Value {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return map[string]any{}
	}
	if normalized == nil {
		return map[string]any{}
	}
	objectValue, ok := normalized.(map[string]any)
	if ok && len(objectValue) == 0 {
		return map[string]any{}
	}
	return normalized
}

func collectionPathFor(logicalPath string) string {
	if strings.TrimSpace(logicalPath) == "/" {
		return "/"
	}
	parent := path.Dir(strings.TrimSpace(logicalPath))
	if parent == "." || parent == "" {
		return "/"
	}
	return parent
}

func joinLogicalPath(base string, segment string) string {
	trimmedBase := strings.TrimSpace(base)
	trimmedSegment := strings.Trim(strings.TrimSpace(segment), "/")
	if trimmedBase == "" || trimmedBase == "/" {
		return "/" + trimmedSegment
	}
	return strings.TrimSuffix(trimmedBase, "/") + "/" + trimmedSegment
}
