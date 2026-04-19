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

package vault

import (
	"context"
	"net/http"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
)

type vaultAuthInfo struct {
	ClientToken string `json:"client_token"`
}

func (s *Store) loginUserPass(ctx context.Context) error {
	credentials := s.auth.userPass
	if credentials == nil {
		return faults.Invalid("vault userpass auth configuration is invalid", nil)
	}

	mount, err := normalizeVaultPath(credentials.Mount, true)
	if err != nil {
		return faults.Invalid("secret-store.vault.auth.password.mount is invalid", err)
	}
	if mount == "" {
		mount = "userpass"
	}

	creds, err := promptauth.ResolveCredentials(
		s.runtime,
		ctx,
		credentials.CredentialName(),
		credentials.Username,
		credentials.Password,
	)
	if err != nil {
		return err
	}
	username := strings.TrimSpace(creds.Username)
	password := strings.TrimSpace(creds.Password)
	if username == "" || password == "" {
		return faults.Invalid("secret-store.vault.auth.password requires username and password", nil)
	}

	return s.loginUserPassWithCredentials(ctx, username, password, mount)
}

func (s *Store) loginUserPassWithCredentials(
	ctx context.Context,
	username string,
	password string,
	mount string,
) error {
	endpoint := buildEndpoint("auth", mount, "login", username)
	payload := map[string]string{"password": password}

	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if err := mapVaultStatus(status, response, false, ""); err != nil {
		return err
	}
	if response.Auth == nil || strings.TrimSpace(response.Auth.ClientToken) == "" {
		return faults.Auth("vault authentication response did not include a client token", nil)
	}

	s.token = strings.TrimSpace(response.Auth.ClientToken)
	return nil
}

func (s *Store) loginAppRole(ctx context.Context) error {
	credentials := s.auth.appRole
	if credentials == nil {
		return faults.Invalid("vault approle auth configuration is invalid", nil)
	}

	mount, err := normalizeVaultPath(credentials.Mount, true)
	if err != nil {
		return faults.Invalid("secret-store.vault.auth.approle.mount is invalid", err)
	}
	if mount == "" {
		mount = "approle"
	}

	roleID := strings.TrimSpace(credentials.RoleID)
	secretID := strings.TrimSpace(credentials.SecretID)
	if roleID == "" || secretID == "" {
		return faults.Invalid("secret-store.vault.auth.approle requires role-id and secret-id", nil)
	}

	endpoint := buildEndpoint("auth", mount, "login")
	payload := map[string]string{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if err := mapVaultStatus(status, response, false, ""); err != nil {
		return err
	}
	if response.Auth == nil || strings.TrimSpace(response.Auth.ClientToken) == "" {
		return faults.Auth("vault authentication response did not include a client token", nil)
	}

	s.token = strings.TrimSpace(response.Auth.ClientToken)
	return nil
}
