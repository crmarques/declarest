package render

import (
	"context"
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestRenderResourceMetadataWithFormatRendersResourceFormatTemplate(t *testing.T) {
	t.Parallel()

	md := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		metadatadomain.ResourceMetadata{
			ID:    "{{/id}}",
			Alias: "{{/id}}",
		},
	)

	rendered, err := RenderResourceMetadataWithFormat(
		context.Background(),
		"/customers/acme",
		md,
		map[string]any{"id": "acme"},
		"yaml",
	)
	if err != nil {
		t.Fatalf("RenderResourceMetadataWithFormat returned error: %v", err)
	}

	getOp := rendered.Operations[string(metadatadomain.OperationGet)]
	if getOp.Accept != "application/yaml" {
		t.Fatalf("expected get accept application/yaml, got %q", getOp.Accept)
	}

	createOp := rendered.Operations[string(metadatadomain.OperationCreate)]
	if createOp.ContentType != "application/yaml" {
		t.Fatalf("expected create contentType application/yaml, got %q", createOp.ContentType)
	}
}

func TestRenderResourceMetadataInfersTextPayloadDescriptorForTemplates(t *testing.T) {
	t.Parallel()

	md := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationCreate): {
					Headers: map[string]string{
						"X-Content-Type": "{{index . \"contentType\"}}",
					},
				},
			},
		},
	)

	rendered, err := RenderResourceMetadata(
		context.Background(),
		"/notes/readme",
		md,
		"hello",
	)
	if err != nil {
		t.Fatalf("RenderResourceMetadata returned error: %v", err)
	}

	getOp := rendered.Operations[string(metadatadomain.OperationGet)]
	if getOp.Accept != "text/plain" {
		t.Fatalf("expected get accept text/plain, got %q", getOp.Accept)
	}

	createOp := rendered.Operations[string(metadatadomain.OperationCreate)]
	if createOp.ContentType != "text/plain" {
		t.Fatalf("expected create contentType text/plain, got %q", createOp.ContentType)
	}
	if createOp.Headers["X-Content-Type"] != "text/plain" {
		t.Fatalf("expected injected contentType text/plain, got %#v", createOp.Headers)
	}
}

func TestRenderResourceMetadataWithDescriptorPreservesExplicitPayloadExtension(t *testing.T) {
	t.Parallel()

	md := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationGet): {
					Query: map[string]string{
						"extension": "{{payload_extension .}}",
					},
				},
			},
		},
	)

	rendered, err := RenderResourceMetadataWithDescriptor(
		context.Background(),
		"/keys/private-key",
		md,
		resource.BinaryValue{Bytes: []byte("secret")},
		resource.PayloadDescriptor{Extension: ".key"},
	)
	if err != nil {
		t.Fatalf("RenderResourceMetadataWithDescriptor returned error: %v", err)
	}

	getOp := rendered.Operations[string(metadatadomain.OperationGet)]
	if getOp.Accept != "application/octet-stream" {
		t.Fatalf("expected get accept application/octet-stream, got %q", getOp.Accept)
	}
	if getOp.Query["extension"] != ".key" {
		t.Fatalf("expected payload_extension .key, got %q", getOp.Query["extension"])
	}
}
