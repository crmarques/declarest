package validation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func NormalizeAttributePointers(field string, attributes []string) ([]string, error) {
	if attributes == nil {
		return nil, nil
	}

	pointers := make([]string, 0, len(attributes))
	seen := make(map[string]struct{}, len(attributes))
	for _, raw := range attributes {
		pointer := strings.TrimSpace(raw)
		if pointer == "" {
			return nil, faults.NewValidationError("payload "+field+" contains an empty JSON pointer", nil)
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return nil, faults.NewValidationError("payload "+field+" contains an invalid JSON pointer", err)
		}
		if _, exists := seen[pointer]; exists {
			continue
		}
		seen[pointer] = struct{}{}
		pointers = append(pointers, pointer)
	}

	return pointers, nil
}

func EffectiveResourceRequiredAttributes(md metadatadomain.ResourceMetadata) []string {
	attributes := append([]string(nil), md.RequiredAttributes...)
	aliasAttribute := strings.TrimSpace(md.AliasAttribute)
	if aliasAttribute == "" {
		return attributes
	}

	for _, attribute := range md.RequiredAttributes {
		if strings.TrimSpace(attribute) == aliasAttribute {
			return attributes
		}
	}

	return append(attributes, aliasAttribute)
}

func ValidateResourceRequiredAttributes(payload resource.Value, md metadatadomain.ResourceMetadata) error {
	return ValidateRequiredAttributes(
		payload,
		"resource.requiredAttributes",
		EffectiveResourceRequiredAttributes(md),
		"resource payload validation",
	)
}

func ValidateRequiredAttributes(
	payload resource.Value,
	field string,
	attributes []string,
	scope string,
) error {
	pointers, err := NormalizeAttributePointers(field, attributes)
	if err != nil {
		return err
	}
	if len(pointers) == 0 {
		return nil
	}

	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return err
	}

	missing := make([]string, 0)
	for _, pointer := range pointers {
		value, found, lookupErr := resource.LookupJSONPointer(normalizedPayload, pointer)
		if lookupErr != nil {
			return lookupErr
		}
		if !found || value == nil {
			missing = append(missing, pointer)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return faults.NewValidationError(
		fmt.Sprintf("%s failed: missing required attributes [%s]", scope, strings.Join(missing, ", ")),
		nil,
	)
}

func DerivePathFields(resolvedResource resource.Resource, md metadatadomain.ResourceMetadata) map[string]any {
	fields := map[string]any{}

	for key, value := range templatescope.DerivePathTemplateFields(resolvedResource.LogicalPath, md) {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		fields[resource.JSONPointerForObjectKey(trimmedKey)] = trimmedValue
	}

	if aliasAttribute := strings.TrimSpace(md.AliasAttribute); aliasAttribute != "" {
		if localAlias := strings.TrimSpace(resolvedResource.LocalAlias); localAlias != "" {
			if _, exists := fields[aliasAttribute]; !exists {
				fields[aliasAttribute] = localAlias
			}
		}
	}
	if idAttribute := strings.TrimSpace(md.IDAttribute); idAttribute != "" {
		if remoteID := strings.TrimSpace(resolvedResource.RemoteID); remoteID != "" {
			if _, exists := fields[idAttribute]; !exists {
				fields[idAttribute] = remoteID
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func MergePayloadFields(payload resource.Value, derivedFields map[string]any) resource.Value {
	if len(derivedFields) == 0 {
		return payload
	}

	baseObject, isObject := payload.(map[string]any)
	if !isObject {
		baseObject = map[string]any{}
		if payload != nil {
			baseObject["payload"] = payload
		}
	}

	merged := resource.DeepCopyValue(baseObject)
	for _, pointer := range sortedMapKeysAny(derivedFields) {
		if _, found, err := resource.LookupJSONPointer(merged, pointer); err == nil && found {
			continue
		}

		next, err := resource.SetJSONPointerValue(merged, pointer, derivedFields[pointer])
		if err != nil {
			continue
		}
		merged = next
	}

	return merged
}

func sortedMapKeysAny(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
