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

package fsmetadata

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/identitytemplate"
	"github.com/crmarques/declarest/resource"
)

func validateResourceMetadata(kind metadataPathKind, metadata metadatadomain.ResourceMetadata) error {
	resolvedPayloadType := ""
	if strings.TrimSpace(metadata.Format) != "" {
		payloadType, err := metadatadomain.ValidateResourceFormat(metadata.Format)
		if err != nil {
			return err
		}
		if !metadatadomain.ResourceFormatAllowsMixedItems(payloadType) {
			resolvedPayloadType = payloadType
		}
	}
	if err := metadatadomain.ValidateDefaultsSpec(metadata.Defaults); err != nil {
		return err
	}
	if err := validateSelectorSpec(kind, metadata.Selector); err != nil {
		return err
	}

	if _, err := metadatadomain.ResolveExternalizedAttributes(metadata); err != nil {
		return err
	}
	if err := validateIdentityTemplate("resource.id", metadata.ID); err != nil {
		return err
	}
	if err := validateIdentityTemplate("resource.alias", metadata.Alias); err != nil {
		return err
	}
	if err := validateAttributePointers("resource.requiredAttributes", metadata.RequiredAttributes); err != nil {
		return err
	}
	if err := validateAttributePointers("resource.secretAttributes", metadata.SecretAttributes); err != nil {
		return err
	}
	if err := validateStructuredOnlyMetadataFields(resolvedPayloadType, metadata); err != nil {
		return err
	}
	if metadata.IsWholeResourceSecret() && len(metadata.SecretAttributes) > 0 {
		return faults.Invalid(
			"resource.secret: true and resource.secretAttributes are mutually exclusive",
			nil,
		)
	}
	if err := validateStructuredPayloadDirectives("metadata defaults", resolvedPayloadType, metadata.Transforms, nil); err != nil {
		return err
	}

	keys := slices.Sorted(maps.Keys(metadata.Operations))
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return faults.Invalid(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}

		operationSpec := metadata.Operations[key]
		if err := validateStructuredPayloadDirectives(
			fmt.Sprintf("operation %q", key),
			resolvedPayloadType,
			operationSpec.Transforms,
			operationSpec.Validate,
		); err != nil {
			return err
		}
		if err := validateOperationValidationSpec(metadatadomain.Operation(key), operationSpec.Validate); err != nil {
			return err
		}
		if err := metadatadomain.ValidateOperationSpecTemplates(fmt.Sprintf("operation %q", key), operationSpec); err != nil {
			return err
		}
	}
	return nil
}

func validateSelectorSpec(kind metadataPathKind, spec *metadatadomain.SelectorSpec) error {
	if spec == nil || spec.Descendants == nil {
		return nil
	}
	if kind == metadataPathCollection {
		return nil
	}
	return faults.Invalid("selector.descendants is only supported on collection metadata", nil)
}

func validateStructuredOnlyMetadataFields(
	payloadType string,
	metadata metadatadomain.ResourceMetadata,
) error {
	if strings.TrimSpace(payloadType) == "" || resource.IsStructuredPayloadType(payloadType) {
		return nil
	}

	structuredPayloadTypes := "json, yaml, ini, properties"

	if strings.TrimSpace(metadata.ID) != "" {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.id requires structured payload type (%s); got %q",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	if strings.TrimSpace(metadata.Alias) != "" {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.alias requires structured payload type (%s); got %q",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	if len(metadata.SecretAttributes) > 0 {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.secretAttributes requires structured payload type (%s); got %q; use resource.secret: true for whole-resource secrets",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	if len(metadata.RequiredAttributes) > 0 {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.requiredAttributes requires structured payload type (%s); got %q",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	if len(metadata.ExternalizedAttributes) > 0 {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.externalizedAttributes requires structured payload type (%s); got %q",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	if metadatadomain.HasDefaultsSpecDirectives(metadata.Defaults) {
		return faults.Invalid(
			fmt.Sprintf(
				"resource.defaults requires structured payload type (%s); got %q",
				structuredPayloadTypes,
				payloadType,
			),
			nil,
		)
	}
	return nil
}

func validateStructuredPayloadDirectives(
	scope string,
	payloadType string,
	mutations []metadatadomain.TransformStep,
	validate *metadatadomain.OperationValidationSpec,
) error {
	for idx, step := range mutations {
		stepType := metadatadomain.TransformStepType(step)
		if stepType == "" {
			return faults.Invalid(
				fmt.Sprintf("%s transforms[%d] must define exactly one of selectAttributes, excludeAttributes, or jqExpression", scope, idx),
				nil,
			)
		}
		switch stepType {
		case "selectAttributes":
			if err := validateAttributePointers(fmt.Sprintf("%s transforms[%d].selectAttributes", scope, idx), step.SelectAttributes); err != nil {
				return err
			}
		case "excludeAttributes":
			if err := validateAttributePointers(fmt.Sprintf("%s transforms[%d].excludeAttributes", scope, idx), step.ExcludeAttributes); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(payloadType) == "" || resource.IsStructuredPayloadType(payloadType) {
		return nil
	}
	if len(mutations) == 0 && validate == nil {
		return nil
	}
	return faults.Invalid(
		fmt.Sprintf("%s uses structured payload directives with non-structured payload type %q", scope, payloadType),
		nil,
	)
}

func validateOperationValidationSpec(
	operation metadatadomain.Operation,
	spec *metadatadomain.OperationValidationSpec,
) error {
	if spec == nil {
		return nil
	}

	for idx, attribute := range spec.RequiredAttributes {
		if strings.TrimSpace(attribute) == "" {
			return faults.Invalid(
				fmt.Sprintf("operation %q validate.requiredAttributes[%d] must not be empty", operation, idx),
				nil,
			)
		}
	}
	if err := validateAttributePointers(fmt.Sprintf("operation %q validate.requiredAttributes", operation), spec.RequiredAttributes); err != nil {
		return err
	}

	for idx, assertion := range spec.Assertions {
		if strings.TrimSpace(assertion.JQ) == "" {
			return faults.Invalid(
				fmt.Sprintf("operation %q validate.assertions[%d].jq must not be empty", operation, idx),
				nil,
			)
		}
	}

	schemaRef := strings.TrimSpace(spec.SchemaRef)
	if schemaRef == "" {
		return nil
	}
	if schemaRef == "openapi:request-body" {
		return nil
	}
	if strings.HasPrefix(schemaRef, "openapi:#/") {
		return nil
	}
	return faults.Invalid(
		fmt.Sprintf(
			"operation %q validate.schemaRef %q is not supported (expected openapi:request-body or openapi:#/...)",
			operation,
			schemaRef,
		),
		nil,
	)
}

func validateIdentityTemplate(field string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if _, err := identitytemplate.Compile(trimmed); err != nil {
		return faults.Invalid(field+" must be a valid identity template or JSON pointer shorthand", err)
	}
	return nil
}

func validateAttributePointers(field string, values []string) error {
	for idx, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return faults.Invalid(fmt.Sprintf("%s[%d] must not be empty", field, idx), nil)
		}
		if _, err := resource.ParseJSONPointer(trimmed); err != nil {
			return faults.Invalid(fmt.Sprintf("%s[%d] must be a valid JSON pointer", field, idx), err)
		}
	}
	return nil
}
