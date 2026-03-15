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

package secrets

import (
	"strings"

	"github.com/crmarques/declarest/faults"
)

func NormalizeKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "", faults.NewValidationError("secret key must not be empty", nil)
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", faults.NewValidationError("secret key contains invalid path segment", nil)
		}
	}

	return strings.Join(parts, "/"), nil
}
