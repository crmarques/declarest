package openapi

import (
	"errors"
	"strings"

	"declarest/internal/resource"
)

var (
	errOpenAPISpecNotConfigured = errors.New("openapi spec is not configured")
	errOpenAPISchemaNotFound    = errors.New("openapi schema for resource creation not found")
	errOpenAPISchemaEmpty       = errors.New("openapi schema does not define any payload properties")
)

func BuildResourceFromSpec(spec *Spec, logicalPath string) (resource.Resource, error) {
	if spec == nil {
		return resource.Resource{}, errOpenAPISpecNotConfigured
	}
	schema := schemaForResource(spec, logicalPath)
	if schema == nil {
		return resource.Resource{}, errOpenAPISchemaNotFound
	}
	value, ok := schemaDefaultValue(spec, schema)
	if !ok {
		return resource.Resource{}, errOpenAPISchemaEmpty
	}
	return resource.NewResource(value)
}

func schemaForResource(spec *Spec, logicalPath string) map[string]any {
	normalized := resource.NormalizePath(logicalPath)
	collection := parentPath(normalized)
	if schema := schemaForPathAndMethods(spec, collection, []string{"post"}); schema != nil {
		return schema
	}
	return schemaForPathAndMethods(spec, normalized, []string{"put", "patch", "post"})
}

func schemaForPathAndMethods(spec *Spec, path string, methods []string) map[string]any {
	if spec == nil {
		return nil
	}
	item := spec.MatchPath(path)
	if item == nil {
		return nil
	}
	for _, method := range methods {
		if op := item.Operation(method); op != nil && op.RequestSchema != nil {
			return op.RequestSchema
		}
	}
	return nil
}

func parentPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	trimmed = strings.TrimRight(trimmed, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx <= 0 {
		return "/"
	}
	return trimmed[:idx]
}

func schemaDefaultValue(spec *Spec, schema map[string]any) (any, bool) {
	if schema == nil {
		return nil, false
	}
	if constValue, ok := schema["const"]; ok {
		return constValue, true
	}
	if def, ok := schema["default"]; ok {
		return def, true
	}
	if example, ok := schema["example"]; ok {
		return example, true
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return enum[0], true
	}
	if types, ok := schema["type"].([]any); ok && len(types) > 0 {
		if first, ok := types[0].(string); ok {
			schema["type"] = first
		}
	}

	if choices, ok := schema["oneOf"].([]any); ok {
		for _, choice := range choices {
			if resolved, ok := spec.resolveSchema(choice); ok {
				if value, ok := schemaDefaultValue(spec, resolved); ok {
					return value, true
				}
			}
		}
	}
	if choices, ok := schema["anyOf"].([]any); ok {
		for _, choice := range choices {
			if resolved, ok := spec.resolveSchema(choice); ok {
				if value, ok := schemaDefaultValue(spec, resolved); ok {
					return value, true
				}
			}
		}
	}
	if choices, ok := schema["allOf"].([]any); ok {
		merged := map[string]any{}
		for _, choice := range choices {
			if resolved, ok := spec.resolveSchema(choice); ok {
				if value, ok := schemaDefaultValue(spec, resolved); ok {
					if obj, ok := value.(map[string]any); ok {
						for key, val := range obj {
							merged[key] = val
						}
					}
				}
			}
		}
		if len(merged) > 0 {
			return merged, true
		}
	}

	switch schemaType(schema) {
	case "object":
		return schemaDefaultObject(spec, schema), true
	case "array":
		if arr, ok := schemaDefaultArray(spec, schema); ok {
			return arr, true
		}
		return []any{}, true
	case "string":
		return "", true
	case "number":
		return 0, true
	case "integer":
		return 0, true
	case "boolean":
		return false, true
	}

	if _, ok := schema["properties"].(map[string]any); ok {
		return schemaDefaultObject(spec, schema), true
	}

	return nil, false
}

func schemaType(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	switch value := schema["type"].(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(value))
	case []any:
		if len(value) == 0 {
			return ""
		}
		if str, ok := value[0].(string); ok {
			return strings.ToLower(strings.TrimSpace(str))
		}
	}
	return ""
}

func schemaDefaultObject(spec *Spec, schema map[string]any) map[string]any {
	result := map[string]any{}
	properties, _ := schema["properties"].(map[string]any)
	for key, raw := range properties {
		value := any(nil)
		if resolved, ok := spec.resolveSchema(raw); ok {
			if val, ok := schemaDefaultValue(spec, resolved); ok {
				value = val
			}
		}
		result[key] = value
	}
	return result
}

func schemaDefaultArray(spec *Spec, schema map[string]any) (any, bool) {
	items, exists := schema["items"]
	if !exists {
		return []any{}, true
	}
	if list, ok := items.([]any); ok {
		for _, entry := range list {
			if resolved, ok := spec.resolveSchema(entry); ok {
				if value, ok := schemaDefaultValue(spec, resolved); ok {
					return []any{value}, true
				}
			}
		}
	} else if resolved, ok := spec.resolveSchema(items); ok {
		if value, ok := schemaDefaultValue(spec, resolved); ok {
			return []any{value}, true
		}
	}
	return []any{}, true
}
