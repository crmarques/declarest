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

package resource

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDecodeOptionalRequestPayloadPrefersFilePayloadOverInheritedStdin(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	command.SetIn(strings.NewReader("/api/projects/acme\n"))

	payloadFile := filepath.Join(t.TempDir(), "resource.json")
	if err := os.WriteFile(payloadFile, []byte(`{"id":"acme"}`), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	content, hasBody, err := decodeOptionalRequestPayload(command, "json", []string{payloadFile}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasBody {
		t.Fatal("expected request body")
	}
	expected := map[string]any{"id": "acme"}
	if !reflect.DeepEqual(content.Value, expected) {
		t.Fatalf("expected decoded file payload, got %#v", content.Value)
	}
}
