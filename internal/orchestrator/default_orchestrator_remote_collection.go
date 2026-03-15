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

package orchestrator

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func (r *Orchestrator) fetchRemoteCollectionValue(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
) (resource.Content, bool, error) {
	if !r.shouldTreatRemotePathAsCollection(ctx, serverManager, resolvedResource) {
		return resource.Content{}, false, nil
	}

	items, err := r.listRemoteResources(ctx, serverManager, resolvedResource.LogicalPath, md)
	if err != nil {
		// Some APIs incorrectly return 404 for empty collections.
		if faults.IsCategory(err, faults.NotFoundError) {
			if r.isMissingParentForCollectionNotFound(ctx, serverManager, resolvedResource) {
				return resource.Content{}, true, err
			}
			return resource.Content{
				Value:      []any{},
				Descriptor: resolvedResource.PayloadDescriptor,
			}, true, nil
		}
		if isFallbackListPayloadShapeError(err) {
			return resource.Content{}, false, nil
		}
		return resource.Content{}, true, err
	}

	descriptor := resolvedResource.PayloadDescriptor
	if len(items) > 0 && resource.IsPayloadDescriptorExplicit(items[0].PayloadDescriptor) {
		descriptor = items[0].PayloadDescriptor
	}
	return resource.Content{
		Value:      listPayloadFromResources(items),
		Descriptor: descriptor,
	}, true, nil
}

func (r *Orchestrator) withListJQResourceResolver(ctx context.Context) context.Context {
	return managedserver.WithListJQResourceResolver(ctx, r.resolveListJQResource)
}

func (r *Orchestrator) listRemoteResources(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	collectionPath string,
	md metadata.ResourceMetadata,
) ([]resource.Resource, error) {
	return serverManager.List(r.withListJQResourceResolver(ctx), collectionPath, md)
}

func (r *Orchestrator) resolveListJQResource(
	ctx context.Context,
	logicalPath string,
) (resource.Value, error) {
	content, err := r.GetRemote(ctx, logicalPath)
	if err != nil {
		return nil, err
	}
	return content.Value, nil
}

func (r *Orchestrator) shouldTreatRemotePathAsCollection(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
) bool {
	if r.collectionHintFromRepository(ctx, resolvedResource.LogicalPath) {
		return true
	}

	return r.collectionHintFromOpenAPI(ctx, serverManager, resolvedResource)
}

func (r *Orchestrator) listOperationTargetsLogicalPath(
	ctx context.Context,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
) bool {
	normalizedPath, ok := r.renderedOperationPath(ctx, resolvedResource, md, metadata.OperationList)
	if !ok {
		return false
	}
	return normalizedPath == resolvedResource.LogicalPath
}

func (r *Orchestrator) collectionHintFromRepository(
	ctx context.Context,
	logicalPath string,
) bool {
	if r == nil || r.repository == nil {
		return false
	}

	exists, err := r.repository.Exists(ctx, logicalPath)
	if err != nil || !exists {
		return false
	}

	_, err = r.repository.Get(ctx, logicalPath)
	if err == nil {
		return false
	}

	return faults.IsCategory(err, faults.NotFoundError)
}

func (r *Orchestrator) collectionHintFromOpenAPI(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
) bool {
	openAPISpec, err := serverManager.GetOpenAPISpec(ctx)
	if err != nil {
		return false
	}
	existsInOpenAPI, err := metadata.HasOpenAPIPath(resolvedResource.LogicalPath, openAPISpec.Value)
	if err != nil || !existsInOpenAPI {
		return false
	}

	if r.openAPIInferenceHintsCollection(ctx, resolvedResource, resolvedResource.LogicalPath, openAPISpec.Value) {
		return true
	}

	// Avoid treating concrete resource paths with child endpoints as collections
	// (for example /admin/realms/{realm}) when probing a synthetic trailing-slash
	// variant for collection hints.
	if openAPIExactPathLooksLikeResource(openAPISpec.Value, resolvedResource.LogicalPath) {
		return false
	}

	if resolvedResource.LogicalPath == "/" {
		return false
	}

	collectionSelector := strings.TrimSuffix(resolvedResource.LogicalPath, "/") + "/"
	return r.openAPIInferenceHintsCollection(ctx, resolvedResource, collectionSelector, openAPISpec.Value)
}

func (r *Orchestrator) openAPIInferenceHintsCollection(
	ctx context.Context,
	resolvedResource resource.Resource,
	logicalPath string,
	openAPISpec any,
) bool {
	inferred, err := metadata.InferFromOpenAPISpec(ctx, logicalPath, metadata.InferenceRequest{}, openAPISpec)
	if err != nil {
		return false
	}

	hintInfo := resolvedResource
	hintInfo.Payload = buildCollectionHintPayload(resolvedResource.Payload, resolvedResource.LogicalPath, inferred)

	if !r.listOperationTargetsLogicalPath(ctx, hintInfo, inferred) {
		return false
	}

	if createPath, ok := r.renderedOperationPath(ctx, hintInfo, inferred, metadata.OperationCreate); ok && createPath == hintInfo.LogicalPath {
		return true
	}

	getPath, ok := r.renderedOperationPath(ctx, hintInfo, inferred, metadata.OperationGet)
	if !ok {
		return false
	}
	return isCollectionItemPath(hintInfo.LogicalPath, getPath)
}

func (r *Orchestrator) renderedOperationPath(
	ctx context.Context,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
) (string, bool) {
	spec, err := r.renderOperationSpec(ctx, resolvedResource, md, operation, resolvedResource.Payload)
	if err != nil {
		return "", false
	}

	normalizedPath, err := resource.NormalizeLogicalPath(spec.Path)
	if err != nil {
		return "", false
	}
	return normalizedPath, true
}

func isCollectionItemPath(collectionPath string, resourcePath string) bool {
	if collectionPath == "/" {
		return resourcePath != "/" && strings.Count(strings.Trim(resourcePath, "/"), "/") == 0
	}

	trimmedPrefix := strings.TrimSuffix(collectionPath, "/") + "/"
	return strings.HasPrefix(resourcePath, trimmedPrefix)
}

func buildCollectionHintPayload(
	basePayload resource.Value,
	logicalPath string,
	inferred metadata.ResourceMetadata,
) resource.Value {
	payload, _ := basePayload.(map[string]any)
	scope := make(map[string]any, len(payload))
	for key, value := range payload {
		scope[key] = value
	}

	for key, value := range templatescope.DerivePathTemplateFields(logicalPath, inferred) {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if _, exists := scope[key]; exists {
			continue
		}
		scope[key] = value
	}

	if len(scope) == 0 {
		return basePayload
	}
	return scope
}

func openAPIExactPathLooksLikeResource(openAPISpec any, logicalPath string) bool {
	root, ok := openAPISpec.(map[string]any)
	if !ok {
		return false
	}

	pathsValue, found := root["paths"]
	if !found {
		return false
	}
	paths, ok := pathsValue.(map[string]any)
	if !ok {
		return false
	}

	targetSegments := resource.SplitRawPathSegments(logicalPath)
	if len(targetSegments) == 0 {
		return false
	}

	for rawPath, pathItemValue := range paths {
		candidateSegments := resource.SplitRawPathSegments(rawPath)
		if len(candidateSegments) != len(targetSegments) {
			continue
		}
		if !matchesOpenAPIPathSegments(candidateSegments, targetSegments) {
			continue
		}

		pathItem, ok := pathItemValue.(map[string]any)
		if !ok {
			return false
		}

		hasGet := false
		hasPost := false
		hasResourceMutation := false
		for method := range pathItem {
			switch strings.ToLower(strings.TrimSpace(method)) {
			case "get":
				hasGet = true
			case "post":
				hasPost = true
			case "put", "patch", "delete":
				hasResourceMutation = true
			}
		}

		return hasGet && hasResourceMutation && !hasPost
	}

	return false
}

func matchesOpenAPIPathSegments(candidate []string, target []string) bool {
	if len(candidate) != len(target) {
		return false
	}

	for idx := range candidate {
		candidateSegment := strings.TrimSpace(candidate[idx])
		targetSegment := strings.TrimSpace(target[idx])
		if candidateSegment == targetSegment {
			continue
		}
		if strings.HasPrefix(candidateSegment, "{") && strings.HasSuffix(candidateSegment, "}") {
			if targetSegment == "" {
				return false
			}
			continue
		}
		return false
	}

	return true
}

func (r *Orchestrator) isMissingParentForCollectionNotFound(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
) bool {
	parentPath := path.Dir(strings.TrimSuffix(resolvedResource.LogicalPath, "/"))
	if parentPath == "." || parentPath == "" || parentPath == "/" {
		return false
	}

	parentInfo, parentMd, err := r.buildResourceInfoForRemoteRead(ctx, parentPath)
	if err != nil {
		return false
	}

	_, err = serverManager.Get(ctx, parentInfo, parentMd)
	return faults.IsCategory(err, faults.NotFoundError)
}

func listPayloadFromResources(items []resource.Resource) resource.Value {
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

func isFallbackListPayloadShapeError(err error) bool {
	return managedserver.IsListPayloadShapeError(err)
}

func (r *Orchestrator) renderOperationSpec(
	ctx context.Context,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
	value resource.Value,
) (metadata.OperationSpec, error) {
	metadataCopy := metadata.CloneResourceMetadata(md)
	templateResource := resolvedResource
	templateResource.Payload = value

	if renderer, ok := r.metadata.(metadata.ResourceOperationSpecRenderer); ok {
		spec, err := renderer.RenderOperationSpecForResource(ctx, metadata.ResourceOperationSpecInput{
			LogicalPath:       templateResource.LogicalPath,
			CollectionPath:    templateResource.CollectionPath,
			LocalAlias:        templateResource.LocalAlias,
			RemoteID:          templateResource.RemoteID,
			PayloadDescriptor: templateResource.PayloadDescriptor,
			Metadata:          metadataCopy,
			Payload:           templateResource.Payload,
		}, operation)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
		if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
			spec.Method = overrideMethod
		}
		return spec, nil
	}

	scope, err := templatescope.BuildResourceScope(templateResource, metadataCopy)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	metadata.ApplyPayloadTemplateScope(
		scope,
		metadataCopy,
		templateResource.Payload,
		templateResource.PayloadDescriptor,
	)

	spec, err := metadata.ResolveOperationSpecWithScope(ctx, metadataCopy, operation, scope)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
		spec.Method = overrideMethod
	}
	return spec, nil
}
