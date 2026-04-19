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

package validation

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/identitytemplate"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
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
			return nil, faults.Invalid("payload "+field+" contains an empty JSON pointer", nil)
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return nil, faults.Invalid("payload "+field+" contains an invalid JSON pointer", err)
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
	attributes, err := identity.RequiredAttributes(md)
	if err != nil {
		return append([]string(nil), md.RequiredAttributes...)
	}
	return attributes
}

func EffectiveResourceRequiredAttributesForOperation(
	md metadatadomain.ResourceMetadata,
	operation metadatadomain.Operation,
) []string {
	attributes, err := resourceRequiredAttributesForOperation(md, operation)
	if err != nil {
		return append([]string(nil), md.RequiredAttributes...)
	}
	return attributes
}

func EffectiveCreatePayloadRequiredAttributes(md metadatadomain.ResourceMetadata) ([]string, error) {
	createSpec, hasCreate := md.Operations[string(metadatadomain.OperationCreate)]
	if !hasCreate || createSpec.Validate == nil || createSpec.Validate.RequiredAttributes == nil {
		return resourceRequiredAttributesForOperation(md, metadatadomain.OperationCreate)
	}

	attributes := append([]string(nil), createSpec.Validate.RequiredAttributes...)
	addPointer := orderedStringCollector(&attributes)
	if err := appendIdentityTemplatePointers(addPointer, "resource.alias", md.Alias); err != nil {
		return nil, err
	}

	return attributes, nil
}

func ValidateResourceRequiredAttributes(payload resource.Value, md metadatadomain.ResourceMetadata) error {
	required, err := identity.RequiredAttributes(md)
	if err != nil {
		return err
	}
	return ValidateRequiredAttributes(
		payload,
		"resource.requiredAttributes",
		required,
		"resource payload validation",
	)
}

func ValidateResourceRequiredAttributesForOperation(
	payload resource.Value,
	md metadatadomain.ResourceMetadata,
	operation metadatadomain.Operation,
) error {
	required, err := resourceRequiredAttributesForOperation(md, operation)
	if err != nil {
		return err
	}
	return ValidateRequiredAttributes(
		payload,
		"resource.requiredAttributes",
		required,
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

	return faults.Invalid(
		fmt.Sprintf("%s failed: missing required attributes [%s]", scope, strings.Join(missing, ", ")),
		nil,
	)
}

func resourceRequiredAttributesForOperation(
	md metadatadomain.ResourceMetadata,
	operation metadatadomain.Operation,
) ([]string, error) {
	attributes := append([]string(nil), md.RequiredAttributes...)
	addPointer := orderedStringCollector(&attributes)

	if err := appendIdentityTemplatePointers(addPointer, "resource.alias", md.Alias); err != nil {
		return nil, err
	}
	if operation != metadatadomain.OperationCreate {
		if err := appendIdentityTemplatePointers(addPointer, "resource.id", md.ID); err != nil {
			return nil, err
		}
	}

	return attributes, nil
}

func appendIdentityTemplatePointers(
	addPointer func(string),
	field string,
	template string,
) error {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		return nil
	}

	pointers, err := identitytemplate.ExtractPointers(trimmed)
	if err != nil {
		return faults.Invalid(field+" must be a valid identity template", err)
	}
	for _, pointer := range pointers {
		addPointer(pointer)
	}

	return nil
}

func orderedStringCollector(target *[]string) func(string) {
	seen := make(map[string]struct{}, len(*target))
	for _, item := range *target {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}

	return func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		*target = append(*target, trimmed)
	}
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

	aliasPointer, aliasOK, aliasErr := identity.SimpleAliasPointer(md)
	idPointer, idOK, idErr := identity.SimpleIDPointer(md)
	if aliasErr == nil && idErr == nil && aliasOK && idOK && aliasPointer == idPointer {
		if _, exists := fields[idPointer]; !exists {
			if remoteID := strings.TrimSpace(resolvedResource.RemoteID); remoteID != "" {
				fields[idPointer] = remoteID
			} else if localAlias := strings.TrimSpace(resolvedResource.LocalAlias); localAlias != "" {
				fields[idPointer] = localAlias
			}
		}
	} else {
		if aliasErr == nil && aliasOK {
			if localAlias := strings.TrimSpace(resolvedResource.LocalAlias); localAlias != "" {
				if _, exists := fields[aliasPointer]; !exists {
					fields[aliasPointer] = localAlias
				}
			}
		}
		if idErr == nil && idOK {
			if remoteID := strings.TrimSpace(resolvedResource.RemoteID); remoteID != "" {
				if _, exists := fields[idPointer]; !exists {
					fields[idPointer] = remoteID
				}
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
	for _, pointer := range slices.Sorted(maps.Keys(derivedFields)) {
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
