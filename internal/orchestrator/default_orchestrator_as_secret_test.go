package orchestrator

import (
	"bytes"
	"context"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestOrchestratorResolvePayloadForRemoteResolvesWholeResourceSecretPlaceholder(t *testing.T) {
	t.Parallel()

	descriptor := resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		Extension: ".key",
	})
	orchestrator := &Orchestrator{
		secrets: &fakeSecretProvider{
			values: map[string]string{
				"/projects/platform/secrets/private-key:.": "private-key-bytes",
			},
		},
	}

	resolved, err := orchestrator.resolvePayloadForRemote(
		context.Background(),
		"/projects/platform/secrets/private-key",
		resource.Content{
			Value:      resource.BinaryValue{Bytes: []byte("{{secret .}}")},
			Descriptor: descriptor,
		},
	)
	if err != nil {
		t.Fatalf("resolvePayloadForRemote returned error: %v", err)
	}

	binaryValue, ok := resolved.Value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue payload, got %T", resolved.Value)
	}
	if !bytes.Equal(binaryValue.Bytes, []byte("private-key-bytes")) {
		t.Fatalf("expected decoded binary payload, got %q", string(binaryValue.Bytes))
	}
}
