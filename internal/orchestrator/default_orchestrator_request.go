package orchestrator

import (
	"context"
	"maps"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *Orchestrator) Request(
	ctx context.Context,
	requestSpec managedserver.RequestSpec,
) (resource.Content, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Content{}, err
	}

	resolvedRequest, requestCtx, err := r.resolveRequestSpec(ctx, requestSpec)
	if err != nil {
		return resource.Content{}, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator request method=%q path=%q resolved_path=%q has_body=%t",
		requestSpec.Method,
		requestSpec.Path,
		resolvedRequest.Path,
		requestBodyPresent(resolvedRequest.Body),
	)

	value, err := serverManager.Request(requestCtx, resolvedRequest)
	if err != nil {
		if fallbackValue, handled, fallbackErr := r.retryRequestResolvedMutationWithLiteralPath(
			requestCtx,
			serverManager,
			requestSpec,
			resolvedRequest,
			err,
		); handled {
			if fallbackErr != nil {
				debugctx.Printf(
					ctx,
					"orchestrator request literal fallback failed method=%q path=%q resolved_path=%q error=%v",
					requestSpec.Method,
					requestSpec.Path,
					resolvedRequest.Path,
					fallbackErr,
				)
				return resource.Content{}, fallbackErr
			}
			debugctx.Printf(
				ctx,
				"orchestrator request literal fallback succeeded method=%q path=%q resolved_path=%q value_type=%T",
				requestSpec.Method,
				requestSpec.Path,
				resolvedRequest.Path,
				fallbackValue,
			)
			return fallbackValue, nil
		}

		if faults.IsCategory(err, faults.NotFoundError) {
			fallbackValue, handled, fallbackErr := r.retryRequestNotFoundWithMetadata(
				requestCtx,
				serverManager,
				requestSpec,
			)
			if handled {
				if fallbackErr != nil {
					debugctx.Printf(
						ctx,
						"orchestrator request fallback failed method=%q path=%q resolved_path=%q error=%v",
						requestSpec.Method,
						requestSpec.Path,
						resolvedRequest.Path,
						fallbackErr,
					)
					return resource.Content{}, fallbackErr
				}
				debugctx.Printf(
					ctx,
					"orchestrator request fallback succeeded method=%q path=%q resolved_path=%q value_type=%T",
					requestSpec.Method,
					requestSpec.Path,
					resolvedRequest.Path,
					fallbackValue,
				)
				return fallbackValue, nil
			}
		}
		debugctx.Printf(
			ctx,
			"orchestrator request failed method=%q path=%q resolved_path=%q error=%v",
			requestSpec.Method,
			requestSpec.Path,
			resolvedRequest.Path,
			err,
		)
		return resource.Content{}, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator request succeeded method=%q path=%q resolved_path=%q value_type=%T",
		requestSpec.Method,
		requestSpec.Path,
		resolvedRequest.Path,
		value,
	)
	return value, nil
}

func (r *Orchestrator) retryRequestResolvedMutationWithLiteralPath(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	original managedserver.RequestSpec,
	resolved managedserver.RequestSpec,
	requestErr error,
) (resource.Content, bool, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(original.Method))
	if normalizedMethod != "PUT" && normalizedMethod != "PATCH" {
		return resource.Content{}, false, nil
	}
	if !faults.IsCategory(requestErr, faults.NotFoundError) {
		return resource.Content{}, false, nil
	}
	if sameNormalizedRequestPath(original.Path, resolved.Path) {
		return resource.Content{}, false, nil
	}

	fallback := cloneRequestSpec(resolved)
	fallback.Path = original.Path
	value, err := serverManager.Request(ctx, fallback)
	return value, true, err
}

func (r *Orchestrator) retryRequestNotFoundWithMetadata(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	requestSpec managedserver.RequestSpec,
) (resource.Content, bool, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(requestSpec.Method))

	switch normalizedMethod {
	case "GET":
		value, err := r.GetRemote(ctx, requestSpec.Path)
		return value, true, err
	case "DELETE":
		resolvedResource, resourceMd, err := r.buildResourceInfoForRemoteRead(ctx, requestSpec.Path)
		if err != nil {
			return resource.Content{}, true, err
		}

		remoteValue, err := r.fetchRemoteValue(ctx, resolvedResource, resourceMd)
		if err != nil {
			return resource.Content{}, true, err
		}

		normalizedPayload, err := resource.Normalize(remoteValue.Value)
		if err != nil {
			return resource.Content{}, true, err
		}
		resolvedResource.Payload = normalizedPayload
		resolvedResource.PayloadDescriptor = remoteValue.Descriptor

		localAlias, remoteID, err := resolveResourceIdentity(
			resolvedResource.LogicalPath,
			resourceMd,
			normalizedPayload,
		)
		if err != nil {
			return resource.Content{}, true, err
		}
		resolvedResource.LocalAlias = localAlias
		resolvedResource.RemoteID = remoteID

		spec, err := r.renderOperationSpec(ctx, resolvedResource, resourceMd, metadata.OperationDelete, resolvedResource.Payload)
		if err != nil {
			return resource.Content{}, true, err
		}

		fallback := requestSpecFromOperationSpec(requestSpec, spec)
		value, err := serverManager.Request(ctx, fallback)
		return value, true, err
	default:
		return resource.Content{}, false, nil
	}
}

func (r *Orchestrator) resolveRequestSpec(
	ctx context.Context,
	requestSpec managedserver.RequestSpec,
) (managedserver.RequestSpec, context.Context, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(requestSpec.Method))

	operation, ok := requestMetadataOperation(normalizedMethod)
	if !ok {
		resolved := cloneRequestSpec(requestSpec)
		resolved.Method = normalizedMethod
		resolved.Path = normalizeRequestPathForOrchestrator(resolved.Path)
		return resolved, ctx, nil
	}

	resolvedResource, resourceMd, err := r.buildResourceInfoForRemoteRead(ctx, requestSpec.Path)
	if err != nil {
		return managedserver.RequestSpec{}, ctx, err
	}

	if normalizedMethod == "GET" && shouldResolveRequestGetAsList(resolvedResource, resourceMd) {
		operation = metadata.OperationList
	}

	// Collection-target operations should preserve the requested logical path as
	// the default collection path when metadata does not override it.
	if operation == metadata.OperationCreate || operation == metadata.OperationList {
		resolvedResource.CollectionPath = resolvedResource.LogicalPath
	}

	if requestBodyPresent(requestSpec.Body) {
		normalizedBody, normalizeErr := resource.Normalize(requestSpec.Body.Value)
		if normalizeErr != nil {
			return managedserver.RequestSpec{}, ctx, normalizeErr
		}
		resolvedResource.Payload = normalizedBody
		resolvedResource.PayloadDescriptor = requestSpec.Body.Descriptor
	}

	spec, err := r.renderOperationSpec(ctx, resolvedResource, resourceMd, operation, resolvedResource.Payload)
	if err != nil {
		return managedserver.RequestSpec{}, ctx, err
	}

	resolved := requestSpecFromOperationSpec(requestSpec, spec)

	ctxWithValidation := metadata.WithRequestOperationValidation(
		ctx,
		operation,
		metadata.ResourceOperationSpecInput{
			LogicalPath:    resolvedResource.LogicalPath,
			CollectionPath: resolvedResource.CollectionPath,
			LocalAlias:     resolvedResource.LocalAlias,
			RemoteID:       resolvedResource.RemoteID,
			Metadata:       resourceMd,
			Payload:        resolvedResource.Payload,
		},
		spec.Validate,
	)

	return resolved, ctxWithValidation, nil
}

func requestMetadataOperation(method string) (metadata.Operation, bool) {
	switch method {
	case "GET":
		return metadata.OperationGet, true
	case "POST":
		return metadata.OperationCreate, true
	case "PUT", "PATCH":
		return metadata.OperationUpdate, true
	case "DELETE":
		return metadata.OperationDelete, true
	default:
		return "", false
	}
}

func shouldResolveRequestGetAsList(resolvedResource resource.Resource, md metadata.ResourceMetadata) bool {
	if strings.TrimSpace(md.CollectionPath) == "" {
		return false
	}

	if md.Operations != nil {
		if getSpec, ok := md.Operations[string(metadata.OperationGet)]; ok && strings.TrimSpace(getSpec.Path) != "" {
			return false
		}
	}

	templateDepth := len(resource.SplitRawPathSegments(md.CollectionPath))
	if templateDepth == 0 {
		return false
	}

	logicalDepth := len(resource.SplitRawPathSegments(resolvedResource.LogicalPath))
	if logicalDepth == 0 {
		return false
	}

	// Selector-depth logical paths (for example /.../user-registry) often map to
	// remote collection endpoints via metadata collectionPath overrides. For
	// request GET, prefer the list path in this case so the raw request targets
	// the correct collection endpoint (for example /components) before fallback.
	return logicalDepth <= templateDepth
}

func sameNormalizedRequestPath(first string, second string) bool {
	normalizedFirst, errFirst := resource.NormalizeLogicalPath(first)
	normalizedSecond, errSecond := resource.NormalizeLogicalPath(second)
	if errFirst == nil && errSecond == nil {
		return normalizedFirst == normalizedSecond
	}

	return strings.TrimSpace(first) == strings.TrimSpace(second)
}

func requestSpecFromOperationSpec(base managedserver.RequestSpec, spec metadata.OperationSpec) managedserver.RequestSpec {
	resolved := managedserver.RequestSpec{
		Method:      strings.ToUpper(strings.TrimSpace(spec.Method)),
		Path:        spec.Path,
		Query:       maps.Clone(spec.Query),
		Headers:     maps.Clone(spec.Headers),
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        requestBodyContent(spec.Body, spec.ContentType),
	}
	if resolved.Method == "" {
		resolved.Method = strings.ToUpper(strings.TrimSpace(base.Method))
	}
	if strings.TrimSpace(base.Accept) != "" {
		resolved.Accept = base.Accept
	}
	if strings.TrimSpace(base.ContentType) != "" {
		resolved.ContentType = base.ContentType
	}
	if len(base.Query) > 0 {
		if resolved.Query == nil {
			resolved.Query = map[string]string{}
		}
		for key, value := range base.Query {
			resolved.Query[key] = value
		}
	}
	if len(base.Headers) > 0 {
		if resolved.Headers == nil {
			resolved.Headers = map[string]string{}
		}
		for key, value := range base.Headers {
			resolved.Headers[key] = value
		}
	}
	if requestBodyPresent(base.Body) && !requestBodyPresent(resolved.Body) {
		resolved.Body = base.Body
	}
	return resolved
}

func cloneRequestSpec(value managedserver.RequestSpec) managedserver.RequestSpec {
	return managedserver.RequestSpec{
		Method:      value.Method,
		Path:        value.Path,
		Query:       maps.Clone(value.Query),
		Headers:     maps.Clone(value.Headers),
		Accept:      value.Accept,
		ContentType: value.ContentType,
		Body:        value.Body,
	}
}

func normalizeRequestPathForOrchestrator(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if trimmed != "/" {
		trimmed = strings.TrimSuffix(trimmed, "/")
	}
	return trimmed
}

func requestBodyContent(value any, contentType string) resource.Content {
	if content, ok := value.(resource.Content); ok {
		if !resource.IsPayloadDescriptorExplicit(content.Descriptor) && strings.TrimSpace(contentType) != "" {
			content.Descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
				MediaType: contentType,
			})
		}
		return content
	}
	if value == nil {
		return resource.Content{}
	}
	descriptor := resource.PayloadDescriptor{}
	if strings.TrimSpace(contentType) != "" {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			MediaType: contentType,
		})
	}
	return resource.Content{
		Value:      value,
		Descriptor: descriptor,
	}
}

func (r *Orchestrator) GetOpenAPISpec(ctx context.Context) (resource.Content, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Content{}, err
	}
	return serverManager.GetOpenAPISpec(ctx)
}
