package metadata

import (
	"maps"
	"sort"
)

func CloneResourceMetadata(value ResourceMetadata) ResourceMetadata {
	cloned := ResourceMetadata{
		IDFromAttribute:        value.IDFromAttribute,
		AliasFromAttribute:     value.AliasFromAttribute,
		CollectionPath:         value.CollectionPath,
		PayloadType:            value.PayloadType,
		Secret:                 cloneBoolPointer(value.Secret),
		SecretsFromAttributes:  cloneStringSlice(value.SecretsFromAttributes),
		ExternalizedAttributes: cloneExternalizedAttributes(value.ExternalizedAttributes),
		Operations:             make(map[string]OperationSpec, len(value.Operations)),
		PayloadMutation:        clonePayloadMutationSteps(value.PayloadMutation),
	}

	for key, operationSpec := range value.Operations {
		cloned.Operations[key] = OperationSpec{
			Method:          operationSpec.Method,
			Path:            operationSpec.Path,
			Query:           maps.Clone(operationSpec.Query),
			Headers:         maps.Clone(operationSpec.Headers),
			Accept:          operationSpec.Accept,
			ContentType:     operationSpec.ContentType,
			Body:            operationSpec.Body,
			PayloadMutation: clonePayloadMutationSteps(operationSpec.PayloadMutation),
			Validate:        cloneOperationValidationSpec(operationSpec.Validate),
		}
	}

	return cloned
}

func MergeResourceMetadata(base ResourceMetadata, overlay ResourceMetadata) ResourceMetadata {
	merged := ResourceMetadata{
		IDFromAttribute:        base.IDFromAttribute,
		AliasFromAttribute:     base.AliasFromAttribute,
		CollectionPath:         base.CollectionPath,
		PayloadType:            base.PayloadType,
		Secret:                 cloneBoolPointer(base.Secret),
		SecretsFromAttributes:  cloneStringSlice(base.SecretsFromAttributes),
		ExternalizedAttributes: cloneExternalizedAttributes(base.ExternalizedAttributes),
		Operations:             cloneOperationMap(base.Operations),
		PayloadMutation:        clonePayloadMutationSteps(base.PayloadMutation),
	}

	if overlay.IDFromAttribute != "" {
		merged.IDFromAttribute = overlay.IDFromAttribute
	}
	if overlay.AliasFromAttribute != "" {
		merged.AliasFromAttribute = overlay.AliasFromAttribute
	}
	if overlay.CollectionPath != "" {
		merged.CollectionPath = overlay.CollectionPath
	}
	if overlay.PayloadType != "" {
		merged.PayloadType = overlay.PayloadType
	}
	if overlay.Secret != nil {
		merged.Secret = cloneBoolPointer(overlay.Secret)
	}
	if overlay.SecretsFromAttributes != nil {
		merged.SecretsFromAttributes = cloneStringSlice(overlay.SecretsFromAttributes)
	}
	if overlay.ExternalizedAttributes != nil {
		merged.ExternalizedAttributes = cloneExternalizedAttributes(overlay.ExternalizedAttributes)
	}
	if overlay.Operations != nil {
		if merged.Operations == nil {
			merged.Operations = map[string]OperationSpec{}
		}
		keys := sortedOperationKeys(overlay.Operations)
		for _, key := range keys {
			merged.Operations[key] = MergeOperationSpec(merged.Operations[key], overlay.Operations[key])
		}
	}
	if overlay.PayloadMutation != nil {
		merged.PayloadMutation = clonePayloadMutationSteps(overlay.PayloadMutation)
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

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func MergeOperationSpec(base OperationSpec, overlay OperationSpec) OperationSpec {
	merged := OperationSpec{
		Method:          base.Method,
		Path:            base.Path,
		Query:           maps.Clone(base.Query),
		Headers:         maps.Clone(base.Headers),
		Accept:          base.Accept,
		ContentType:     base.ContentType,
		Body:            base.Body,
		PayloadMutation: clonePayloadMutationSteps(base.PayloadMutation),
		Validate:        cloneOperationValidationSpec(base.Validate),
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
			keys := sortedMapKeys(overlay.Query)
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
			keys := sortedMapKeys(overlay.Headers)
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
	if overlay.PayloadMutation != nil {
		merged.PayloadMutation = clonePayloadMutationSteps(overlay.PayloadMutation)
	}
	merged.Validate = mergeOperationValidationSpec(merged.Validate, overlay.Validate)

	return merged
}

func sortedOperationKeys(values map[string]OperationSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneOperationMap(values map[string]OperationSpec) map[string]OperationSpec {
	if values == nil {
		return nil
	}

	cloned := make(map[string]OperationSpec, len(values))
	for key, value := range values {
		cloned[key] = OperationSpec{
			Method:          value.Method,
			Path:            value.Path,
			Query:           maps.Clone(value.Query),
			Headers:         maps.Clone(value.Headers),
			Accept:          value.Accept,
			ContentType:     value.ContentType,
			Body:            value.Body,
			PayloadMutation: clonePayloadMutationSteps(value.PayloadMutation),
			Validate:        cloneOperationValidationSpec(value.Validate),
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

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
