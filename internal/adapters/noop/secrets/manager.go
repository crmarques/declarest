package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/secrets"
)

var _ secrets.Manager = (*Manager)(nil)

type Manager struct{}

func (m *Manager) Init(context.Context) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Store(context.Context, string, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) Get(context.Context, string) (string, error) {
	return "", core.ErrToBeImplemented
}

func (m *Manager) Delete(context.Context, string) error {
	return core.ErrToBeImplemented
}

func (m *Manager) List(context.Context) ([]string, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) MaskPayload(context.Context, core.Resource) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) ResolvePayload(context.Context, core.Resource) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) NormalizeSecretPlaceholders(context.Context, core.Resource) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (m *Manager) DetectSecretCandidates(context.Context, core.Resource) ([]string, error) {
	return nil, core.ErrToBeImplemented
}
