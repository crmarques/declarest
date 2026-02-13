package metadata

import (
	"context"

	"github.com/crmarques/declarest/core"
)

func ResolveOperationSpec(_ context.Context, _ ResourceMetadata, _ string, _ core.Resource) (OperationSpec, error) {
	return OperationSpec{}, core.ErrToBeImplemented
}
