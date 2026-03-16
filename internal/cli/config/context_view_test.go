// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestSelectContextForViewCompactsDefaultMetadataBaseDir(t *testing.T) {
	t.Parallel()

	contexts := []configdomain.Context{
		{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
		},
	}

	selected, idx, err := selectContextForView(contexts, "dev")
	if err != nil {
		t.Fatalf("selectContextForView returned error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected selected index 0, got %d", idx)
	}
	if selected.Metadata.BaseDir != "" {
		t.Fatalf("expected default metadata base-dir to be compacted, got %q", selected.Metadata.BaseDir)
	}
}

func TestSelectContextForViewReturnsNotFound(t *testing.T) {
	t.Parallel()

	_, _, err := selectContextForView([]configdomain.Context{{Name: "dev"}}, "prod")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != faults.NotFoundError {
		t.Fatalf("expected not found category, got %q", typedErr.Category)
	}
}

func TestCompactContextCatalogForViewCompactsEntries(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
		},
		CurrentContext: "dev",
	}

	compacted := compactContextCatalogForView(catalog)
	if compacted.Contexts[0].Metadata.BaseDir != "" {
		t.Fatalf("expected compacted metadata base-dir, got %q", compacted.Contexts[0].Metadata.BaseDir)
	}
}

func TestSelectSingleContextEditViewPreservesCatalogScopedCredentials(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		DefaultEditor: "vim",
		Credentials: []configdomain.Credential{{
			Name: "shared-proxy-auth",
			Username: configdomain.CredentialValue{
				Prompt: &configdomain.CredentialPrompt{Prompt: true, PersistInSession: true},
			},
			Password: configdomain.CredentialValue{
				Prompt: &configdomain.CredentialPrompt{Prompt: true, PersistInSession: true},
			},
		}},
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
		},
	}

	view, idx, err := selectSingleContextEditView(catalog, "dev")
	if err != nil {
		t.Fatalf("selectSingleContextEditView returned error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected selected index 0, got %d", idx)
	}
	if view.DefaultEditor != "vim" {
		t.Fatalf("expected default editor vim, got %q", view.DefaultEditor)
	}
	if len(view.Credentials) != 1 || view.Credentials[0].Name != "shared-proxy-auth" {
		t.Fatalf("expected shared credential in edit view, got %#v", view.Credentials)
	}
	if !view.Credentials[0].Username.IsPrompt() || !view.Credentials[0].Username.PersistInSession() {
		t.Fatalf("expected prompt-backed username credential, got %#v", view.Credentials[0].Username)
	}
	if view.Context.Metadata.BaseDir != "" {
		t.Fatalf("expected default metadata base-dir to be compacted, got %q", view.Context.Metadata.BaseDir)
	}
}

func TestApplySingleContextEditViewUpdatesCatalogScopedAttributes(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		CurrentContext: "dev",
		DefaultEditor:  "vim",
		Credentials: []configdomain.Credential{{
			Name:     "shared",
			Username: configdomain.LiteralCredential("old-user"),
			Password: configdomain.LiteralCredential("old-pass"),
		}},
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
			{
				Name: "prod",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
				},
			},
		},
	}

	updated := applySingleContextEditView(catalog, 0, singleContextEditView{
		DefaultEditor: "nvim",
		Credentials: []configdomain.Credential{{
			Name:     "shared-proxy-auth",
			Username: configdomain.LiteralCredential("new-user"),
			Password: configdomain.LiteralCredential("new-pass"),
		}},
		Context: configdomain.Context{
			Name: "dev-renamed",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/renamed"},
			},
		},
	})

	if updated.DefaultEditor != "nvim" {
		t.Fatalf("expected updated default editor nvim, got %q", updated.DefaultEditor)
	}
	if updated.CurrentContext != "dev-renamed" {
		t.Fatalf("expected current context to follow renamed edited context, got %q", updated.CurrentContext)
	}
	if len(updated.Credentials) != 1 || updated.Credentials[0].Name != "shared-proxy-auth" {
		t.Fatalf("expected updated credentials, got %#v", updated.Credentials)
	}
	if updated.Contexts[0].Name != "dev-renamed" {
		t.Fatalf("expected updated edited context name, got %q", updated.Contexts[0].Name)
	}
	if updated.Contexts[1].Name != "prod" {
		t.Fatalf("expected untouched second context, got %q", updated.Contexts[1].Name)
	}
}

func TestSelectContextCatalogForShowPreservesCatalogAttributes(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		CurrentContext: "other",
		DefaultEditor:  "vim",
		Credentials: []configdomain.Credential{{
			Name:     "shared-proxy-auth",
			Username: configdomain.LiteralCredential("demo-user"),
			Password: configdomain.LiteralCredential("demo-pass"),
		}},
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
			{
				Name: "other",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/other"},
				},
			},
		},
	}

	shown, err := selectContextCatalogForShow(catalog, "dev")
	if err != nil {
		t.Fatalf("selectContextCatalogForShow returned error: %v", err)
	}
	if shown.CurrentContext != "dev" {
		t.Fatalf("expected shown current context dev, got %q", shown.CurrentContext)
	}
	if shown.DefaultEditor != "vim" {
		t.Fatalf("expected default editor vim, got %q", shown.DefaultEditor)
	}
	if len(shown.Credentials) != 1 || shown.Credentials[0].Name != "shared-proxy-auth" {
		t.Fatalf("expected credentials to be preserved, got %#v", shown.Credentials)
	}
	if len(shown.Contexts) != 1 || shown.Contexts[0].Name != "dev" {
		t.Fatalf("expected one shown context dev, got %#v", shown.Contexts)
	}
	if shown.Contexts[0].Metadata.BaseDir != "/tmp/repo" {
		t.Fatalf("expected metadata base-dir to be preserved, got %q", shown.Contexts[0].Metadata.BaseDir)
	}
}
