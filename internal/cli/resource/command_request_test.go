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
