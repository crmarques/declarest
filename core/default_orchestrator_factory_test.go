package core

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
)

func TestBuildDefaultOrchestratorWiring(t *testing.T) {
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

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{
			Name:      "fs",
			Overrides: map[string]string{"repository.filesystem.base-dir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Metadata.(*fsmetadata.FSMetadataService); !ok {
			t.Fatalf("expected FSMetadataService, got %T", defaultOrchestrator.Metadata)
		}
		if defaultOrchestrator.Server != nil {
			t.Fatalf("expected nil server manager, got %T", defaultOrchestrator.Server)
		}
		if defaultOrchestrator.Secrets != nil {
			t.Fatalf("expected nil secrets provider, got %T", defaultOrchestrator.Secrets)
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
				ResourceServer: &config.ResourceServer{
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

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "git-http-file-secret"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*gitrepository.GitResourceRepository); !ok {
			t.Fatalf("expected GitResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Server.(*httpserver.HTTPResourceServerGateway); !ok {
			t.Fatalf("expected HTTPResourceServerGateway, got %T", defaultOrchestrator.Server)
		}
		if _, ok := defaultOrchestrator.Secrets.(*filesecrets.FileSecretService); !ok {
			t.Fatalf("expected FileSecretService, got %T", defaultOrchestrator.Secrets)
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

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "fs-vault-secret"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Secrets.(*vaultsecrets.VaultSecretService); !ok {
			t.Fatalf("expected VaultSecretService, got %T", defaultOrchestrator.Secrets)
		}
	})
}

func TestBuildDefaultOrchestratorValidationAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil_context_service", func(t *testing.T) {
		t.Parallel()

		_, err := buildDefaultOrchestrator(context.Background(), nil, config.ContextSelection{})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("context_service_error_is_propagated", func(t *testing.T) {
		t.Parallel()

		expected := faults.NewTypedError(faults.NotFoundError, "context not found", nil)
		contextService := &fakeContextService{resolveErr: expected}

		_, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "missing"})
		if !errors.Is(err, expected) {
			t.Fatalf("expected propagated error %v, got %v", expected, err)
		}
	})

	t.Run("missing_repository_provider_is_allowed", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name:       "invalid",
				Repository: config.Repository{},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid"})
		if err != nil {
			t.Fatalf("expected missing repository to be allowed, got error: %v", err)
		}
		if defaultOrchestrator.Repository != nil {
			t.Fatalf("expected nil repository manager, got %T", defaultOrchestrator.Repository)
		}
	})

	t.Run("invalid_resource_server_provider_configuration", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "invalid-resource-server",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				ResourceServer: &config.ResourceServer{
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

		_, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid-resource-server"})
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
