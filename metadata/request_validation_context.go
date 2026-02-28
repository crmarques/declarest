package metadata

import "context"

type requestOperationValidationContextKey struct{}

type requestOperationValidationContext struct {
	Operation Operation
	Resource  ResourceOperationSpecInput
	Validate  *OperationValidationSpec
}

// WithRequestOperationValidation stores operation validation metadata for
// generic request flows (for example `resource request post|put|patch`) so the
// resource-server adapter can enforce metadata validation rules.
func WithRequestOperationValidation(
	ctx context.Context,
	operation Operation,
	resourceInfo ResourceOperationSpecInput,
	validate *OperationValidationSpec,
) context.Context {
	if ctx == nil || !operation.IsValid() || validate == nil {
		return ctx
	}

	stored := requestOperationValidationContext{
		Operation: operation,
		Resource:  requestOperationSpecInputClone(resourceInfo),
		Validate:  cloneOperationValidationSpec(validate),
	}
	return context.WithValue(ctx, requestOperationValidationContextKey{}, stored)
}

func RequestOperationValidation(
	ctx context.Context,
) (Operation, ResourceOperationSpecInput, *OperationValidationSpec, bool) {
	if ctx == nil {
		return "", ResourceOperationSpecInput{}, nil, false
	}

	value, ok := ctx.Value(requestOperationValidationContextKey{}).(requestOperationValidationContext)
	if !ok || !value.Operation.IsValid() || value.Validate == nil {
		return "", ResourceOperationSpecInput{}, nil, false
	}

	return value.Operation, requestOperationSpecInputClone(value.Resource), cloneOperationValidationSpec(value.Validate), true
}

func requestOperationSpecInputClone(value ResourceOperationSpecInput) ResourceOperationSpecInput {
	return ResourceOperationSpecInput{
		LogicalPath:    value.LogicalPath,
		CollectionPath: value.CollectionPath,
		LocalAlias:     value.LocalAlias,
		RemoteID:       value.RemoteID,
		Metadata:       CloneResourceMetadata(value.Metadata),
		Payload:        value.Payload,
	}
}
