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

package secret

import (
	"context"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func listSecretKeys(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	request secretListRequest,
) ([]string, error) {
	keys, err := secretProvider.List(ctx)
	if err != nil {
		return nil, err
	}

	if !request.HasPath {
		return renderAllSecretKeys(keys), nil
	}

	normalizedPath := normalizeSecretStoreLookupKey(request.Path)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		normalizedKey := normalizeSecretStoreLookupKey(key)
		pathPart, keyPart, composite := splitStoredSecretPathKey(normalizedKey)
		if !composite {
			continue
		}

		displayKey, matches := renderPathScopedSecretListKey(normalizedPath, pathPart, keyPart, request.Recursive || request.Path == "/")
		if !matches {
			continue
		}
		items = append(items, displayKey)
	}

	sort.Strings(items)
	if len(items) == 0 {
		return nil, faults.NotFound("secret path not found", nil)
	}
	return items, nil
}

func renderPathScopedSecretListKey(
	requestPath string,
	pathPart string,
	keyPart string,
	recursive bool,
) (string, bool) {
	normalizedRequestPath := strings.Trim(requestPath, "/")
	normalizedPathPart := strings.Trim(pathPart, "/")
	if normalizedPathPart == "" || strings.TrimSpace(keyPart) == "" {
		return "", false
	}

	if normalizedRequestPath == "" {
		return "/" + normalizedPathPart + ":" + keyPart, true
	}

	if normalizedPathPart == normalizedRequestPath {
		return keyPart, true
	}

	if !recursive {
		return "", false
	}

	prefix := normalizedRequestPath + "/"
	if !strings.HasPrefix(normalizedPathPart, prefix) {
		return "", false
	}

	relativePath := strings.TrimPrefix(normalizedPathPart, prefix)
	if relativePath == "" {
		return "", false
	}

	return "/" + relativePath + ":" + keyPart, true
}

func renderAllSecretKeys(keys []string) []string {
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		displayKey := displaySecretKey(key)
		if displayKey == "" {
			continue
		}
		items = append(items, displayKey)
	}
	sort.Strings(items)
	return items
}

func displaySecretKey(rawKey string) string {
	normalizedKey := normalizeSecretStoreLookupKey(rawKey)
	if normalizedKey == "" {
		return ""
	}

	pathPart, keyPart, composite := splitStoredSecretPathKey(normalizedKey)
	if !composite {
		return normalizedKey
	}
	return "/" + pathPart + ":" + keyPart
}
