package metadata

import "sort"

func CloneResourceMetadata(value ResourceMetadata) ResourceMetadata {
	cloned := ResourceMetadata{
		IDFromAttribute:       value.IDFromAttribute,
		AliasFromAttribute:    value.AliasFromAttribute,
		CollectionPath:        value.CollectionPath,
		SecretsFromAttributes: cloneStringSlice(value.SecretsFromAttributes),
		Operations:            make(map[string]OperationSpec, len(value.Operations)),
		Filter:                cloneStringSlice(value.Filter),
		Suppress:              cloneStringSlice(value.Suppress),
		JQ:                    value.JQ,
		PayloadTransformOrder: cloneStringSlice(value.PayloadTransformOrder),
	}

	for key, operationSpec := range value.Operations {
		cloned.Operations[key] = OperationSpec{
			Method:                operationSpec.Method,
			Path:                  operationSpec.Path,
			Query:                 cloneStringMap(operationSpec.Query),
			Headers:               cloneStringMap(operationSpec.Headers),
			Accept:                operationSpec.Accept,
			ContentType:           operationSpec.ContentType,
			Body:                  operationSpec.Body,
			Filter:                cloneStringSlice(operationSpec.Filter),
			Suppress:              cloneStringSlice(operationSpec.Suppress),
			JQ:                    operationSpec.JQ,
			Validate:              cloneOperationValidationSpec(operationSpec.Validate),
			PayloadTransformOrder: cloneStringSlice(operationSpec.PayloadTransformOrder),
		}
	}

	return cloned
}

func MergeResourceMetadata(base ResourceMetadata, overlay ResourceMetadata) ResourceMetadata {
	merged := ResourceMetadata{
		IDFromAttribute:       base.IDFromAttribute,
		AliasFromAttribute:    base.AliasFromAttribute,
		CollectionPath:        base.CollectionPath,
		SecretsFromAttributes: cloneStringSlice(base.SecretsFromAttributes),
		Operations:            cloneOperationMap(base.Operations),
		Filter:                cloneStringSlice(base.Filter),
		Suppress:              cloneStringSlice(base.Suppress),
		JQ:                    base.JQ,
		PayloadTransformOrder: cloneStringSlice(base.PayloadTransformOrder),
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
	if overlay.SecretsFromAttributes != nil {
		merged.SecretsFromAttributes = cloneStringSlice(overlay.SecretsFromAttributes)
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
	if overlay.Filter != nil {
		merged.Filter = cloneStringSlice(overlay.Filter)
	}
	if overlay.Suppress != nil {
		merged.Suppress = cloneStringSlice(overlay.Suppress)
	}
	if overlay.JQ != "" {
		merged.JQ = overlay.JQ
	}
	merged.PayloadTransformOrder = mergePayloadTransformOrder(merged.PayloadTransformOrder, overlay.PayloadTransformOrder)

	return merged
}

func MergeOperationSpec(base OperationSpec, overlay OperationSpec) OperationSpec {
	merged := OperationSpec{
		Method:                base.Method,
		Path:                  base.Path,
		Query:                 cloneStringMap(base.Query),
		Headers:               cloneStringMap(base.Headers),
		Accept:                base.Accept,
		ContentType:           base.ContentType,
		Body:                  base.Body,
		Filter:                cloneStringSlice(base.Filter),
		Suppress:              cloneStringSlice(base.Suppress),
		JQ:                    base.JQ,
		Validate:              cloneOperationValidationSpec(base.Validate),
		PayloadTransformOrder: cloneStringSlice(base.PayloadTransformOrder),
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
	if overlay.Filter != nil {
		merged.Filter = cloneStringSlice(overlay.Filter)
	}
	if overlay.Suppress != nil {
		merged.Suppress = cloneStringSlice(overlay.Suppress)
	}
	if overlay.JQ != "" {
		merged.JQ = overlay.JQ
	}
	merged.Validate = mergeOperationValidationSpec(merged.Validate, overlay.Validate)
	merged.PayloadTransformOrder = mergePayloadTransformOrder(merged.PayloadTransformOrder, overlay.PayloadTransformOrder)

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
			Method:                value.Method,
			Path:                  value.Path,
			Query:                 cloneStringMap(value.Query),
			Headers:               cloneStringMap(value.Headers),
			Accept:                value.Accept,
			ContentType:           value.ContentType,
			Body:                  value.Body,
			Filter:                cloneStringSlice(value.Filter),
			Suppress:              cloneStringSlice(value.Suppress),
			JQ:                    value.JQ,
			Validate:              cloneOperationValidationSpec(value.Validate),
			PayloadTransformOrder: cloneStringSlice(value.PayloadTransformOrder),
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

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
