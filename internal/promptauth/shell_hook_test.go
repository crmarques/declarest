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

package promptauth

import (
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestRenderSessionHook(t *testing.T) {
	t.Parallel()

	t.Run("bash", func(t *testing.T) {
		t.Parallel()

		hook, err := RenderSessionHook("bash")
		if err != nil {
			t.Fatalf("RenderSessionHook(bash) returned error: %v", err)
		}
		required := []string{
			`export DECLAREST_PROMPT_AUTH_SESSION_ID="bash:${BASHPID:-$$}"`,
			"command declarest context clean --credentials-in-session >/dev/null 2>&1 || true",
			`trap 'declarest_prompt_auth_on_exit' EXIT`,
		}
		for _, snippet := range required {
			if !strings.Contains(hook, snippet) {
				t.Fatalf("expected bash hook to contain %q, got %q", snippet, hook)
			}
		}
	})

	t.Run("zsh", func(t *testing.T) {
		t.Parallel()

		hook, err := RenderSessionHook("zsh")
		if err != nil {
			t.Fatalf("RenderSessionHook(zsh) returned error: %v", err)
		}
		required := []string{
			`export DECLAREST_PROMPT_AUTH_SESSION_ID="zsh:$$"`,
			"command declarest context clean --credentials-in-session >/dev/null 2>&1 || true",
			"typeset -ga zshexit_functions",
			"zshexit_functions+=(declarest_prompt_auth_cleanup)",
		}
		for _, snippet := range required {
			if !strings.Contains(hook, snippet) {
				t.Fatalf("expected zsh hook to contain %q, got %q", snippet, hook)
			}
		}
	})

	t.Run("invalid shell", func(t *testing.T) {
		t.Parallel()

		_, err := RenderSessionHook("fish")
		if err == nil {
			t.Fatal("expected invalid shell validation error")
		}
		if !faults.IsCategory(err, faults.ValidationError) {
			t.Fatalf("expected validation error category, got %v", err)
		}
	})
}
