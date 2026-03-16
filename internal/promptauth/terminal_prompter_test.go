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
	"bytes"
	"testing"
)

func TestWritePromptWarningWithArgs(t *testing.T) {
	t.Parallel()

	t.Run("uses standard warning label", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		writePromptWarningWithArgs(
			&buf,
			nil,
			"credentials for managed-server proxy auth will be stored in declarest session environment variables and reused by later declarest commands in this terminal session.",
		)

		want := "[WARNING] credentials for managed-server proxy auth will be stored in declarest session environment variables and reused by later declarest commands in this terminal session.\n"
		if got := buf.String(); got != want {
			t.Fatalf("writePromptWarningWithArgs() = %q, want %q", got, want)
		}
	})

	t.Run("suppresses warning when ignore-warnings is set", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		writePromptWarningWithArgs(&buf, []string{"--ignore-warnings"}, "credentials for managed-server proxy auth will be stored")
		if got := buf.String(); got != "" {
			t.Fatalf("expected warning suppression, got %q", got)
		}
	})
}

func TestSessionReuseScope(t *testing.T) {
	t.Parallel()

	if got, want := sessionReuseScope(false), "this declarest command"; got != want {
		t.Fatalf("sessionReuseScope(false) = %q, want %q", got, want)
	}
	if got, want := sessionReuseScope(true), "later declarest commands in this terminal session"; got != want {
		t.Fatalf("sessionReuseScope(true) = %q, want %q", got, want)
	}
}
