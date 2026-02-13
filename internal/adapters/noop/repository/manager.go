package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

var _ repository.Manager = (*Manager)(nil)

type Manager struct{}

func (m *Manager) Save(context.Context, string, core.Resource) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Get(context.Context, string) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Delete(context.Context, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) List(context.Context, string) ([]resource.Info, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) Exists(context.Context, string) (bool, error) {
	return false, core.ErrToBeImplemented
}

func (m *Manager) Move(context.Context, string, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Init(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Refresh(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Reset(context.Context, bool) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Check(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Push(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) ForcePush(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) PullStatus(context.Context) (string, error) {
	return "", core.ErrToBeImplemented
}
