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

package file

import (
	"testing"

	"github.com/crmarques/declarest/config"
)

func TestInjectContextCredentialsReusesNamedCredentialAcrossReferences(t *testing.T) {
	t.Parallel()

	cfg := config.Context{
		Name: "shared",
		Repository: config.Repository{
			Git: &config.GitRepository{
				Local: config.GitLocal{BaseDir: "/tmp/repo"},
				Remote: &config.GitRemote{
					URL: "https://example.com/config.git",
					Auth: &config.GitAuth{
						Basic: &config.BasicAuth{
							CredentialsRef: &config.CredentialsRef{Name: "shared"},
						},
					},
				},
			},
		},
		ManagedServer: &config.ManagedServer{
			HTTP: &config.HTTPServer{
				BaseURL: "https://api.example.com",
				Auth: &config.HTTPAuth{
					Basic: &config.BasicAuth{
						CredentialsRef: &config.CredentialsRef{Name: "shared"},
					},
				},
			},
		},
	}

	resolved, err := injectContextCredentials(cfg, map[string]config.Credential{
		"shared": {
			Name:     "shared",
			Username: config.LiteralCredential("demo-user"),
			Password: config.LiteralCredential("demo-pass"),
		},
	})
	if err != nil {
		t.Fatalf("injectContextCredentials() returned error: %v", err)
	}

	repoAuth := resolved.Repository.Git.Remote.Auth.Basic
	serverAuth := resolved.ManagedServer.HTTP.Auth.Basic
	if repoAuth == nil || serverAuth == nil {
		t.Fatalf("expected repository and managed-server basic auth to be configured, got %#v %#v", repoAuth, serverAuth)
	}
	if repoAuth.CredentialName() != "shared" || serverAuth.CredentialName() != "shared" {
		t.Fatalf("expected both auth blocks to retain credentialsRef name shared, got %q and %q", repoAuth.CredentialName(), serverAuth.CredentialName())
	}
	if repoAuth.Username.Literal() != "demo-user" || repoAuth.Password.Literal() != "demo-pass" {
		t.Fatalf("expected repository auth to receive injected credential values, got %#v", repoAuth)
	}
	if serverAuth.Username.Literal() != "demo-user" || serverAuth.Password.Literal() != "demo-pass" {
		t.Fatalf("expected managed-server auth to receive injected credential values, got %#v", serverAuth)
	}
}
