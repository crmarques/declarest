package file

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

var _ secrets.SecretProvider = (*FileSecretService)(nil)

type FileSecretService struct{}

func (s *FileSecretService) Init(context.Context) error {
	return notimpl.Error("FileSecretService", "Init")
}

func (s *FileSecretService) Store(context.Context, string, string) error {
	return notimpl.Error("FileSecretService", "Store")
}

func (s *FileSecretService) Get(context.Context, string) (string, error) {
	return "", notimpl.Error("FileSecretService", "Get")
}

func (s *FileSecretService) Delete(context.Context, string) error {
	return notimpl.Error("FileSecretService", "Delete")
}

func (s *FileSecretService) List(context.Context) ([]string, error) {
	return nil, notimpl.Error("FileSecretService", "List")
}

func (s *FileSecretService) MaskPayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("FileSecretService", "MaskPayload")
}

func (s *FileSecretService) ResolvePayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("FileSecretService", "ResolvePayload")
}

func (s *FileSecretService) NormalizeSecretPlaceholders(context.Context, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("FileSecretService", "NormalizeSecretPlaceholders")
}

func (s *FileSecretService) DetectSecretCandidates(context.Context, resource.Value) ([]string, error) {
	return nil, notimpl.Error("FileSecretService", "DetectSecretCandidates")
}
