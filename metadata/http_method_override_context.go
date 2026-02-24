package metadata

import (
	"context"
	"strings"
)

type operationHTTPMethodOverridesKey struct{}

type operationHTTPMethodOverrides map[Operation]string

func WithOperationHTTPMethodOverride(ctx context.Context, operation Operation, method string) context.Context {
	if ctx == nil || !operation.IsValid() {
		return ctx
	}

	normalized := strings.ToUpper(strings.TrimSpace(method))
	if normalized == "" {
		return ctx
	}

	current := cloneOperationHTTPMethodOverrides(operationHTTPMethodOverridesFromContext(ctx))
	if current == nil {
		current = operationHTTPMethodOverrides{}
	}
	current[operation] = normalized
	return context.WithValue(ctx, operationHTTPMethodOverridesKey{}, current)
}

func OperationHTTPMethodOverride(ctx context.Context, operation Operation) (string, bool) {
	if ctx == nil || !operation.IsValid() {
		return "", false
	}

	current := operationHTTPMethodOverridesFromContext(ctx)
	if len(current) == 0 {
		return "", false
	}
	value := strings.ToUpper(strings.TrimSpace(current[operation]))
	if value == "" {
		return "", false
	}
	return value, true
}

func operationHTTPMethodOverridesFromContext(ctx context.Context) operationHTTPMethodOverrides {
	if ctx == nil {
		return nil
	}
	current, _ := ctx.Value(operationHTTPMethodOverridesKey{}).(operationHTTPMethodOverrides)
	return current
}

func cloneOperationHTTPMethodOverrides(values operationHTTPMethodOverrides) operationHTTPMethodOverrides {
	if len(values) == 0 {
		return nil
	}

	cloned := make(operationHTTPMethodOverrides, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
