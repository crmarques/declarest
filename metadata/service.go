package metadata

import (
	"context"

	"github.com/crmarques/declarest/faults"
)

func ResolveOperationSpec(_ context.Context, _ ResourceMetadata, _ Operation, _ any) (OperationSpec, error) {
	return OperationSpec{}, faults.ErrToBeImplemented
}

func InferFromOpenAPI(_ context.Context, _ string, _ InferenceRequest) (ResourceMetadata, error) {
	return ResourceMetadata{}, faults.ErrToBeImplemented
}
