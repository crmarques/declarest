package http

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

var _ server.RemoteResourceGateway = (*HTTPRemoteResourceGateway)(nil)

type HTTPRemoteResourceGateway struct{}

func (g *HTTPRemoteResourceGateway) Get(context.Context, resource.Resource) (resource.Value, error) {
	return nil, notimpl.Error("HTTPRemoteResourceGateway", "Get")
}

func (g *HTTPRemoteResourceGateway) Create(context.Context, resource.Resource) (resource.Value, error) {
	return nil, notimpl.Error("HTTPRemoteResourceGateway", "Create")
}

func (g *HTTPRemoteResourceGateway) Update(context.Context, resource.Resource) (resource.Value, error) {
	return nil, notimpl.Error("HTTPRemoteResourceGateway", "Update")
}

func (g *HTTPRemoteResourceGateway) Delete(context.Context, resource.Resource) error {
	return notimpl.Error("HTTPRemoteResourceGateway", "Delete")
}

func (g *HTTPRemoteResourceGateway) List(context.Context, string, metadata.ResourceMetadata) ([]resource.Resource, error) {
	return nil, notimpl.Error("HTTPRemoteResourceGateway", "List")
}

func (g *HTTPRemoteResourceGateway) Exists(context.Context, resource.Resource) (bool, error) {
	return false, notimpl.Error("HTTPRemoteResourceGateway", "Exists")
}

func (g *HTTPRemoteResourceGateway) GetOpenAPISpec(context.Context) (resource.Value, error) {
	return nil, notimpl.Error("HTTPRemoteResourceGateway", "GetOpenAPISpec")
}

func (g *HTTPRemoteResourceGateway) BuildRequestFromMetadata(context.Context, resource.Resource, metadata.Operation) (metadata.OperationSpec, error) {
	return metadata.OperationSpec{}, notimpl.Error("HTTPRemoteResourceGateway", "BuildRequestFromMetadata")
}
