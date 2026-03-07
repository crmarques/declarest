package fsmetadata

import (
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
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
	if err := validateStructuredPayloadDirectives("metadata defaults", resolvedPayloadType, metadata.Filter, metadata.Suppress, metadata.JQ, nil); err != nil {
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
			operationSpec.Filter,
			operationSpec.Suppress,
			operationSpec.JQ,
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

func validateStructuredPayloadDirectives(
	scope string,
	payloadType string,
	filter []string,
	suppress []string,
	jq string,
	validate *metadatadomain.OperationValidationSpec,
) error {
	if strings.TrimSpace(payloadType) == "" || payloadType == metadatadomain.NormalizeResourceFormat("json") || payloadType == metadatadomain.NormalizeResourceFormat("yaml") {
		return nil
	}
	if len(filter) == 0 && len(suppress) == 0 && strings.TrimSpace(jq) == "" && validate == nil {
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
