package defaults

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/google/uuid"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies = appdeps.Dependencies

type InferRequest struct {
	ManagedServer bool
}

type CheckRequest struct {
	ManagedServer bool
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
	logicalPath       string
	resourceContent   resource.Content
	defaultsContent   resource.Content
	defaultsFound     bool
	payloadDescriptor resource.PayloadDescriptor
}

type localResourceResolver interface {
	ResolveLocalResource(ctx context.Context, logicalPath string) (resource.Resource, error)
}

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
		ResolvedPath: resolvedTarget.logicalPath,
		Content:      content,
	}, nil
}

func Save(ctx context.Context, deps Dependencies, logicalPath string, content resource.Content) (Result, error) {
	resolvedTarget, err := resolveTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	defaultsStore, err := requireDefaultsStore(deps)
	if err != nil {
		return Result{}, err
	}

	content.Descriptor = chooseDefaultsDescriptor(content.Descriptor, resolvedTarget.payloadDescriptor)
	if err := defaultsStore.SaveDefaults(ctx, resolvedTarget.logicalPath, content); err != nil {
		return Result{}, err
	}

	return Result{
		ResolvedPath: resolvedTarget.logicalPath,
		Content: resource.Content{
			Value:      normalizeEmptyDefaultsValue(content.Value),
			Descriptor: content.Descriptor,
		},
	}, nil
}

func Infer(ctx context.Context, deps Dependencies, logicalPath string, request InferRequest) (Result, error) {
	resolvedTarget, err := resolveTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	var inferred resource.Value
	if request.ManagedServer {
		inferred, err = inferFromManagedServer(ctx, deps, resolvedTarget)
	} else {
		inferred, err = inferFromRepository(ctx, deps, resolvedTarget)
	}
	if err != nil {
		return Result{}, err
	}

	return Result{
		ResolvedPath: resolvedTarget.logicalPath,
		Content: resource.Content{
			Value:      inferred,
			Descriptor: resolvedTarget.payloadDescriptor,
		},
	}, nil
}

func Check(ctx context.Context, deps Dependencies, logicalPath string, request CheckRequest) (CheckResult, error) {
	inferred, err := Infer(ctx, deps, logicalPath, InferRequest{ManagedServer: request.ManagedServer})
	if err != nil {
		return CheckResult{}, err
	}

	current, err := Get(ctx, deps, inferred.ResolvedPath)
	if err != nil {
		return CheckResult{}, err
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
		ResolvedPath:    inferred.ResolvedPath,
		InferredContent: inferred.Content,
		CurrentContent:  current.Content,
		Matches:         reflect.DeepEqual(currentNormalized, inferredNormalized),
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

	defaultsStore, err := requireDefaultsStore(deps)
	if err != nil {
		return resource.Content{}, false, err
	}

	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return resource.Content{}, false, err
	}
	if resolver, ok := orchestratorService.(localResourceResolver); ok {
		item, resolveErr := resolver.ResolveLocalResource(ctx, resolvedPath)
		if resolveErr == nil && strings.TrimSpace(item.LogicalPath) != "" {
			resolvedPath = item.LogicalPath
		} else if resolveErr != nil && !faults.IsCategory(resolveErr, faults.NotFoundError) {
			return resource.Content{}, false, resolveErr
		}
	}

	defaultsContent, defaultsFound, err := readDefaultsContent(ctx, defaultsStore, resolvedPath)
	if err != nil {
		return resource.Content{}, false, err
	}
	if !defaultsFound {
		return content, false, nil
	}
	if err := resource.ValidateDefaultsSidecarValue(defaultsContent.Value); err != nil {
		return resource.Content{}, false, err
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
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return target{}, err
	}
	if normalizedPath == "/" {
		return target{}, faults.NewValidationError("logical path must target a resource, not root", nil)
	}

	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return target{}, err
	}

	defaultsStore, err := requireDefaultsStore(deps)
	if err != nil {
		return target{}, err
	}

	resourceContent, resolvedPath, err := resolveTargetResourceContent(ctx, orchestratorService, logicalPath, normalizedPath)
	if err != nil {
		return target{}, err
	}
	if resolver, ok := orchestratorService.(localResourceResolver); ok {
		item, resolveErr := resolver.ResolveLocalResource(ctx, resolvedPath)
		if resolveErr == nil && strings.TrimSpace(item.LogicalPath) != "" {
			resolvedPath = item.LogicalPath
			resourceContent = resource.Content{
				Value:      item.Payload,
				Descriptor: item.PayloadDescriptor,
			}
		} else if resolveErr != nil && !faults.IsCategory(resolveErr, faults.NotFoundError) {
			return target{}, resolveErr
		}
	}

	defaultsContent, defaultsFound, err := readDefaultsContent(ctx, defaultsStore, resolvedPath)
	if err != nil {
		return target{}, err
	}

	payloadDescriptor, err := resolveDefaultsPayloadDescriptor(ctx, deps, resolvedPath, defaultsContent, defaultsFound, resourceContent)
	if err != nil {
		return target{}, err
	}
	if err := validateDefaultsDescriptor(payloadDescriptor); err != nil {
		return target{}, err
	}
	if !resource.IsPayloadDescriptorExplicit(resourceContent.Descriptor) {
		resourceContent.Descriptor = payloadDescriptor
	}
	if defaultsFound && !resource.IsPayloadDescriptorExplicit(defaultsContent.Descriptor) {
		defaultsContent.Descriptor = payloadDescriptor
	}

	return target{
		logicalPath:       resolvedPath,
		resourceContent:   resourceContent,
		defaultsContent:   defaultsContent,
		defaultsFound:     defaultsFound,
		payloadDescriptor: payloadDescriptor,
	}, nil
}

func resolveTargetResourceContent(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	rawPath string,
	normalizedPath string,
) (resource.Content, string, error) {
	resourceContent, err := resolveLocalResourceContent(ctx, orchestratorService, normalizedPath)
	if err == nil {
		return resourceContent, normalizedPath, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) || !resource.HasExplicitCollectionTarget(rawPath) {
		return resource.Content{}, "", err
	}
	return resolveSingleCollectionTargetResourceContent(ctx, orchestratorService, normalizedPath)
}

func resolveSingleCollectionTargetResourceContent(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	collectionPath string,
) (resource.Content, string, error) {
	items, err := orchestratorService.ListLocal(ctx, collectionPath, orchestratordomain.ListPolicy{})
	if err != nil {
		return resource.Content{}, "", err
	}

	resolvedPath := ""
	for _, item := range items {
		candidatePath := strings.TrimSpace(item.LogicalPath)
		if candidatePath == "" {
			continue
		}
		if _, ok := resource.ChildSegment(collectionPath, candidatePath); !ok {
			continue
		}
		if resolvedPath != "" && candidatePath != resolvedPath {
			return resource.Content{}, "", faults.NewValidationError(
				fmt.Sprintf("logical path %q must target a concrete resource path", collectionPath),
				nil,
			)
		}
		resolvedPath = candidatePath
	}

	if resolvedPath == "" {
		return resource.Content{}, "", faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource %q not found", collectionPath),
			nil,
		)
	}

	resourceContent, err := resolveLocalResourceContent(ctx, orchestratorService, resolvedPath)
	if err != nil {
		return resource.Content{}, "", err
	}
	return resourceContent, resolvedPath, nil
}

func requireDefaultsStore(deps Dependencies) (repository.ResourceDefaultsStore, error) {
	store, err := appdeps.RequireResourceStore(deps)
	if err != nil {
		return nil, err
	}
	defaultsStore, ok := store.(repository.ResourceDefaultsStore)
	if !ok {
		return nil, faults.NewValidationError("resource defaults are not supported by the configured repository", nil)
	}
	return defaultsStore, nil
}

func resolveLocalResourceContent(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
) (resource.Content, error) {
	content, err := orchestratorService.GetLocal(ctx, logicalPath)
	if err != nil {
		return resource.Content{}, err
	}
	return content, nil
}

func readDefaultsContent(
	ctx context.Context,
	store repository.ResourceDefaultsStore,
	logicalPath string,
) (resource.Content, bool, error) {
	content, err := store.GetDefaults(ctx, logicalPath)
	if err == nil {
		return content, true, nil
	}
	if faults.IsCategory(err, faults.NotFoundError) {
		return resource.Content{}, false, nil
	}
	return resource.Content{}, false, err
}

func resolveDefaultsPayloadDescriptor(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	defaultsContent resource.Content,
	defaultsFound bool,
	resourceContent resource.Content,
) (resource.PayloadDescriptor, error) {
	if defaultsFound && resource.IsPayloadDescriptorExplicit(defaultsContent.Descriptor) {
		return resource.NormalizePayloadDescriptor(defaultsContent.Descriptor), nil
	}
	if resource.IsPayloadDescriptorExplicit(resourceContent.Descriptor) {
		return resource.NormalizePayloadDescriptor(resourceContent.Descriptor), nil
	}

	metadataService := deps.MetadataService()
	if metadataService != nil {
		md, err := metadataService.ResolveForPath(ctx, logicalPath)
		if err != nil {
			return resource.PayloadDescriptor{}, err
		}
		payloadType, err := metadata.EffectivePayloadType(md, resource.PayloadTypeJSON)
		if err != nil {
			return resource.PayloadDescriptor{}, err
		}
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: payloadType}), nil
	}

	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}), nil
}

func validateDefaultsDescriptor(descriptor resource.PayloadDescriptor) error {
	resolved := resource.NormalizePayloadDescriptor(descriptor)
	if resource.SupportsDefaultsOverlayPayloadType(resolved.PayloadType) {
		return nil
	}
	return faults.NewValidationError(
		fmt.Sprintf(
			"resource defaults are supported only for merge-capable payload types (json, yaml, ini, properties); got %q",
			resolved.PayloadType,
		),
		nil,
	)
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
			deleteErr := orchestratorService.Delete(ctx, tempPaths[idx], orchestratordomain.DeletePolicy{})
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

	firstRemote, err := orchestratorService.GetRemote(ctx, firstPath)
	if err != nil {
		return nil, err
	}
	secondRemote, err := orchestratorService.GetRemote(ctx, secondPath)
	if err != nil {
		return nil, err
	}

	inputs := []resource.Value{
		resolveSecretsForManagedServerProbe(ctx, deps.SecretProvider(), firstPath, firstPayload),
		resolveSecretsForManagedServerProbe(ctx, deps.SecretProvider(), secondPath, secondPayload),
	}
	outputs := []resource.Value{firstRemote.Value, secondRemote.Value}
	return resource.InferCreatedDefaults(inputs, outputs)
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

	if aliasOK {
		next, err = resource.SetJSONPointerValue(next, aliasPointer, tempName)
		if err != nil {
			return resource.Content{}, "", err
		}
	}
	if idOK {
		next, err = resource.SetJSONPointerValue(next, idPointer, tempName)
		if err != nil {
			return resource.Content{}, "", err
		}
	}

	nextPayload, ok := next.(map[string]any)
	if !ok {
		return resource.Content{}, "", faults.NewValidationError("managed-server defaults inference requires an object payload", nil)
	}

	return resource.Content{
		Value:      nextPayload,
		Descriptor: content.Descriptor,
	}, joinLogicalPath(collectionPathFor(logicalPath), tempName), nil
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
