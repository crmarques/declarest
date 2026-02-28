package fsmetadata

import (
	"fmt"
	"strings"

	metadatadomain "github.com/crmarques/declarest/metadata"
)

func validateResourceMetadata(metadata metadatadomain.ResourceMetadata) error {
	keys := sortedOperationKeys(metadata.Operations)
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return validationError(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}

		operationSpec := metadata.Operations[key]
		if err := validateOperationValidationSpec(metadatadomain.Operation(key), operationSpec.Validate); err != nil {
			return err
		}
	}
	return nil
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
			return validationError(
				fmt.Sprintf("operation %q validate.requiredAttributes[%d] must not be empty", operation, idx),
				nil,
			)
		}
	}

	for idx, assertion := range spec.Assertions {
		if strings.TrimSpace(assertion.JQ) == "" {
			return validationError(
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
	return validationError(
		fmt.Sprintf(
			"operation %q validate.schemaRef %q is not supported (expected openapi:request-body or openapi:#/...)",
			operation,
			schemaRef,
		),
		nil,
	)
}
