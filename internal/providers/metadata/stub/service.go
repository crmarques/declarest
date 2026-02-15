package stub

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var _ metadatadomain.MetadataService = (*StubMetadataService)(nil)

type StubMetadataService struct{}

func (s *StubMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, notimpl.Error("StubMetadataService", "Get")
}

func (s *StubMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return notimpl.Error("StubMetadataService", "Set")
}

func (s *StubMetadataService) Unset(context.Context, string) error {
	return notimpl.Error("StubMetadataService", "Unset")
}

func (s *StubMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, notimpl.Error("StubMetadataService", "ResolveForPath")
}

func (s *StubMetadataService) RenderOperationSpec(context.Context, string, metadatadomain.Operation, resource.Value) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, notimpl.Error("StubMetadataService", "RenderOperationSpec")
}

func (s *StubMetadataService) Infer(context.Context, string, metadatadomain.InferenceRequest) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, notimpl.Error("StubMetadataService", "Infer")
}
