package render

import (
	"context"
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
)

func TestRenderResourceMetadataWithFormatRendersResourceFormatTemplate(t *testing.T) {
	t.Parallel()

	md := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		metadatadomain.ResourceMetadata{
			IDFromAttribute:    "id",
			AliasFromAttribute: "id",
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
