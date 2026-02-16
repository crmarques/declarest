package metadata

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/support/paths"
)

func ResolveOperationSpec(_ context.Context, metadata ResourceMetadata, operation Operation, value any) (OperationSpec, error) {
	if !operation.IsValid() {
		return OperationSpec{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported metadata operation %q", operation),
			nil,
		)
	}

	spec := OperationSpec{
		Filter:   cloneStringSlice(metadata.Filter),
		Suppress: cloneStringSlice(metadata.Suppress),
		JQ:       metadata.JQ,
	}

	if metadata.Operations != nil {
		if operationSpec, found := metadata.Operations[string(operation)]; found {
			spec = mergeOperationSpec(spec, operationSpec)
		}
	}

	scope, err := buildTemplateScope(value)
	if err != nil {
		return OperationSpec{}, err
	}

	rendered, err := renderOperationSpecTemplates(spec, scope)
	if err != nil {
		return OperationSpec{}, err
	}

	if strings.TrimSpace(rendered.Path) == "" {
		return OperationSpec{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("metadata operation %q path is required", operation),
			nil,
		)
	}

	return rendered, nil
}

func InferFromOpenAPI(_ context.Context, logicalPath string, _ InferenceRequest) (ResourceMetadata, error) {
	normalizedPath, err := paths.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return ResourceMetadata{}, err
	}

	collectionPath := path.Dir(normalizedPath)
	if collectionPath == "." || collectionPath == "" {
		collectionPath = "/"
	}

	return ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Method: "GET",
				Path:   normalizedPath,
			},
			string(OperationCreate): {
				Method: "POST",
				Path:   normalizedPath,
			},
			string(OperationUpdate): {
				Method: "PUT",
				Path:   normalizedPath,
			},
			string(OperationDelete): {
				Method: "DELETE",
				Path:   normalizedPath,
			},
			string(OperationList): {
				Method: "GET",
				Path:   collectionPath,
			},
			string(OperationCompare): {
				Method: "GET",
				Path:   normalizedPath,
			},
		},
	}, nil
}

func mergeOperationSpec(base OperationSpec, overlay OperationSpec) OperationSpec {
	merged := OperationSpec{
		Method:      base.Method,
		Path:        base.Path,
		Query:       cloneStringMap(base.Query),
		Headers:     cloneStringMap(base.Headers),
		Accept:      base.Accept,
		ContentType: base.ContentType,
		Body:        base.Body,
		Filter:      cloneStringSlice(base.Filter),
		Suppress:    cloneStringSlice(base.Suppress),
		JQ:          base.JQ,
	}

	if overlay.Method != "" {
		merged.Method = overlay.Method
	}
	if overlay.Path != "" {
		merged.Path = overlay.Path
	}
	if overlay.Query != nil {
		if len(overlay.Query) == 0 {
			merged.Query = map[string]string{}
		} else {
			if merged.Query == nil {
				merged.Query = make(map[string]string, len(overlay.Query))
			}
			keys := sortedMapKeys(overlay.Query)
			for _, key := range keys {
				merged.Query[key] = overlay.Query[key]
			}
		}
	}
	if overlay.Headers != nil {
		if len(overlay.Headers) == 0 {
			merged.Headers = map[string]string{}
		} else {
			if merged.Headers == nil {
				merged.Headers = make(map[string]string, len(overlay.Headers))
			}
			keys := sortedMapKeys(overlay.Headers)
			for _, key := range keys {
				merged.Headers[key] = overlay.Headers[key]
			}
		}
	}
	if overlay.Accept != "" {
		merged.Accept = overlay.Accept
	}
	if overlay.ContentType != "" {
		merged.ContentType = overlay.ContentType
	}
	if overlay.Body != nil {
		merged.Body = overlay.Body
	}
	if overlay.Filter != nil {
		merged.Filter = cloneStringSlice(overlay.Filter)
	}
	if overlay.Suppress != nil {
		merged.Suppress = cloneStringSlice(overlay.Suppress)
	}
	if overlay.JQ != "" {
		merged.JQ = overlay.JQ
	}

	return merged
}

func renderOperationSpecTemplates(spec OperationSpec, scope map[string]any) (OperationSpec, error) {
	rendered := OperationSpec{
		Query:    cloneStringMap(spec.Query),
		Headers:  cloneStringMap(spec.Headers),
		Body:     spec.Body,
		Filter:   cloneStringSlice(spec.Filter),
		Suppress: cloneStringSlice(spec.Suppress),
	}

	var err error
	rendered.Method, err = renderTemplateString("method", spec.Method, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.Path, err = renderTemplateString("path", spec.Path, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.Accept, err = renderTemplateString("accept", spec.Accept, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.ContentType, err = renderTemplateString("contentType", spec.ContentType, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.JQ, err = renderTemplateString("jq", spec.JQ, scope)
	if err != nil {
		return OperationSpec{}, err
	}

	for _, key := range sortedMapKeys(rendered.Query) {
		value, renderErr := renderTemplateString("query."+key, rendered.Query[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Query[key] = value
	}

	for _, key := range sortedMapKeys(rendered.Headers) {
		value, renderErr := renderTemplateString("headers."+key, rendered.Headers[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Headers[key] = value
	}

	return rendered, nil
}

func renderTemplateString(field string, raw string, scope map[string]any) (string, error) {
	if !strings.Contains(raw, "{{") {
		return raw, nil
	}

	tmpl, err := template.New(field).Option("missingkey=error").Parse(raw)
	if err != nil {
		return "", faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("invalid metadata template for %s", field),
			err,
		)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, scope); err != nil {
		return "", faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("failed to render metadata template for %s", field),
			err,
		)
	}
	return buffer.String(), nil
}

func buildTemplateScope(value any) (map[string]any, error) {
	scope := make(map[string]any)
	if payload, ok := value.(map[string]any); ok {
		for key, item := range payload {
			scope[key] = item
		}
		scope["payload"] = payload
	} else {
		scope["payload"] = value
	}
	scope["value"] = value

	return scope, nil
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
