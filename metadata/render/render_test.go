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
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationGet): {
					Accept: "{{payload_media_type .}}",
				},
				string(metadatadomain.OperationCreate): {
					ContentType: "{{payload_media_type .}}",
				},
			},
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
				string(metadatadomain.OperationGet): {
					Accept: "{{payload_media_type .}}",
				},
				string(metadatadomain.OperationCreate): {
					ContentType: "{{payload_media_type .}}",
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
					Accept: "{{payload_media_type .}}",
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
