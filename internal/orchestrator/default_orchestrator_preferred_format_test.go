package orchestrator

import (
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestApplyDefaultFormatSkipsExplicitDescriptor(t *testing.T) {
	t.Parallel()

	o := New(nil, nil, nil, nil, WithDefaultFormat("yaml"))
	content := resource.Content{
		Value:      map[string]any{"name": "test"},
		Descriptor: resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON},
	}
	md := metadatadomain.ResourceMetadata{}

	result := o.applyDefaultFormat(content, md)
	if result.Descriptor.PayloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json, got %q", result.Descriptor.PayloadType)
	}
}

func TestApplyDefaultFormatUsesOrchestratorDefault(t *testing.T) {
	t.Parallel()

	o := New(nil, nil, nil, nil, WithDefaultFormat("yaml"))
	content := resource.Content{
		Value:      map[string]any{"name": "test"},
		Descriptor: resource.PayloadDescriptor{},
	}
	md := metadatadomain.ResourceMetadata{}

	result := o.applyDefaultFormat(content, md)
	if result.Descriptor.PayloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml, got %q", result.Descriptor.PayloadType)
	}
}

func TestApplyDefaultFormatMetadataOverridesOrchestrator(t *testing.T) {
	t.Parallel()

	o := New(nil, nil, nil, nil, WithDefaultFormat("json"))
	content := resource.Content{
		Value:      map[string]any{"name": "test"},
		Descriptor: resource.PayloadDescriptor{},
	}
	md := metadatadomain.ResourceMetadata{DefaultFormat: "yaml"}

	result := o.applyDefaultFormat(content, md)
	if result.Descriptor.PayloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml (from metadata), got %q", result.Descriptor.PayloadType)
	}
}

func TestApplyDefaultFormatSkipsMixedItemDefault(t *testing.T) {
	t.Parallel()

	o := New(nil, nil, nil, nil, WithDefaultFormat("yaml"))
	content := resource.Content{
		Value:      map[string]any{"name": "test"},
		Descriptor: resource.PayloadDescriptor{},
	}
	md := metadatadomain.ResourceMetadata{DefaultFormat: metadatadomain.ResourceDefaultFormatAny}

	result := o.applyDefaultFormat(content, md)
	if result.Descriptor.PayloadType != "" {
		t.Fatalf("expected empty descriptor, got %q", result.Descriptor.PayloadType)
	}
}

func TestApplyDefaultFormatNoPreference(t *testing.T) {
	t.Parallel()

	o := New(nil, nil, nil, nil)
	content := resource.Content{
		Value:      map[string]any{"name": "test"},
		Descriptor: resource.PayloadDescriptor{},
	}
	md := metadatadomain.ResourceMetadata{}

	result := o.applyDefaultFormat(content, md)
	if result.Descriptor.PayloadType != "" {
		t.Fatalf("expected empty descriptor, got %q", result.Descriptor.PayloadType)
	}
}

func TestResolveDefaultFormatPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		orchestrator   string
		metadata       string
		expectedFormat string
	}{
		{"metadata wins over orchestrator", "json", "yaml", "yaml"},
		{"orchestrator used when no metadata", "json", "", "json"},
		{"empty when neither set", "", "", ""},
		{"metadata only", "", "yaml", "yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := New(nil, nil, nil, nil, WithDefaultFormat(tc.orchestrator))
			md := metadatadomain.ResourceMetadata{DefaultFormat: tc.metadata}

			result := o.resolveDefaultFormat(md)
			if result != tc.expectedFormat {
				t.Fatalf("expected %q, got %q", tc.expectedFormat, result)
			}
		})
	}
}
