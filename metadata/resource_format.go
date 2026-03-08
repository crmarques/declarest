package metadata

import (
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

	return template.FuncMap{
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
