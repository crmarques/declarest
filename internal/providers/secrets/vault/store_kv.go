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
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type vaultResponse struct {
	Data   map[string]any `json:"data"`
	Errors []string       `json:"errors"`
	Auth   *vaultAuthInfo `json:"auth"`
}

func (s *Store) Store(ctx context.Context, key string, value string) error {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return err
	}

	payload := map[string]any{}
	if s.kvVersion == 2 {
		payload["data"] = map[string]string{"value": value}
	} else {
		payload["value"] = value
	}

	endpoint := s.writeEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	return mapVaultStatus(status, response, false, "")
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return "", err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return "", err
	}

	endpoint := s.readEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	if err := mapVaultStatus(status, response, false, "secret key not found"); err != nil {
		return "", err
	}

	value, err := s.extractValue(response)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return err
	}

	endpoint := s.deleteEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	// Delete is idempotent. Missing keys are treated as success.
	if status == http.StatusNotFound {
		return nil
	}

	return mapVaultStatus(status, response, false, "")
}

func (s *Store) List(ctx context.Context) ([]string, error) {
	if err := s.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	pendingPrefixes := []string{""}
	seenPrefixes := map[string]struct{}{"": {}}
	seenKeys := map[string]struct{}{}
	keys := []string{}

	for len(pendingPrefixes) > 0 {
		prefix := pendingPrefixes[len(pendingPrefixes)-1]
		pendingPrefixes = pendingPrefixes[:len(pendingPrefixes)-1]

		entries, err := s.listEntries(ctx, prefix)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry, "/") {
				childPrefix, err := normalizeVaultPath(joinVaultListPath(prefix, strings.TrimSuffix(entry, "/")), false)
				if err != nil {
					return nil, faults.Internal("vault list response payload is invalid", err)
				}
				if _, seen := seenPrefixes[childPrefix]; seen {
					continue
				}
				seenPrefixes[childPrefix] = struct{}{}
				pendingPrefixes = append(pendingPrefixes, childPrefix)
				continue
			}

			key, err := secretdomain.NormalizeKey(joinVaultListPath(prefix, entry))
			if err != nil {
				return nil, faults.Internal("vault list response payload is invalid", err)
			}
			if _, seen := seenKeys[key]; seen {
				continue
			}
			seenKeys[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)

	return keys, nil
}

func (s *Store) listEntries(ctx context.Context, key string) ([]string, error) {
	endpoint := s.listEndpoint(key)
	response, status, err := s.request(ctx, "LIST", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusBadRequest {
		fallbackEndpoint := endpoint + "?list=true"
		response, status, err = s.request(ctx, http.MethodGet, fallbackEndpoint, nil)
		if err != nil {
			return nil, err
		}
	}
	if err := mapVaultStatus(status, response, true, ""); err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return []string{}, nil
	}

	rawKeys, found := response.Data["keys"]
	if !found {
		return []string{}, nil
	}

	typedKeys, ok := rawKeys.([]any)
	if !ok {
		return nil, faults.Internal("vault list response payload is invalid", nil)
	}

	entries := make([]string, 0, len(typedKeys))
	for _, item := range typedKeys {
		entry, ok := item.(string)
		if !ok {
			return nil, faults.Internal("vault list response payload is invalid", nil)
		}
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.HasSuffix(entry, "/") {
			entry = strings.TrimSpace(strings.TrimSuffix(entry, "/"))
			if entry == "" {
				continue
			}
			entries = append(entries, entry+"/")
			continue
		}
		entry = strings.Trim(entry, "/")
		if entry == "" {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func joinVaultListPath(prefix string, entry string) string {
	trimmedPrefix := strings.Trim(prefix, "/")
	trimmedEntry := strings.Trim(entry, "/")
	switch {
	case trimmedPrefix == "":
		return trimmedEntry
	case trimmedEntry == "":
		return trimmedPrefix
	default:
		return trimmedPrefix + "/" + trimmedEntry
	}
}

func (s *Store) extractValue(response vaultResponse) (string, error) {
	if response.Data == nil {
		return "", faults.Internal("vault response missing secret payload", nil)
	}

	target := response.Data
	if s.kvVersion == 2 {
		rawData, found := response.Data["data"]
		if !found {
			return "", faults.Internal("vault response missing kv-v2 data payload", nil)
		}
		typedData, ok := rawData.(map[string]any)
		if !ok {
			return "", faults.Internal("vault response has invalid kv-v2 data payload", nil)
		}
		target = typedData
	}

	rawValue, found := target["value"]
	if !found {
		return "", faults.NotFound("secret key not found", nil)
	}

	value, ok := rawValue.(string)
	if !ok {
		return "", faults.Internal("vault secret value is not a string", nil)
	}
	return value, nil
}
