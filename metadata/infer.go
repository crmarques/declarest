package metadata

import (
	"context"

	"github.com/crmarques/declarest/core"
)

func InferFromOpenAPI(_ context.Context, _ string, _ bool, _ bool) (ResourceMetadata, error) {
	return ResourceMetadata{}, core.ErrToBeImplemented
}
