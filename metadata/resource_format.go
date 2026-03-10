package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

// NormalizeResourceFormat normalizes payload types for internal callers that
// still use the historical helper name.
func NormalizeResourceFormat(value string) string {
	return resource.NormalizePayloadType(value)
}

// ValidateResourceFormat validates payload types for internal callers that
// still use the historical helper name.
func ValidateResourceFormat(value string) (string, error) {
	return resource.ValidatePayloadType(value)
}

func ResourceFormatMediaType(value string) (string, error) {
	format, err := ValidateResourceFormat(value)
	if err != nil {
		return "", err
	}
	return resource.PayloadMediaType(format)
}

func ResourceFormatExtension(value string) (string, error) {
	format, err := ValidateResourceFormat(value)
	if err != nil {
		return "", err
	}
	return resource.PayloadExtension(format)
}

func EffectivePayloadType(metadata ResourceMetadata, fallback string) (string, error) {
	if strings.TrimSpace(metadata.PayloadType) != "" {
		return ValidateResourceFormat(metadata.PayloadType)
	}
	return ValidateResourceFormat(fallback)
}

// TemplateFuncMap returns metadata template helpers evaluated against the
// provided render scope.
func TemplateFuncMap(scope map[string]any) template.FuncMap {
	resolveScopePayloadType := func(arg any) (string, error) {
		if arg != nil {
			if _, ok := arg.(map[string]any); !ok {
				return "", faults.NewTypedError(
					faults.ValidationError,
					"payload_type template function expects root scope argument (.)",
					nil,
				)
			}
		}

		candidate := scopeString(scopeValue(scope, "payloadType"))
		if strings.TrimSpace(candidate) == "" {
			if descriptor, ok := resource.PayloadDescriptorForContentType(scopeString(scopeValue(scope, "payloadMediaType"))); ok {
				candidate = descriptor.PayloadType
			}
		}
		if strings.TrimSpace(candidate) == "" {
			if descriptor, ok := resource.PayloadDescriptorForExtension(scopeString(scopeValue(scope, "payloadExtension"))); ok {
				candidate = descriptor.PayloadType
			}
		}
		return ValidateResourceFormat(candidate)
	}

	resolveJSONPointer := func(pointer string) (string, error) {
		trimmed := strings.TrimSpace(pointer)
		if trimmed == "" {
			return "", fmt.Errorf("json_pointer template function expects a JSON pointer argument")
		}

		value, found, err := resource.LookupJSONPointer(scope, trimmed)
		if err != nil {
			return "", err
		}
		if !found || value == nil {
			return "", fmt.Errorf("JSON pointer %q did not resolve to a value", trimmed)
		}

		rendered, ok := templateScalarString(value)
		if !ok {
			return "", fmt.Errorf("JSON pointer %q resolved to a non-scalar value", trimmed)
		}
		return rendered, nil
	}

	return template.FuncMap{
		"json_pointer": resolveJSONPointer,
		"payload_type": resolveScopePayloadType,
		"payload_media_type": func(arg any) (string, error) {
			payloadType, err := resolveScopePayloadType(arg)
			if err != nil {
				return "", err
			}
			return ResourceFormatMediaType(payloadType)
		},
		"payload_extension": func(arg any) (string, error) {
			payloadType, err := resolveScopePayloadType(arg)
			if err != nil {
				return "", err
			}
			return ResourceFormatExtension(payloadType)
		},
	}
}

func scopeValue(scope map[string]any, key string) any {
	if scope == nil {
		return nil
	}
	return scope[key]
}

func templateScalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case fmt.Stringer:
		return typed.String(), true
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}
