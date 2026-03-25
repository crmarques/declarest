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

package webhookreceiver

import (
	"path"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

// RefToBranch extracts the branch name from a git ref like "refs/heads/main".
func RefToBranch(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	return ref
}

// MatchBranchFilter checks if a branch matches the include/exclude filter.
// If no filter is specified, all branches match.
func MatchBranchFilter(branch string, filter *declarestv1alpha1.RepositoryWebhookBranchFilter) bool {
	if filter == nil {
		return true
	}

	// Check exclude first.
	for _, pattern := range filter.Exclude {
		if matchGlob(branch, pattern) {
			return false
		}
	}

	// If include is empty, all non-excluded branches match.
	if len(filter.Include) == 0 {
		return true
	}

	for _, pattern := range filter.Include {
		if matchGlob(branch, pattern) {
			return true
		}
	}
	return false
}

// matchGlob matches a branch name against a glob pattern (supports * wildcard).
func matchGlob(name, pattern string) bool {
	matched, _ := path.Match(pattern, name)
	return matched
}
