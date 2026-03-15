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

package completion

import (
	"strings"
	"testing"
)

func TestNormalizeZshCompletionPreservesCompletionItemsWithSpaces(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`completions+=${comp}`,
		"",
	}, "\n")

	normalized := string(normalizeZshCompletion([]byte(raw)))
	if !strings.Contains(normalized, `completions+=("${comp}")`) {
		t.Fatalf("expected zsh completion append to preserve spaced tokens, got %q", normalized)
	}
}

func TestNormalizeZshCompletionQuotesEvalRequest(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`out=$(eval ${requestComp} 2>/dev/null)`,
		"",
	}, "\n")

	normalized := string(normalizeZshCompletion([]byte(raw)))
	if !strings.Contains(normalized, `out=$(eval "${requestComp}" 2>/dev/null)`) {
		t.Fatalf("expected zsh completion request to be eval-quoted, got %q", normalized)
	}
}
