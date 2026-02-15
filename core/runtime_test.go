package core

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsrepository "github.com/crmarques/declarest/internal/providers/repository/fs"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
)

func TestBuildDefaultReconcilerWiring(t *testing.T) {
	t.Parallel()

	t.Run("filesystem_context_without_optional_managers", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "fs",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
		}

		defaultReconciler, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{
			Name:      "fs",
			Overrides: map[string]string{"repository.filesystem.base-dir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("buildDefaultReconciler returned error: %v", err)
		}

		if defaultReconciler.Name != "fs" {
			t.Fatalf("expected reconciler name fs, got %q", defaultReconciler.Name)
		}
		if _, ok := defaultReconciler.RepositoryManager.(*fsrepository.FSResourceRepository); !ok {
			t.Fatalf("expected FSResourceRepository, got %T", defaultReconciler.RepositoryManager)
		}
		if _, ok := defaultReconciler.MetadataService.(*fsmetadata.FSMetadataService); !ok {
			t.Fatalf("expected FSMetadataService, got %T", defaultReconciler.MetadataService)
		}
		if defaultReconciler.ServerManager != nil {
			t.Fatalf("expected nil server manager, got %T", defaultReconciler.ServerManager)
		}
		if defaultReconciler.SecretsProvider != nil {
			t.Fatalf("expected nil secrets provider, got %T", defaultReconciler.SecretsProvider)
		}
	})

	t.Run("git_context_with_http_server_and_file_secret_store", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "git-http-file-secret",
				Repository: config.Repository{
					Git: &config.GitRepository{
						Local: config.GitLocal{BaseDir: "/tmp/repo"},
					},
				},
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							BearerToken: &config.BearerTokenAuth{Token: "token"},
						},
					},
				},
				SecretStore: &config.SecretStore{
					File: &config.FileSecretStore{
						Path:       "/tmp/secrets.enc",
						Passphrase: "change-me",
					},
				},
			},
		}

		defaultReconciler, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{Name: "git-http-file-secret"})
		if err != nil {
			t.Fatalf("buildDefaultReconciler returned error: %v", err)
		}

		if _, ok := defaultReconciler.RepositoryManager.(*gitrepository.GitResourceRepository); !ok {
			t.Fatalf("expected GitResourceRepository, got %T", defaultReconciler.RepositoryManager)
		}
		if _, ok := defaultReconciler.ServerManager.(*httpserver.HTTPResourceServerGateway); !ok {
			t.Fatalf("expected HTTPResourceServerGateway, got %T", defaultReconciler.ServerManager)
		}
		if _, ok := defaultReconciler.SecretsProvider.(*filesecrets.FileSecretService); !ok {
			t.Fatalf("expected FileSecretService, got %T", defaultReconciler.SecretsProvider)
		}
	})

	t.Run("filesystem_context_with_vault_secret_store", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "fs-vault-secret",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				SecretStore: &config.SecretStore{
					Vault: &config.VaultSecretStore{
						Address: "https://vault.example.com",
						Auth: &config.VaultAuth{
							Token: "root-token",
						},
					},
				},
			},
		}

		defaultReconciler, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{Name: "fs-vault-secret"})
		if err != nil {
			t.Fatalf("buildDefaultReconciler returned error: %v", err)
		}

		if _, ok := defaultReconciler.RepositoryManager.(*fsrepository.FSResourceRepository); !ok {
			t.Fatalf("expected FSResourceRepository, got %T", defaultReconciler.RepositoryManager)
		}
		if _, ok := defaultReconciler.SecretsProvider.(*vaultsecrets.VaultSecretService); !ok {
			t.Fatalf("expected VaultSecretService, got %T", defaultReconciler.SecretsProvider)
		}
	})
}

func TestBuildDefaultReconcilerValidationAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil_context_service", func(t *testing.T) {
		t.Parallel()

		_, err := buildDefaultReconciler(context.Background(), nil, config.ContextSelection{})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("context_service_error_is_propagated", func(t *testing.T) {
		t.Parallel()

		expected := faults.NewTypedError(faults.NotFoundError, "context not found", nil)
		contextService := &fakeContextService{resolveErr: expected}

		_, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{Name: "missing"})
		if !errors.Is(err, expected) {
			t.Fatalf("expected propagated error %v, got %v", expected, err)
		}
	})

	t.Run("invalid_repository_provider", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name:       "invalid",
				Repository: config.Repository{},
			},
		}

		_, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{Name: "invalid"})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.InternalError)
	})

	t.Run("invalid_managed_server_provider_configuration", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "invalid-managed-server",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com/api",
						Auth: &config.HTTPAuth{
							OAuth2: &config.OAuth2{
								TokenURL:     "https://example.com/oauth/token",
								GrantType:    "password",
								ClientID:     "id",
								ClientSecret: "secret",
							},
						},
					},
				},
			},
		}

		_, err := buildDefaultReconciler(context.Background(), contextService, config.ContextSelection{Name: "invalid-managed-server"})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})
}

type fakeContextService struct {
	resolvedContext config.Context
	resolveErr      error
}

func (s *fakeContextService) Create(context.Context, config.Context) error {
	return nil
}

func (s *fakeContextService) Update(context.Context, config.Context) error {
	return nil
}

func (s *fakeContextService) Delete(context.Context, string) error {
	return nil
}

func (s *fakeContextService) Rename(context.Context, string, string) error {
	return nil
}

func (s *fakeContextService) List(context.Context) ([]config.Context, error) {
	return nil, nil
}

func (s *fakeContextService) SetCurrent(context.Context, string) error {
	return nil
}

func (s *fakeContextService) GetCurrent(context.Context) (config.Context, error) {
	return config.Context{}, nil
}

func (s *fakeContextService) ResolveContext(context.Context, config.ContextSelection) (config.Context, error) {
	if s.resolveErr != nil {
		return config.Context{}, s.resolveErr
	}
	return s.resolvedContext, nil
}

func (s *fakeContextService) Validate(context.Context, config.Context) error {
	return nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
