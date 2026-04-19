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

package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
)

func (g *Client) applyOpenAPIFallback(
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
		return faults.Invalid(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, method), nil)
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

func (g *Client) validateOpenAPIMethodSupport(ctx context.Context, requestPath string, method string) error {
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
		return faults.Invalid(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, strings.ToUpper(method)), nil)
	}
	return nil
}
