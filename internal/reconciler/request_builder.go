package reconciler

import (
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/resource"
)

func (r *DefaultReconciler) buildRequestSpecWithTarget(record resource.ResourceRecord, targetPath, logicalPath string, op *resource.OperationMetadata, isCollection bool) (managedserver.RequestSpec, error) {
	if op == nil {
		op = &resource.OperationMetadata{
			HTTPMethod: "GET",
		}
	}
	method := strings.ToUpper(strings.TrimSpace(op.HTTPMethod))
	if method == "" {
		method = "GET"
	}

	resolvedPath := targetPath
	if strings.TrimSpace(logicalPath) != "" && op.URL != nil && strings.TrimSpace(op.URL.Path) != "" {
		opPath, err := record.ResolveOperationPath(logicalPath, op, isCollection)
		if err != nil {
			return managedserver.RequestSpec{}, err
		}
		if strings.TrimSpace(opPath) != "" {
			resolvedPath = opPath
		}
	}

	headerPath := logicalPath
	if strings.TrimSpace(headerPath) == "" {
		headerPath = record.Path
	}
	if strings.TrimSpace(headerPath) == "" {
		headerPath = targetPath
	}
	headers := record.HeadersFor(op, headerPath, isCollection)
	query := record.QueryFor(op)
	accept := strings.Join(takeHeaderValues(headers, "Accept"), ", ")
	contentType := firstHeaderValue(headers, "Content-Type")

	return managedserver.RequestSpec{
		Kind: managedserver.KindHTTP,
		HTTP: &managedserver.HTTPRequestSpec{
			Method:      method,
			Path:        resolvedPath,
			Headers:     headers,
			Query:       query,
			Accept:      accept,
			ContentType: contentType,
		},
	}, nil
}

func takeHeaderValues(headers map[string][]string, name string) []string {
	if len(headers) == 0 {
		return nil
	}
	var values []string
	for key, list := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		if len(list) > 0 {
			values = append(values, list...)
		}
		delete(headers, key)
	}
	return values
}

func firstHeaderValue(headers map[string][]string, name string) string {
	values := takeHeaderValues(headers, name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
