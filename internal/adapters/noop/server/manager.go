package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

var _ server.Manager = (*Manager)(nil)

type Manager struct{}

func (m *Manager) Get(context.Context, resource.Info) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Create(context.Context, resource.Info) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Update(context.Context, resource.Info) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Delete(context.Context, resource.Info) error {
	return core.ErrToBeImplemented
}

func (m *Manager) List(context.Context, string, metadata.ResourceMetadata) ([]resource.Info, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Exists(context.Context, resource.Info) (bool, error) {
	return false, core.ErrToBeImplemented
}

func (m *Manager) GetOpenAPISpec(context.Context) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) BuildRequestFromMetadata(context.Context, resource.Info, string) (metadata.OperationSpec, error) {
	return metadata.OperationSpec{}, core.ErrToBeImplemented
}
