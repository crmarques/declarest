package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/ctx"
)

var _ ctx.Manager = (*Manager)(nil)

type Manager struct{}

func (m *Manager) Create(context.Context, ctx.Config) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Update(context.Context, ctx.Config) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Delete(context.Context, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Rename(context.Context, string, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) List(context.Context) ([]ctx.Config, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) SetCurrent(context.Context, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) GetCurrent(context.Context) (ctx.Config, error) {
	return ctx.Config{}, core.ErrToBeImplemented
}

func (m *Manager) LoadResolvedConfig(context.Context, string, map[string]string) (ctx.Runtime, error) {
	return ctx.Runtime{}, core.ErrToBeImplemented
}

func (m *Manager) Validate(context.Context, ctx.Config) error {
	return core.ErrToBeImplemented
}
