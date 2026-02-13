package resource

func CloneOperationPayloadConfig(src *OperationPayloadConfig) *OperationPayloadConfig {
	if src == nil {
		return nil
	}
	return &OperationPayloadConfig{
		SuppressAttributes: append([]string{}, src.SuppressAttributes...),
		FilterAttributes:   append([]string{}, src.FilterAttributes...),
		JQExpression:       src.JQExpression,
	}
}

func CloneMapStringAny(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
