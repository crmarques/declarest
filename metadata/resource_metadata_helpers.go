package metadata

import (
	"maps"
	"slices"
	"strings"

	"github.com/crmarques/declarest/resource"
)

func HasResourceMetadataDirectives(value ResourceMetadata) bool {
	return strings.TrimSpace(value.ID) != "" ||
		strings.TrimSpace(value.Alias) != "" ||
		value.RequiredAttributes != nil ||
		strings.TrimSpace(value.RemoteCollectionPath) != "" ||
		strings.TrimSpace(value.PayloadType) != "" ||
		strings.TrimSpace(value.DefaultFormat) != "" ||
		value.Secret != nil ||
		value.SecretAttributes != nil ||
		value.ExternalizedAttributes != nil ||
		value.Operations != nil ||
		value.Transforms != nil
}

func CloneResourceMetadata(value ResourceMetadata) ResourceMetadata {
	cloned := ResourceMetadata{
		ID:                     value.ID,
		Alias:                  value.Alias,
		RequiredAttributes:     cloneStringSlice(value.RequiredAttributes),
		RemoteCollectionPath:   value.RemoteCollectionPath,
		PayloadType:            value.PayloadType,
		DefaultFormat:          value.DefaultFormat,
		Secret:                 cloneBoolPointer(value.Secret),
		SecretAttributes:       cloneStringSlice(value.SecretAttributes),
		ExternalizedAttributes: cloneExternalizedAttributes(value.ExternalizedAttributes),
		Operations:             make(map[string]OperationSpec, len(value.Operations)),
		Transforms:             cloneTransformSteps(value.Transforms),
	}

	for key, operationSpec := range value.Operations {
		cloned.Operations[key] = OperationSpec{
			Method:      operationSpec.Method,
			Path:        operationSpec.Path,
			Query:       maps.Clone(operationSpec.Query),
			Headers:     maps.Clone(operationSpec.Headers),
			Accept:      operationSpec.Accept,
			ContentType: operationSpec.ContentType,
			Body:        resource.DeepCopyValue(operationSpec.Body),
			Transforms:  cloneTransformSteps(operationSpec.Transforms),
			Validate:    cloneOperationValidationSpec(operationSpec.Validate),
		}
	}

	return cloned
}

func MergeResourceMetadata(base ResourceMetadata, overlay ResourceMetadata) ResourceMetadata {
	merged := ResourceMetadata{
		ID:                     base.ID,
		Alias:                  base.Alias,
		RequiredAttributes:     cloneStringSlice(base.RequiredAttributes),
		RemoteCollectionPath:   base.RemoteCollectionPath,
		PayloadType:            base.PayloadType,
		DefaultFormat:          base.DefaultFormat,
		Secret:                 cloneBoolPointer(base.Secret),
		SecretAttributes:       cloneStringSlice(base.SecretAttributes),
		ExternalizedAttributes: cloneExternalizedAttributes(base.ExternalizedAttributes),
		Operations:             cloneOperationMap(base.Operations),
		Transforms:             cloneTransformSteps(base.Transforms),
	}

	if overlay.ID != "" {
		merged.ID = overlay.ID
	}
	if overlay.Alias != "" {
		merged.Alias = overlay.Alias
	}
	if overlay.RequiredAttributes != nil {
		merged.RequiredAttributes = cloneStringSlice(overlay.RequiredAttributes)
	}
	if overlay.RemoteCollectionPath != "" {
		merged.RemoteCollectionPath = overlay.RemoteCollectionPath
	}
	if overlay.PayloadType != "" {
		merged.PayloadType = overlay.PayloadType
	}
	if overlay.DefaultFormat != "" {
		merged.DefaultFormat = overlay.DefaultFormat
	}
	if overlay.Secret != nil {
		merged.Secret = cloneBoolPointer(overlay.Secret)
	}
	if overlay.SecretAttributes != nil {
		merged.SecretAttributes = cloneStringSlice(overlay.SecretAttributes)
	}
	if overlay.ExternalizedAttributes != nil {
		merged.ExternalizedAttributes = cloneExternalizedAttributes(overlay.ExternalizedAttributes)
	}
	if overlay.Operations != nil {
		if merged.Operations == nil {
			merged.Operations = map[string]OperationSpec{}
		}
		keys := slices.Sorted(maps.Keys(overlay.Operations))
		for _, key := range keys {
			merged.Operations[key] = MergeOperationSpec(merged.Operations[key], overlay.Operations[key])
		}
	}
	if overlay.Transforms != nil {
		merged.Transforms = cloneTransformSteps(overlay.Transforms)
	}

	return merged
}

func cloneExternalizedAttributes(values []ExternalizedAttribute) []ExternalizedAttribute {
	if values == nil {
		return nil
	}

	cloned := make([]ExternalizedAttribute, len(values))
	for idx := range values {
		cloned[idx] = ExternalizedAttribute{
			Path:           values[idx].Path,
			File:           values[idx].File,
			Template:       values[idx].Template,
			Mode:           values[idx].Mode,
			SaveBehavior:   values[idx].SaveBehavior,
			RenderBehavior: values[idx].RenderBehavior,
			Enabled:        cloneBoolPointer(values[idx].Enabled),
		}
	}
	return cloned
}

func MergeOperationSpec(base OperationSpec, overlay OperationSpec) OperationSpec {
	merged := OperationSpec{
		Method:      base.Method,
		Path:        base.Path,
		Query:       maps.Clone(base.Query),
		Headers:     maps.Clone(base.Headers),
		Accept:      base.Accept,
		ContentType: base.ContentType,
		Body:        resource.DeepCopyValue(base.Body),
		Transforms:  cloneTransformSteps(base.Transforms),
		Validate:    cloneOperationValidationSpec(base.Validate),
	}

	if overlay.Method != "" {
		merged.Method = overlay.Method
	}
	if overlay.Path != "" {
		merged.Path = overlay.Path
	}
	if overlay.Query != nil {
		if len(overlay.Query) == 0 {
			merged.Query = map[string]string{}
		} else {
			if merged.Query == nil {
				merged.Query = make(map[string]string, len(overlay.Query))
			}
			keys := slices.Sorted(maps.Keys(overlay.Query))
			for _, key := range keys {
				merged.Query[key] = overlay.Query[key]
			}
		}
	}
	if overlay.Headers != nil {
		if len(overlay.Headers) == 0 {
			merged.Headers = map[string]string{}
		} else {
			if merged.Headers == nil {
				merged.Headers = make(map[string]string, len(overlay.Headers))
			}
			keys := slices.Sorted(maps.Keys(overlay.Headers))
			for _, key := range keys {
				merged.Headers[key] = overlay.Headers[key]
			}
		}
	}
	if overlay.Accept != "" {
		merged.Accept = overlay.Accept
	}
	if overlay.ContentType != "" {
		merged.ContentType = overlay.ContentType
	}
	if overlay.Body != nil {
		merged.Body = overlay.Body
	}
	if overlay.Transforms != nil {
		merged.Transforms = cloneTransformSteps(overlay.Transforms)
	}
	merged.Validate = mergeOperationValidationSpec(merged.Validate, overlay.Validate)

	return merged
}

func cloneOperationMap(values map[string]OperationSpec) map[string]OperationSpec {
	if values == nil {
		return nil
	}

	cloned := make(map[string]OperationSpec, len(values))
	for key, value := range values {
		cloned[key] = OperationSpec{
			Method:      value.Method,
			Path:        value.Path,
			Query:       maps.Clone(value.Query),
			Headers:     maps.Clone(value.Headers),
			Accept:      value.Accept,
			ContentType: value.ContentType,
			Body:        resource.DeepCopyValue(value.Body),
			Transforms:  cloneTransformSteps(value.Transforms),
			Validate:    cloneOperationValidationSpec(value.Validate),
		}
	}
	return cloned
}

func cloneOperationValidationSpec(value *OperationValidationSpec) *OperationValidationSpec {
	if value == nil {
		return nil
	}

	cloned := &OperationValidationSpec{
		RequiredAttributes: cloneStringSlice(value.RequiredAttributes),
		Assertions:         cloneValidationAssertions(value.Assertions),
		SchemaRef:          value.SchemaRef,
	}
	return cloned
}

func cloneValidationAssertions(values []ValidationAssertion) []ValidationAssertion {
	if values == nil {
		return nil
	}

	cloned := make([]ValidationAssertion, len(values))
	copy(cloned, values)
	return cloned
}

func mergeOperationValidationSpec(
	base *OperationValidationSpec,
	overlay *OperationValidationSpec,
) *OperationValidationSpec {
	if base == nil && overlay == nil {
		return nil
	}
	if overlay == nil {
		return cloneOperationValidationSpec(base)
	}

	merged := cloneOperationValidationSpec(base)
	if merged == nil {
		merged = &OperationValidationSpec{}
	}

	if overlay.RequiredAttributes != nil {
		merged.RequiredAttributes = cloneStringSlice(overlay.RequiredAttributes)
	}
	if overlay.Assertions != nil {
		merged.Assertions = cloneValidationAssertions(overlay.Assertions)
	}
	if overlay.SchemaRef != "" {
		merged.SchemaRef = overlay.SchemaRef
	}

	return merged
}
