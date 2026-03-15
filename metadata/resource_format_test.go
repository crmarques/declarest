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

package metadata

import (
	"context"
	"testing"
)

func TestResolveOperationSpecWithScopeResolvesPayloadTemplateHelpers(t *testing.T) {
	t.Parallel()

	spec, err := ResolveOperationSpecWithScope(
		context.Background(),
		ResourceMetadata{
			RemoteCollectionPath: "/api/{{payload_type .}}/customers",
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Path:        "/api/customers/{{/id}}",
					Accept:      "{{payload_media_type .}}",
					ContentType: "application/{{payload_type .}}",
					Query: map[string]string{
						"format":    "{{payload_type .}}",
						"extension": "{{payload_extension .}}",
					},
				},
			},
		},
		OperationGet,
		map[string]any{
			"id":               "acme",
			"payloadType":      "yaml",
			"payloadMediaType": "application/yaml",
			"payloadExtension": ".yaml",
		},
	)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
	}

	if spec.Path != "/api/customers/acme" {
		t.Fatalf("expected rendered path, got %q", spec.Path)
	}
	if spec.Accept != "application/yaml" {
		t.Fatalf("expected accept to resolve, got %q", spec.Accept)
	}
	if spec.ContentType != "application/yaml" {
		t.Fatalf("expected contentType to resolve, got %q", spec.ContentType)
	}
	if spec.Query["format"] != "yaml" {
		t.Fatalf("expected query.format to resolve, got %q", spec.Query["format"])
	}
	if spec.Query["extension"] != ".yaml" {
		t.Fatalf("expected query.extension to resolve, got %q", spec.Query["extension"])
	}
}
