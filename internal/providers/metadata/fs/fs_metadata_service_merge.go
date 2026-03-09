package fsmetadata

import (
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func validateResourceMetadata(metadata metadatadomain.ResourceMetadata) error {
	resolvedPayloadType := ""
	if strings.TrimSpace(metadata.PayloadType) != "" {
		payloadType, err := metadatadomain.ValidateResourceFormat(metadata.PayloadType)
		if err != nil {
			return err
		}
		resolvedPayloadType = payloadType
	}

	if _, err := metadatadomain.ResolveExternalizedAttributes(metadata); err != nil {
		return err
	}
	if err := validateAttributePointer("resourceInfo.idFromAttribute", metadata.IDFromAttribute); err != nil {
		return err
	}
	if err := validateAttributePointer("resourceInfo.aliasFromAttribute", metadata.AliasFromAttribute); err != nil {
		return err
	}
	if err := validateAttributePointers("resourceInfo.secretInAttributes", metadata.SecretsFromAttributes); err != nil {
		return err
	}
	if err := validateStructuredOnlyMetadataFields(resolvedPayloadType, metadata); err != nil {
		return err
	}
	if metadata.IsWholeResourceSecret() && len(metadata.SecretsFromAttributes) > 0 {
		return faults.NewValidationError(
			"resourceInfo.secret: true and resourceInfo.secretInAttributes are mutually exclusive",
			nil,
		)
	}
	if err := validateStructuredPayloadDirectives("metadata defaults", resolvedPayloadType, metadata.PayloadMutation, nil); err != nil {
		return err
	}

	keys := sortedOperationKeys(metadata.Operations)
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return faults.NewValidationError(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}

		operationSpec := metadata.Operations[key]
		if err := validateStructuredPayloadDirectives(
			fmt.Sprintf("operation %q", key),
			resolvedPayloadType,
			operationSpec.PayloadMutation,
			operationSpec.Validate,
		); err != nil {
			return err
		}
		if err := validateOperationValidationSpec(metadatadomain.Operation(key), operationSpec.Validate); err != nil {
			return err
		}
	}
	return nil
}

func validateStructuredOnlyMetadataFields(
	payloadType string,
	metadata metadatadomain.ResourceMetadata,
) error {
	if strings.TrimSpace(payloadType) == "" || resource.IsStructuredPayloadType(payloadType) {
		return nil
	}

	if strings.TrimSpace(metadata.IDFromAttribute) != "" {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.idFromAttribute requires structured payload type (json, yaml); got %q",
				payloadType,
			),
			nil,
		)
	}
	if strings.TrimSpace(metadata.AliasFromAttribute) != "" {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.aliasFromAttribute requires structured payload type (json, yaml); got %q",
				payloadType,
			),
			nil,
		)
	}
	if len(metadata.SecretsFromAttributes) > 0 {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.secretInAttributes requires structured payload type (json, yaml); got %q; use resourceInfo.secret: true for whole-resource secrets",
				payloadType,
			),
			nil,
		)
	}
	if len(metadata.ExternalizedAttributes) > 0 {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.externalizedAttributes requires structured payload type (json, yaml); got %q",
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
	mutations []metadatadomain.PayloadMutationStep,
	validate *metadatadomain.OperationValidationSpec,
) error {
	for idx, step := range mutations {
		stepType := metadatadomain.PayloadMutationStepType(step)
		if stepType == "" {
			return faults.NewValidationError(
				fmt.Sprintf("%s payloadMutation[%d] must define exactly one of selectAttributes, suppressAttributes, or jqExpression", scope, idx),
				nil,
			)
		}
		switch stepType {
		case "selectAttributes":
			if err := validateAttributePointers(fmt.Sprintf("%s payloadMutation[%d].selectAttributes", scope, idx), step.SelectAttributes); err != nil {
				return err
			}
		case "suppressAttributes":
			if err := validateAttributePointers(fmt.Sprintf("%s payloadMutation[%d].suppressAttributes", scope, idx), step.SuppressAttributes); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(payloadType) == "" || payloadType == metadatadomain.NormalizeResourceFormat("json") || payloadType == metadatadomain.NormalizeResourceFormat("yaml") {
		return nil
	}
	if len(mutations) == 0 && validate == nil {
		return nil
	}
	return faults.NewValidationError(
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
			return faults.NewValidationError(
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
			return faults.NewValidationError(
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
	return faults.NewValidationError(
		fmt.Sprintf(
			"operation %q validate.schemaRef %q is not supported (expected openapi:request-body or openapi:#/...)",
			operation,
			schemaRef,
		),
		nil,
	)
}

func validateAttributePointer(field string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if _, err := resource.ParseJSONPointer(trimmed); err != nil {
		return faults.NewValidationError(field+" must be a valid JSON pointer", err)
	}
	return nil
}

func validateAttributePointers(field string, values []string) error {
	for idx, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return faults.NewValidationError(fmt.Sprintf("%s[%d] must not be empty", field, idx), nil)
		}
		if _, err := resource.ParseJSONPointer(trimmed); err != nil {
			return faults.NewValidationError(fmt.Sprintf("%s[%d] must be a valid JSON pointer", field, idx), err)
		}
	}
	return nil
}
