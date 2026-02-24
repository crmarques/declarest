package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/metadata"
)

func (g *HTTPResourceServerGateway) applyOpenAPIFallback(
	ctx context.Context,
	requestPath string,
	operation metadata.Operation,
	spec *metadata.OperationSpec,
	explicitMethod bool,
	explicitAccept bool,
	explicitContentType bool,
) error {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return err
	}

	_, pathItem, ok := findOpenAPIPathItem(document, requestPath)
	if !ok {
		return nil
	}

	if !explicitMethod && strings.TrimSpace(spec.Method) == "" {
		spec.Method = inferMethodFromPathItem(pathItem, operation)
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		return nil
	}

	operationItem, found := openAPIPathMethod(pathItem, method)
	if !found {
		return validationError(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, method), nil)
	}

	if !explicitAccept && strings.TrimSpace(spec.Accept) == "" {
		if accept := inferAcceptContentType(operationItem); accept != "" {
			spec.Accept = accept
		}
	}
	if !explicitContentType && strings.TrimSpace(spec.ContentType) == "" {
		if contentType := inferRequestContentType(operationItem); contentType != "" {
			spec.ContentType = contentType
		}
	}

	return nil
}

func (g *HTTPResourceServerGateway) validateOpenAPIMethodSupport(ctx context.Context, requestPath string, method string) error {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return err
	}

	_, pathItem, ok := findOpenAPIPathItem(document, requestPath)
	if !ok {
		return nil
	}

	if _, found := openAPIPathMethod(pathItem, method); !found {
		return validationError(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, strings.ToUpper(method)), nil)
	}
	return nil
}
