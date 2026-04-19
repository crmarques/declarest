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

package defaults

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

type Dependencies = appdeps.Dependencies

type InferSource string

const (
	InferSourceRepository     InferSource = "repository"
	InferSourceManagedService InferSource = "managed-service"
)

type InferRequest struct {
	Sources []InferSource
	Items   []string
	Wait    time.Duration
}

type CheckRequest struct {
	Sources []InferSource
	Items   []string
	Wait    time.Duration
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

type defaultsLocalResolver interface {
	ResolveLocalResource(ctx context.Context, logicalPath string) (resource.Resource, error)
}

type managedServiceAuthCacheInvalidator interface {
	InvalidateAuthCache()
}

const (
	managedServiceProbeReadAttempts    = 8
	managedServiceProbeReadMinAttempts = 4
	managedServiceProbeReadDelay       = 250 * time.Millisecond
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

	sources, err := normalizeInferSources(request.Sources)
	if err != nil {
		return Result{}, err
	}

	samples := make([]resource.Value, 0, 8)
	if inferSourcesInclude(sources, InferSourceRepository) {
		repositorySamples, sampleErr := inferFromRepository(ctx, deps, resolvedTarget, request)
		if sampleErr != nil {
			return Result{}, sampleErr
		}
		samples = append(samples, repositorySamples...)
	}
	if inferSourcesInclude(sources, InferSourceManagedService) {
		managedServiceSamples, sampleErr := inferFromManagedService(ctx, deps, resolvedTarget, request)
		if sampleErr != nil {
			return Result{}, sampleErr
		}
		samples = append(samples, managedServiceSamples...)
	}

	inferred, err := resource.InferDefaultsFromValues(samples...)
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
	inferred, err := Infer(ctx, deps, logicalPath, InferRequest(request))
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
		return target{}, faults.Invalid("logical path must target a resource or collection, not root", nil)
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
		return resource.Content{}, false, faults.Invalid("logical path must target a resource, not root", nil)
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
	if resolver, ok := orchestratorService.(defaultsLocalResolver); ok {
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

func normalizeInferSources(sources []InferSource) ([]InferSource, error) {
	if len(sources) == 0 {
		return []InferSource{InferSourceRepository}, nil
	}

	normalized := make([]InferSource, 0, len(sources))
	seen := make(map[InferSource]struct{}, len(sources))
	for _, source := range sources {
		trimmed := InferSource(strings.TrimSpace(string(source)))
		switch trimmed {
		case InferSourceRepository, InferSourceManagedService:
		default:
			return nil, faults.Invalid(
				"defaults inference sources must be repository and/or managed-service",
				nil,
			)
		}
		if _, found := seen[trimmed]; found {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, faults.Invalid(
			"defaults inference sources must be repository and/or managed-service",
			nil,
		)
	}
	return normalized, nil
}

func inferSourcesInclude(sources []InferSource, target InferSource) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}

func inferFromRepository(
	ctx context.Context,
	deps Dependencies,
	resolvedTarget target,
	request InferRequest,
) ([]resource.Value, error) {
	selectedItems, err := resolveInferTemplateItems(ctx, deps, resolvedTarget.scopePath, request.Items)
	if err != nil {
		return nil, err
	}

	samples := make([]resource.Value, 0, len(selectedItems))
	targetPayloadType := resource.NormalizePayloadDescriptor(resolvedTarget.payloadDescriptor).PayloadType
	for _, item := range selectedItems {
		if resource.NormalizePayloadDescriptor(item.localContent.Descriptor).PayloadType != targetPayloadType {
			continue
		}
		samples = append(samples, item.localContent.Value)
	}
	return samples, nil
}

func canonicalManagedServiceProbeFieldKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(normalized)
}

func singularManagedServiceProbeIdentityField(value string) string {
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

func resolveInferTemplateItems(
	ctx context.Context,
	deps Dependencies,
	collectionPath string,
	aliases []string,
) ([]inferTemplateItem, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	listedItems, err := orchestratorService.ListLocal(ctx, collectionPath, orchestratordomain.ListPolicy{})
	if err != nil {
		return nil, err
	}

	candidatePaths := make([]string, 0, len(listedItems))
	for _, item := range listedItems {
		candidatePath := strings.TrimSpace(item.LogicalPath)
		if candidatePath == "" {
			continue
		}
		if _, ok := resource.ChildSegment(collectionPath, candidatePath); !ok {
			continue
		}
		candidatePaths = append(candidatePaths, candidatePath)
	}
	if len(candidatePaths) == 0 {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource %q not found", collectionPath),
			nil,
		)
	}
	sort.Strings(candidatePaths)

	items := make([]inferTemplateItem, 0, len(candidatePaths))
	byAlias := make(map[string]inferTemplateItem, len(candidatePaths))
	for _, logicalPath := range candidatePaths {
		item, itemErr := resolveInferTemplateItem(ctx, deps, logicalPath)
		if itemErr != nil {
			return nil, itemErr
		}
		items = append(items, item)

		aliasKey := strings.TrimSpace(item.localAlias)
		if aliasKey == "" {
			continue
		}
		if existing, found := byAlias[aliasKey]; found && existing.logicalPath != item.logicalPath {
			return nil, faults.Conflict(
				fmt.Sprintf("multiple collection items match alias %q", aliasKey),
				nil,
			)
		}
		byAlias[aliasKey] = item
	}

	if len(aliases) == 0 {
		return items, nil
	}

	selected := make([]inferTemplateItem, 0, len(aliases))
	missing := make([]string, 0)
	for _, alias := range aliases {
		item, found := byAlias[alias]
		if !found {
			missing = append(missing, alias)
			continue
		}
		selected = append(selected, item)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, faults.Invalid(
			fmt.Sprintf("items alias not found: %s", strings.Join(missing, ", ")),
			nil,
		)
	}
	return selected, nil
}

func resolveInferTemplateItem(ctx context.Context, deps Dependencies, logicalPath string) (inferTemplateItem, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return inferTemplateItem{}, err
	}

	resolvedResource, err := resolveResolvedLocalTarget(ctx, orchestratorService, logicalPath)
	if err != nil {
		return inferTemplateItem{}, err
	}

	localAlias := strings.TrimSpace(resolvedResource.LocalAlias)
	if localAlias == "" {
		localAlias = path.Base(logicalPath)
	}

	return inferTemplateItem{
		logicalPath: logicalPath,
		localAlias:  localAlias,
		localContent: resource.Content{
			Value:      resolvedResource.Payload,
			Descriptor: resolvedResource.PayloadDescriptor,
		},
	}, nil
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
