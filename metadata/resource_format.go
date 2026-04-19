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

package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

const ResourceFormatAny = "any"

func NormalizeResourceFormat(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "":
		return ""
	case ResourceFormatAny:
		return ResourceFormatAny
	default:
		return resource.NormalizePayloadType(trimmed)
	}
}

func ValidateResourceFormat(value string) (string, error) {
	normalized := NormalizeResourceFormat(value)
	if normalized == "" {
		return "", nil
	}
	if normalized == ResourceFormatAny {
		return normalized, nil
	}
	return resource.ValidatePayloadType(normalized)
}

func ValidateConcreteResourceFormat(value string) (string, error) {
	format, err := ValidateResourceFormat(value)
	if err != nil {
		return "", err
	}
	if format == ResourceFormatAny {
		return "", faults.Invalid("resource format must be concrete", nil)
	}
	return format, nil
}

func ResourceFormatMediaType(value string) (string, error) {
	format, err := ValidateConcreteResourceFormat(value)
	if err != nil {
		return "", err
	}
	if format == "" {
		return "", nil
	}
	return resource.PayloadMediaType(format)
}

func ResourceFormatExtension(value string) (string, error) {
	format, err := ValidateConcreteResourceFormat(value)
	if err != nil {
		return "", err
	}
	if format == "" {
		return "", nil
	}
	return resource.PayloadExtension(format)
}

func ResourceFormatAllowsMixedItems(value string) bool {
	return NormalizeResourceFormat(value) == ResourceFormatAny
}

func EffectivePayloadType(metadata ResourceMetadata, fallback string) (string, error) {
	if format := NormalizeResourceFormat(metadata.Format); format != "" && format != ResourceFormatAny {
		return ValidateConcreteResourceFormat(format)
	}
	return resource.ValidatePayloadType(fallback)
}

func ResolveTemplatePayloadDescriptor(
	metadata ResourceMetadata,
	payload any,
	descriptor resource.PayloadDescriptor,
) resource.PayloadDescriptor {
	activeDescriptor := descriptor
	if !resource.IsPayloadDescriptorExplicit(activeDescriptor) {
		activeDescriptor = explicitDescriptorFromPayloadValue(payload)
	}
	if !resource.IsPayloadDescriptorExplicit(activeDescriptor) {
		if format := NormalizeResourceFormat(metadata.Format); format != "" && format != ResourceFormatAny {
			activeDescriptor = resource.PayloadDescriptor{PayloadType: format}
		}
	}
	if !resource.IsPayloadDescriptorExplicit(activeDescriptor) {
		activeDescriptor = InferPayloadDescriptor(payload)
	}
	return resource.NormalizePayloadDescriptor(activeDescriptor)
}

func ApplyPayloadTemplateScope(
	scope map[string]any,
	metadata ResourceMetadata,
	payload any,
	descriptor resource.PayloadDescriptor,
) {
	if scope == nil {
		return
	}

	activeDescriptor := ResolveTemplatePayloadDescriptor(metadata, payload, descriptor)
	scope["payloadType"] = activeDescriptor.PayloadType
	scope["payloadMediaType"] = activeDescriptor.MediaType
	scope["payloadExtension"] = activeDescriptor.Extension
	if _, exists := scope["contentType"]; !exists && strings.TrimSpace(activeDescriptor.MediaType) != "" {
		if _, isPayloadMap := scope["payload"].(map[string]any); !isPayloadMap {
			scope["contentType"] = activeDescriptor.MediaType
		}
	}
}

func InferPayloadDescriptor(value any) resource.PayloadDescriptor {
	if descriptor := explicitDescriptorFromPayloadValue(value); resource.IsPayloadDescriptorExplicit(descriptor) {
		return resource.NormalizePayloadDescriptor(descriptor)
	}

	value = unwrapPayloadContentValue(value)
	switch typed := value.(type) {
	case nil:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	case resource.BinaryValue:
		return resource.DefaultOctetStreamDescriptor()
	case *resource.BinaryValue:
		if typed != nil {
			return resource.DefaultOctetStreamDescriptor()
		}
	case string:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
	switch normalized.(type) {
	case string:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	case resource.BinaryValue:
		return resource.DefaultOctetStreamDescriptor()
	default:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
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
		return ValidateConcreteResourceFormat(candidate)
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
			if mediaType := strings.TrimSpace(scopeString(scopeValue(scope, "payloadMediaType"))); mediaType != "" {
				return mediaType, nil
			}
			payloadType, err := resolveScopePayloadType(arg)
			if err != nil {
				return "", err
			}
			return ResourceFormatMediaType(payloadType)
		},
		"payload_extension": func(arg any) (string, error) {
			if extension := strings.TrimSpace(scopeString(scopeValue(scope, "payloadExtension"))); extension != "" {
				return extension, nil
			}
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

func unwrapPayloadContentValue(value any) any {
	switch typed := value.(type) {
	case resource.Content:
		return typed.Value
	case *resource.Content:
		if typed != nil {
			return typed.Value
		}
	}
	return value
}

func explicitDescriptorFromPayloadValue(value any) resource.PayloadDescriptor {
	switch typed := value.(type) {
	case resource.Content:
		return typed.Descriptor
	case *resource.Content:
		if typed != nil {
			return typed.Descriptor
		}
	}
	return resource.PayloadDescriptor{}
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
