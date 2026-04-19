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
	"net/url"
	"strings"

	"github.com/crmarques/declarest/faults"
)

func (s *Store) readEndpoint(key string) string {
	if s.kvVersion == 2 {
		return buildEndpoint(s.mount, "data", s.fullSecretPath(key))
	}
	return buildEndpoint(s.mount, s.fullSecretPath(key))
}

func (s *Store) writeEndpoint(key string) string {
	return s.readEndpoint(key)
}

func (s *Store) deleteEndpoint(key string) string {
	return s.readEndpoint(key)
}

func (s *Store) listEndpoint(key string) string {
	if s.kvVersion == 2 {
		return buildEndpoint(s.mount, "metadata", s.fullSecretPath(key))
	}
	return buildEndpoint(s.mount, s.fullSecretPath(key))
}

func (s *Store) fullSecretPath(key string) string {
	normalized := strings.TrimSpace(key)
	if s.pathPrefix == "" {
		return normalized
	}
	if normalized == "" {
		return s.pathPrefix
	}
	return s.pathPrefix + "/" + normalized
}

func normalizeVaultAddress(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", faults.NewValidationError("secret-store.vault.address is required", nil)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", faults.NewValidationError("secret-store.vault.address is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", faults.NewValidationError("secret-store.vault.address must use http or https", nil)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", faults.NewValidationError("secret-store.vault.address host is required", nil)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeVaultPath(value string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		if allowEmpty {
			return "", nil
		}
		return "", faults.NewValidationError("vault path must not be empty", nil)
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", faults.NewValidationError("vault path contains invalid segments", nil)
		}
	}

	return strings.Join(parts, "/"), nil
}

func buildEndpoint(parts ...string) string {
	encoded := make([]string, 0, len(parts)+1)
	encoded = append(encoded, "v1")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		for _, segment := range strings.Split(part, "/") {
			if segment == "" {
				continue
			}
			encoded = append(encoded, url.PathEscape(segment))
		}
	}

	return "/" + strings.Join(encoded, "/")
}
