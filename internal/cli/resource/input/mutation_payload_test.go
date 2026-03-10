package input

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func TestDecodeOptionalMutationPayloadInputReadsOpaqueFilePayload(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	payloadFile := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(payloadFile, []byte("private-key-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{Payload: payloadFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.Extension != ".key" {
		t.Fatalf("expected .key extension, got %q", content.Descriptor.Extension)
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeOctetStream {
		t.Fatalf("expected octet-stream payload type, got %q", content.Descriptor.PayloadType)
	}
	binaryValue, ok := content.Value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue payload, got %T", content.Value)
	}
	if string(binaryValue.Bytes) != "private-key-bytes" {
		t.Fatalf("expected original file bytes, got %q", string(binaryValue.Bytes))
	}
}

func TestDecodeOptionalMutationPayloadInputRejectsMissingPathLikePayload(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	_, _, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload: "test/e2e/.runs/20260308-170415-3098387/private.key",
	})
	if err == nil {
		t.Fatal("expected missing payload file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestDecodeOptionalMutationPayloadInputAcceptsInlineJSONObject(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload: `{"id":"acme","name":"Acme"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}

	if !reflect.DeepEqual(content.Value, map[string]any{"id": "acme", "name": "Acme"}) {
		t.Fatalf("expected decoded inline object, got %#v", content.Value)
	}
}

func TestDecodeOptionalMutationPayloadInputAcceptsPointerAssignments(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload: "/id=acme,/name=Acme,/spec/tier=gold",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}

	expected := map[string]any{
		"id":   "acme",
		"name": "Acme",
		"spec": map[string]any{"tier": "gold"},
	}
	if !reflect.DeepEqual(content.Value, expected) {
		t.Fatalf("expected decoded pointer assignment object, got %#v", content.Value)
	}
}

func TestDecodeOptionalMutationPayloadInputAcceptsDotNotationAssignments(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload: "name=test,metadata.labels.env=prod",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}

	expected := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"labels": map[string]any{
				"env": "prod",
			},
		},
	}
	if !reflect.DeepEqual(content.Value, expected) {
		t.Fatalf("expected decoded dot-notation object, got %#v", content.Value)
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json payload type, got %q", content.Descriptor.PayloadType)
	}
}

func TestDecodeOptionalMutationPayloadInputDotNotationRespectsContentType(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload:     "name=test",
		ContentType: "yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml payload type, got %q", content.Descriptor.PayloadType)
	}
}

func TestDecodeOptionalMutationPayloadInputPointerAssignmentsRespectStructuredContentType(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload:     "/name=test",
		ContentType: "yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml payload type, got %q", content.Descriptor.PayloadType)
	}
}

func TestDecodeOptionalMutationPayloadInputExplicitTextContentTypeTreatsInlinePayloadAsLiteralText(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload:     "a=b",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeText {
		t.Fatalf("expected text payload type, got %q", content.Descriptor.PayloadType)
	}
	if value, ok := content.Value.(string); !ok || value != "a=b" {
		t.Fatalf("expected literal text payload %q, got %#v", "a=b", content.Value)
	}
}

func TestDecodeOptionalMutationPayloadInputTxtAliasTreatsInlinePayloadAsLiteralText(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload:     "a=b",
		ContentType: "txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeText {
		t.Fatalf("expected text payload type, got %q", content.Descriptor.PayloadType)
	}
	if content.Descriptor.Extension != ".txt" {
		t.Fatalf("expected .txt extension, got %q", content.Descriptor.Extension)
	}
	if value, ok := content.Value.(string); !ok || value != "a=b" {
		t.Fatalf("expected literal text payload %q, got %#v", "a=b", content.Value)
	}
}

func TestDecodeOptionalMutationPayloadInputExplicitTextMediaTypeTreatsAssignmentsAsLiteralText(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload:     "a=b",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeText {
		t.Fatalf("expected text payload type, got %q", content.Descriptor.PayloadType)
	}
	if value, ok := content.Value.(string); !ok || value != "a=b" {
		t.Fatalf("expected literal text payload %q, got %#v", "a=b", content.Value)
	}
}

func TestDecodeOptionalMutationPayloadInputDotNotationWithQuotedKeys(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}

	content, hasInput, err := DecodeOptionalMutationPayloadInput(command, cliutil.InputFlags{
		Payload: `testA."testB.testC"=bla`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasInput {
		t.Fatal("expected explicit payload input")
	}

	expected := map[string]any{
		"testA": map[string]any{
			"testB.testC": "bla",
		},
	}
	if !reflect.DeepEqual(content.Value, expected) {
		t.Fatalf("expected decoded dot-notation object with quoted key, got %#v", content.Value)
	}
}
