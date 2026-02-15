package vault

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

var _ secrets.SecretProvider = (*VaultSecretService)(nil)

type VaultSecretService struct{}

func (s *VaultSecretService) Init(context.Context) error {
	return notimpl.Error("VaultSecretService", "Init")
}

func (s *VaultSecretService) Store(context.Context, string, string) error {
	return notimpl.Error("VaultSecretService", "Store")
}

func (s *VaultSecretService) Get(context.Context, string) (string, error) {
	return "", notimpl.Error("VaultSecretService", "Get")
}

func (s *VaultSecretService) Delete(context.Context, string) error {
	return notimpl.Error("VaultSecretService", "Delete")
}

func (s *VaultSecretService) List(context.Context) ([]string, error) {
	return nil, notimpl.Error("VaultSecretService", "List")
}

func (s *VaultSecretService) MaskPayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("VaultSecretService", "MaskPayload")
}

func (s *VaultSecretService) ResolvePayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("VaultSecretService", "ResolvePayload")
}

func (s *VaultSecretService) NormalizeSecretPlaceholders(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("VaultSecretService", "NormalizeSecretPlaceholders")
}

func (s *VaultSecretService) DetectSecretCandidates(context.Context, resource.Value) ([]string, error) {
	return nil, notimpl.Error("VaultSecretService", "DetectSecretCandidates")
}
