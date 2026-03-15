package fsmetadata

import (
	"context"
	"path"
	"sort"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

func (s *FSMetadataService) RenderOperationSpec(
	ctx context.Context,
	logicalPath string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	debugctx.Printf(
		ctx,
		"metadata fs render start logical_path=%q operation=%q value_type=%T",
		logicalPath,
		operation,
		value,
	)

	target, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render invalid logical_path=%q operation=%q error=%v",
			logicalPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs render normalized logical_path=%q normalized=%q operation=%q",
		logicalPath,
		target.path,
		operation,
	)

	resolved, err := s.resolveForPathWithContext(ctx, logicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render resolve failed logical_path=%q operation=%q error=%v",
			target.path,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	templateValue, err := buildTemplateValue(target, resolved, value, operation)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render template-value failed logical_path=%q operation=%q error=%v",
			target.path,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	metadatadomain.ApplyPayloadTemplateScope(templateValue, resolved.metadata, value, resource.PayloadDescriptor{})

	spec, err := metadatadomain.ResolveOperationSpecWithScope(ctx, resolved.metadata, operation, templateValue)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render failed logical_path=%q operation=%q error=%v",
			target.path,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs render done logical_path=%q operation=%q resolved_path=%q",
		logicalPath,
		operation,
		spec.Path,
	)
	return spec, nil
}

func (s *FSMetadataService) RenderMetadataSnapshot(
	ctx context.Context,
	logicalPath string,
	payload resource.Value,
	descriptor resource.PayloadDescriptor,
) (metadatadomain.ResourceMetadata, error) {
	target, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	resolved, err := s.resolveForPathWithContext(ctx, logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	resolved.metadata = metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		resolved.metadata,
	)

	return renderMetadataSnapshotWithResolvedContext(
		ctx,
		target,
		resolved,
		payload,
		descriptor,
	)
}

func (s *FSMetadataService) RenderOperationSpecForResource(
	ctx context.Context,
	input metadatadomain.ResourceOperationSpecInput,
	operation metadatadomain.Operation,
) (metadatadomain.OperationSpec, error) {
	resolvedResource := resource.Resource{
		LogicalPath:       input.LogicalPath,
		CollectionPath:    input.CollectionPath,
		LocalAlias:        input.LocalAlias,
		RemoteID:          input.RemoteID,
		Payload:           input.Payload,
		PayloadDescriptor: input.PayloadDescriptor,
	}

	debugctx.Printf(
		ctx,
		"metadata fs render-resource start logical_path=%q operation=%q payload_type=%T",
		resolvedResource.LogicalPath,
		operation,
		resolvedResource.Payload,
	)

	target, err := normalizeResolvePath(resolvedResource.LogicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource invalid logical_path=%q operation=%q error=%v",
			resolvedResource.LogicalPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	resolved, resolveErr := s.resolveForPathWithContext(ctx, resolvedResource.LogicalPath)
	if resolveErr != nil {
		if faults.IsCategory(resolveErr, faults.NotFoundError) {
			resolved = resolvedMetadataResult{}
		} else {
			debugctx.Printf(
				ctx,
				"metadata fs render-resource resolve failed logical_path=%q operation=%q error=%v",
				target.path,
				operation,
				resolveErr,
			)
			return metadatadomain.OperationSpec{}, resolveErr
		}
	}
	if metadatadomain.HasResourceMetadataDirectives(input.Metadata) {
		resolved.metadata = metadatadomain.CloneResourceMetadata(input.Metadata)
	}

	templateScope, err := buildTemplateScopeForResource(target, resolved, resolvedResource, operation)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource template-scope failed logical_path=%q operation=%q error=%v",
			target.path,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	metadatadomain.ApplyPayloadTemplateScope(
		templateScope,
		resolved.metadata,
		resolvedResource.Payload,
		resolvedResource.PayloadDescriptor,
	)

	spec, err := metadatadomain.ResolveOperationSpecWithScope(ctx, resolved.metadata, operation, templateScope)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource failed logical_path=%q operation=%q error=%v",
			target.path,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	debugctx.Printf(
		ctx,
		"metadata fs render-resource done logical_path=%q operation=%q resolved_path=%q",
		target.path,
		operation,
		spec.Path,
	)
	return spec, nil
}

func buildTemplateValue(
	target resolvedPathTarget,
	resolved resolvedMetadataResult,
	value any,
	operation metadatadomain.Operation,
) (map[string]any, error) {
	effectiveTarget := targetForOperation(target, operation)
	normalizedPayload, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	alias, remoteID, err := resolveTemplateScopeIdentity(
		target.path,
		resolved.metadata,
		normalizedPayload,
		"",
		"",
		operation,
	)
	if err != nil {
		return nil, err
	}

	scope, err := templatescope.BuildResourceScopeWithOptions(resource.Resource{
		LogicalPath:    target.path,
		CollectionPath: collectionPathForTarget(effectiveTarget),
		LocalAlias:     alias,
		RemoteID:       remoteID,
		Payload:        normalizedPayload,
	}, resolved.metadata, scopeOptionsForResolvedTarget(resolved.descendant))
	if err != nil {
		return nil, err
	}
	applyDescendantScope(scope, effectiveTarget, resolved.descendant)
	return scope, nil
}

func buildTemplateScopeForResource(
	target resolvedPathTarget,
	resolved resolvedMetadataResult,
	resolvedResource resource.Resource,
	operation metadatadomain.Operation,
) (map[string]any, error) {
	effectiveTarget := targetForOperation(target, operation)
	normalizedPayload, err := resource.Normalize(resolvedResource.Payload)
	if err != nil {
		return nil, err
	}

	collectionPath := strings.TrimSpace(resolvedResource.CollectionPath)
	if collectionPath == "" {
		collectionPath = collectionPathForTarget(effectiveTarget)
	} else {
		collectionPath, err = resource.NormalizeLogicalPath(collectionPath)
		if err != nil {
			return nil, err
		}
	}

	localAlias, remoteID, err := resolveTemplateScopeIdentity(
		target.path,
		resolved.metadata,
		normalizedPayload,
		resolvedResource.LocalAlias,
		resolvedResource.RemoteID,
		operation,
	)
	if err != nil {
		return nil, err
	}

	scope, err := templatescope.BuildResourceScopeWithOptions(resource.Resource{
		LogicalPath:    target.path,
		CollectionPath: collectionPath,
		LocalAlias:     localAlias,
		RemoteID:       remoteID,
		Payload:        normalizedPayload,
	}, resolved.metadata, scopeOptionsForResolvedTarget(resolved.descendant))
	if err != nil {
		return nil, err
	}
	applyDescendantScope(scope, effectiveTarget, resolved.descendant)
	return scope, nil
}

func resolveTemplateScopeIdentity(
	logicalPath string,
	resolvedMetadata metadatadomain.ResourceMetadata,
	normalizedPayload any,
	localAlias string,
	remoteID string,
	operation metadatadomain.Operation,
) (string, string, error) {
	alias := strings.TrimSpace(localAlias)
	id := strings.TrimSpace(remoteID)

	if (alias == "" || id == "") && payloadCanResolveTemplateIdentity(normalizedPayload) {
		derivedAlias, derivedRemoteID, err := identity.ResolveAliasAndRemoteID(
			logicalPath,
			resolvedMetadata,
			normalizedPayload,
		)
		if err != nil {
			return "", "", err
		}
		if alias == "" {
			alias = derivedAlias
		}
		if id == "" {
			id = derivedRemoteID
		}
	}

	if operationNeedsFallbackIdentity(operation) {
		if alias == "" {
			alias = aliasForTemplateScopeLogicalPath(logicalPath)
		}
		if id == "" {
			id = alias
		}
	}

	return alias, id, nil
}

func payloadCanResolveTemplateIdentity(value any) bool {
	payloadMap, ok := value.(map[string]any)
	return ok && len(payloadMap) > 0
}

func operationNeedsFallbackIdentity(operation metadatadomain.Operation) bool {
	switch operation {
	case metadatadomain.OperationGet,
		metadatadomain.OperationUpdate,
		metadatadomain.OperationDelete,
		metadatadomain.OperationCompare:
		return true
	default:
		return false
	}
}

func aliasForTemplateScopeLogicalPath(logicalPath string) string {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	return path.Base(trimmed)
}

func collectionPathForTarget(target resolvedPathTarget) string {
	if target.collection {
		return target.path
	}
	return collectionPathForLogicalPath(target.path)
}

func targetForOperation(
	target resolvedPathTarget,
	operation metadatadomain.Operation,
) resolvedPathTarget {
	if target.collection || operation != metadatadomain.OperationList {
		return target
	}
	return resolvedPathTarget{
		path:       target.path,
		collection: true,
	}
}

func scopeOptionsForResolvedTarget(
	descendant *descendantRuntimeContext,
) templatescope.ResourceScopeOptions {
	if descendant == nil || strings.TrimSpace(descendant.matchedCollectionPath) == "" {
		return templatescope.ResourceScopeOptions{}
	}
	return templatescope.ResourceScopeOptions{
		DerivedCollectionPath: descendant.matchedCollectionPath,
	}
}

func applyDescendantScope(
	scope map[string]any,
	target resolvedPathTarget,
	descendant *descendantRuntimeContext,
) {
	if descendant == nil || len(scope) == 0 {
		return
	}

	scope["descendantCollectionPath"] = descendantSuffix(
		descendant.matchedCollectionPath,
		collectionPathForTarget(target),
	)
	scope["descendantPath"] = descendantSuffix(
		descendant.matchedCollectionPath,
		target.path,
	)
}

func descendantSuffix(root string, candidate string) string {
	trimmedRoot := strings.TrimSpace(root)
	trimmedCandidate := strings.TrimSpace(candidate)
	if trimmedRoot == "" || trimmedCandidate == "" {
		return ""
	}
	if trimmedRoot == trimmedCandidate {
		return ""
	}
	if trimmedRoot == "/" {
		if trimmedCandidate == "/" {
			return ""
		}
		return trimmedCandidate
	}
	prefix := trimmedRoot + "/"
	if !strings.HasPrefix(trimmedCandidate, prefix) {
		return ""
	}
	return trimmedCandidate[len(trimmedRoot):]
}

func renderMetadataSnapshotWithResolvedContext(
	ctx context.Context,
	target resolvedPathTarget,
	resolved resolvedMetadataResult,
	payload resource.Value,
	descriptor resource.PayloadDescriptor,
) (metadatadomain.ResourceMetadata, error) {
	scope, err := buildTemplateScopeForResource(target, resolved, resource.Resource{
		LogicalPath:       target.path,
		CollectionPath:    collectionPathForTarget(target),
		Payload:           payload,
		PayloadDescriptor: descriptor,
	}, metadatadomain.OperationGet)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	metadatadomain.ApplyPayloadTemplateScope(scope, resolved.metadata, payload, descriptor)

	rendered := metadatadomain.CloneResourceMetadata(resolved.metadata)
	rendered.RemoteCollectionPath, err = renderRemoteCollectionPath(rendered.RemoteCollectionPath, scope)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	operationNames := make([]string, 0, len(rendered.Operations))
	for key := range rendered.Operations {
		operationNames = append(operationNames, key)
	}
	sort.Strings(operationNames)

	renderedOperations := make(map[string]metadatadomain.OperationSpec, len(operationNames))
	for _, key := range operationNames {
		spec, err := metadatadomain.ResolveOperationSpecWithScope(
			ctx,
			resolved.metadata,
			metadatadomain.Operation(key),
			scope,
		)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
		renderedOperations[key] = spec
	}
	rendered.Operations = renderedOperations

	return rendered, nil
}

func renderRemoteCollectionPath(raw string, scope map[string]any) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", nil
	}
	rendered, err := metadatadomain.RenderTemplateString("remoteCollectionPath", candidate, scope)
	if err != nil {
		return "", err
	}
	return metadatadomain.NormalizeRenderedOperationPath(rendered), nil
}
