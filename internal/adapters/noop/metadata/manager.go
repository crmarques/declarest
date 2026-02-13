package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/metadata"
)

var _ metadata.Manager = (*Manager)(nil)

type Manager struct{}

func (m *Manager) Get(context.Context, string) (metadata.ResourceMetadata, error) {
	return metadata.ResourceMetadata{}, core.ErrToBeImplemented
}

func (m *Manager) Set(context.Context, string, metadata.ResourceMetadata) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Unset(context.Context, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) ResolveForPath(context.Context, string) (metadata.ResourceMetadata, error) {
	return metadata.ResourceMetadata{}, core.ErrToBeImplemented
}

func (m *Manager) RenderOperationSpec(context.Context, string, string, core.Resource) (metadata.OperationSpec, error) {
	return metadata.OperationSpec{}, core.ErrToBeImplemented
}

func (m *Manager) Infer(context.Context, string, bool, bool) (metadata.ResourceMetadata, error) {
	return metadata.ResourceMetadata{}, core.ErrToBeImplemented
}
