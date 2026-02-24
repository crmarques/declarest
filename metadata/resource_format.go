package metadata

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
)

const (
	resourceFormatJSON = "json"
	resourceFormatYAML = "yaml"
)

var resourceFormatTemplateTokenPattern = regexp.MustCompile(`\{\{\s*resource_format\s+\.\s*\}\}`)

// NormalizeResourceFormat trims and lowercases the configured repository
// resource format and defaults empty values to json.
func NormalizeResourceFormat(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return resourceFormatJSON
	}
	return normalized
}

// ValidateResourceFormat returns the normalized repository resource format when
// it is supported by the runtime.
func ValidateResourceFormat(value string) (string, error) {
	normalized := NormalizeResourceFormat(value)
	switch normalized {
	case resourceFormatJSON, resourceFormatYAML:
		return normalized, nil
	default:
		return "", faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported resource format %q", strings.TrimSpace(value)),
			nil,
		)
	}
}

// ResourceFormatMediaType returns the default media type used for metadata
// operation requests based on the active repository resource format.
func ResourceFormatMediaType(value string) (string, error) {
	format, err := ValidateResourceFormat(value)
	if err != nil {
		return "", err
	}
	return "application/" + format, nil
}

// ResolveResourceFormatTemplatesInMetadata replaces metadata string template
// tokens matching {{resource_format .}} with the provided repository resource
// format while leaving other templates (for example {{.id}}) unchanged.
func ResolveResourceFormatTemplatesInMetadata(value ResourceMetadata, resourceFormat string) (ResourceMetadata, error) {
	resolvedFormat, err := ValidateResourceFormat(resourceFormat)
	if err != nil {
		return ResourceMetadata{}, err
	}

	cloned := CloneResourceMetadata(value)
	cloned.CollectionPath = replaceResourceFormatTemplateTokens(cloned.CollectionPath, resolvedFormat)
	cloned.JQ = replaceResourceFormatTemplateTokens(cloned.JQ, resolvedFormat)

	if cloned.Operations != nil {
		for key, spec := range cloned.Operations {
			spec.Method = replaceResourceFormatTemplateTokens(spec.Method, resolvedFormat)
			spec.Path = replaceResourceFormatTemplateTokens(spec.Path, resolvedFormat)
			spec.Accept = replaceResourceFormatTemplateTokens(spec.Accept, resolvedFormat)
			spec.ContentType = replaceResourceFormatTemplateTokens(spec.ContentType, resolvedFormat)
			spec.JQ = replaceResourceFormatTemplateTokens(spec.JQ, resolvedFormat)
			if spec.Query != nil {
				for queryKey, queryValue := range spec.Query {
					spec.Query[queryKey] = replaceResourceFormatTemplateTokens(queryValue, resolvedFormat)
				}
			}
			if spec.Headers != nil {
				for headerKey, headerValue := range spec.Headers {
					spec.Headers[headerKey] = replaceResourceFormatTemplateTokens(headerValue, resolvedFormat)
				}
			}

			bodyValue, bodyErr := resolveResourceFormatTemplateTokensInValue(spec.Body, resolvedFormat)
			if bodyErr != nil {
				return ResourceMetadata{}, bodyErr
			}
			spec.Body = bodyValue

			cloned.Operations[key] = spec
		}
	}

	return cloned, nil
}

// TemplateFuncMap returns metadata template helpers evaluated against the
// provided render scope.
func TemplateFuncMap(scope map[string]any) template.FuncMap {
	return template.FuncMap{
		"resource_format": func(arg any) (string, error) {
			if arg != nil {
				if _, ok := arg.(map[string]any); !ok {
					return "", faults.NewTypedError(
						faults.ValidationError,
						"resource_format template function expects root scope argument (.)",
						nil,
					)
				}
			}
			return ValidateResourceFormat(scopeString(scopeValue(scope, "resourceFormat")))
		},
	}
}

func scopeValue(scope map[string]any, key string) any {
	if scope == nil {
		return nil
	}
	return scope[key]
}

func replaceResourceFormatTemplateTokens(value string, resourceFormat string) string {
	if strings.TrimSpace(value) == "" || !strings.Contains(value, "resource_format") {
		return value
	}
	return resourceFormatTemplateTokenPattern.ReplaceAllString(value, resourceFormat)
}

func resolveResourceFormatTemplateTokensInValue(value any, resourceFormat string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			resolved, err := resolveResourceFormatTemplateTokensInValue(item, resourceFormat)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			resolved, err := resolveResourceFormatTemplateTokensInValue(typed[idx], resourceFormat)
			if err != nil {
				return nil, err
			}
			result[idx] = resolved
		}
		return result, nil
	case string:
		return replaceResourceFormatTemplateTokens(typed, resourceFormat), nil
	default:
		return typed, nil
	}
}
