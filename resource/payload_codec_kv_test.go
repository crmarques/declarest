package resource

import (
	"reflect"
	"testing"
)

func TestDecodeEncodeINIPayload(t *testing.T) {
	t.Parallel()

	content, err := DecodeContent([]byte("owner = platform\n\n[service]\nenabled = true\n"), PayloadDescriptor{PayloadType: PayloadTypeINI})
	if err != nil {
		t.Fatalf("DecodeContent returned error: %v", err)
	}

	want := map[string]any{
		"owner": "platform",
		"service": map[string]any{
			"enabled": "true",
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected ini payload: got %#v want %#v", content.Value, want)
	}

	encoded, err := EncodeContentPretty(content)
	if err != nil {
		t.Fatalf("EncodeContentPretty returned error: %v", err)
	}
	if string(encoded) != "owner = platform\n\n[service]\nenabled = true\n" {
		t.Fatalf("unexpected ini encoding: %q", string(encoded))
	}
}

func TestDecodeEncodePropertiesPayload(t *testing.T) {
	t.Parallel()

	content, err := DecodeContent([]byte("owner=platform\nregion=us\\-east\n"), PayloadDescriptor{PayloadType: PayloadTypeProperties})
	if err != nil {
		t.Fatalf("DecodeContent returned error: %v", err)
	}

	want := map[string]any{
		"owner":  "platform",
		"region": "us-east",
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected properties payload: got %#v want %#v", content.Value, want)
	}

	encoded, err := EncodeContentPretty(content)
	if err != nil {
		t.Fatalf("EncodeContentPretty returned error: %v", err)
	}
	if string(encoded) != "owner=platform\nregion=us-east\n" {
		t.Fatalf("unexpected properties encoding: %q", string(encoded))
	}
}

func TestDecodeEncodePropertiesPayloadSupportsLineContinuation(t *testing.T) {
	t.Parallel()

	content, err := DecodeContent([]byte("description=hello\\\n  world\n"), PayloadDescriptor{PayloadType: PayloadTypeProperties})
	if err != nil {
		t.Fatalf("DecodeContent returned error: %v", err)
	}

	value, ok := content.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected object payload, got %T", content.Value)
	}
	if got := value["description"]; got != "helloworld" {
		t.Fatalf("unexpected properties continuation value: got %#v", got)
	}
}
