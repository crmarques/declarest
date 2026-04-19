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
	"bytes"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func TestEncodeWholeResourceSecretRoundTripsStructuredPayload(t *testing.T) {
	t.Parallel()

	content := resource.Content{
		Value: map[string]any{
			"id":   "acme",
			"tier": "pro",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			PayloadType: resource.PayloadTypeJSON,
		}),
	}

	encoded, err := EncodeWholeResourceSecret(content)
	if err != nil {
		t.Fatalf("EncodeWholeResourceSecret returned error: %v", err)
	}
	if strings.HasPrefix(encoded, wholeResourceSecretEncodingPrefix) {
		t.Fatalf("expected UTF-8 structured payload to be stored directly, got %q", encoded)
	}

	decoded, err := DecodeWholeResourceSecret(encoded, content.Descriptor)
	if err != nil {
		t.Fatalf("DecodeWholeResourceSecret returned error: %v", err)
	}
	expected := map[string]any{
		"id":   "acme",
		"tier": "pro",
	}
	if got, ok := decoded.(map[string]any); !ok || got["id"] != expected["id"] || got["tier"] != expected["tier"] {
		t.Fatalf("expected decoded structured payload %#v, got %#v", expected, decoded)
	}
}

func TestEncodeWholeResourceSecretBase64EncodesBinaryPayload(t *testing.T) {
	t.Parallel()

	content := resource.Content{
		Value: resource.BinaryValue{Bytes: []byte{0xff, 0x00, 0x10, 0x80}},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			Extension: ".bin",
		}),
	}

	encoded, err := EncodeWholeResourceSecret(content)
	if err != nil {
		t.Fatalf("EncodeWholeResourceSecret returned error: %v", err)
	}
	if !strings.HasPrefix(encoded, wholeResourceSecretEncodingPrefix) {
		t.Fatalf("expected binary payload to be base64-encoded, got %q", encoded)
	}

	decoded, err := DecodeWholeResourceSecret(encoded, content.Descriptor)
	if err != nil {
		t.Fatalf("DecodeWholeResourceSecret returned error: %v", err)
	}
	binaryValue, ok := decoded.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue, got %T", decoded)
	}
	if !bytes.Equal(binaryValue.Bytes, []byte{0xff, 0x00, 0x10, 0x80}) {
		t.Fatalf("expected decoded binary payload to match input, got %v", binaryValue.Bytes)
	}
}

func TestResolveWholeResourcePlaceholderForResourceStructuredPayload(t *testing.T) {
	t.Parallel()

	descriptor := resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		PayloadType: resource.PayloadTypeJSON,
	})
	secretValue, err := EncodeWholeResourceSecret(resource.Content{
		Value: map[string]any{
			"name": "ACME",
		},
		Descriptor: descriptor,
	})
	if err != nil {
		t.Fatalf("EncodeWholeResourceSecret returned error: %v", err)
	}

	resolved, handled, err := ResolveWholeResourcePlaceholderForResource(
		"{{secret .}}",
		"/customers/acme",
		descriptor,
		func(key string) (string, error) {
			if key != "/customers/acme:." {
				return "", faults.NotFound("missing", nil)
			}
			return secretValue, nil
		},
	)
	if err != nil {
		t.Fatalf("ResolveWholeResourcePlaceholderForResource returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected whole-resource placeholder to be handled")
	}

	payload, ok := resolved.(map[string]any)
	if !ok {
		t.Fatalf("expected structured payload, got %T", resolved)
	}
	if payload["name"] != "ACME" {
		t.Fatalf("expected decoded whole-resource payload, got %#v", payload)
	}
}

func TestResolveWholeResourcePlaceholderForResourceBinaryPayload(t *testing.T) {
	t.Parallel()

	descriptor := resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		Extension: ".key",
	})
	secretValue, err := EncodeWholeResourceSecret(resource.Content{
		Value:      resource.BinaryValue{Bytes: []byte("private-key-bytes")},
		Descriptor: descriptor,
	})
	if err != nil {
		t.Fatalf("EncodeWholeResourceSecret returned error: %v", err)
	}

	resolved, handled, err := ResolveWholeResourcePlaceholderForResource(
		resource.BinaryValue{Bytes: []byte("{{secret .}}")},
		"/projects/platform/secrets/private-key",
		descriptor,
		func(key string) (string, error) {
			if key != "/projects/platform/secrets/private-key:." {
				return "", faults.NotFound("missing", nil)
			}
			return secretValue, nil
		},
	)
	if err != nil {
		t.Fatalf("ResolveWholeResourcePlaceholderForResource returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected binary whole-resource placeholder to be handled")
	}

	binaryValue, ok := resolved.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue, got %T", resolved)
	}
	if !bytes.Equal(binaryValue.Bytes, []byte("private-key-bytes")) {
		t.Fatalf("expected decoded binary payload, got %q", string(binaryValue.Bytes))
	}
}
