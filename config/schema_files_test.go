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

package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMachineReadableSchemasRemainParseableAndWired(t *testing.T) {
	t.Parallel()

	contextSchema := loadSchemaDocument(t, "context.schema.json")
	contextProperties := objectProperty(t, contextSchema, "properties")
	if _, ok := contextProperties["name"]; !ok {
		t.Fatal("expected context schema to define name")
	}
	if _, ok := contextProperties["repository"]; !ok {
		t.Fatal("expected context schema to define repository")
	}
	if _, ok := contextProperties["managedService"]; !ok {
		t.Fatal("expected context schema to define managedService")
	}
	if !stringSliceContains(t, contextSchema, "required", "name") {
		t.Fatal("expected context schema to require name")
	}
	if _, ok := contextSchema["$defs"]; !ok {
		t.Fatal("expected context schema to define shared definitions")
	}
	contextDefs := objectProperty(t, contextSchema, "$defs")
	proxyDef, ok := contextDefs["proxy"].(map[string]any)
	if !ok {
		t.Fatalf("expected proxy definition, got %#v", contextDefs["proxy"])
	}
	if _, ok := proxyDef["allOf"]; ok {
		t.Fatalf("expected proxy schema to allow sparse overrides without allOf URL requirements, got %#v", proxyDef["allOf"])
	}

	contextsSchema := loadSchemaDocument(t, "contexts.schema.json")
	contextsProperties := objectProperty(t, contextsSchema, "properties")
	contextsValue, ok := contextsProperties["contexts"].(map[string]any)
	if !ok {
		t.Fatalf("expected contexts property object, got %#v", contextsProperties["contexts"])
	}
	items, ok := contextsValue["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected contexts.items object, got %#v", contextsValue["items"])
	}
	if ref, _ := items["$ref"].(string); ref != "./context.schema.json" {
		t.Fatalf("expected contexts.items.$ref to context schema, got %#v", items["$ref"])
	}
	if !stringSliceContains(t, contextsSchema, "required", "contexts") {
		t.Fatal("expected contexts schema to require contexts")
	}
	if !stringSliceContains(t, contextsSchema, "required", "currentContext") {
		t.Fatal("expected contexts schema to require currentContext")
	}
	if _, ok := contextsProperties["credentials"]; !ok {
		t.Fatal("expected contexts schema to define credentials")
	}
	contextsDefs := objectProperty(t, contextsSchema, "$defs")
	credentialDef, ok := contextsDefs["credential"].(map[string]any)
	if !ok {
		t.Fatalf("expected credential definition, got %#v", contextsDefs["credential"])
	}
	credentialProperties := objectProperty(t, credentialDef, "properties")
	if _, ok := credentialProperties["username"]; !ok {
		t.Fatal("expected credential schema to define username")
	}
	if _, ok := credentialProperties["password"]; !ok {
		t.Fatal("expected credential schema to define password")
	}
	credentialValueDef, ok := contextsDefs["credentialValue"].(map[string]any)
	if !ok {
		t.Fatalf("expected credentialValue definition, got %#v", contextsDefs["credentialValue"])
	}
	if len(arrayProperty(t, credentialValueDef, "oneOf")) != 2 {
		t.Fatalf("expected credentialValue.oneOf to contain literal and prompt variants, got %#v", credentialValueDef["oneOf"])
	}

	metadataSchema := loadSchemaDocument(t, "metadata.schema.json")
	metadataProperties := objectProperty(t, metadataSchema, "properties")
	if _, ok := metadataProperties["resource"]; !ok {
		t.Fatal("expected metadata schema to define resource")
	}
	if _, ok := metadataProperties["operations"]; !ok {
		t.Fatal("expected metadata schema to define operations")
	}
	if _, ok := metadataProperties["idAttribute"]; ok {
		t.Fatal("expected metadata schema to exclude flat legacy idAttribute")
	}
	metadataDefs := objectProperty(t, metadataSchema, "$defs")
	transformStep, ok := metadataDefs["transformStep"].(map[string]any)
	if !ok {
		t.Fatalf("expected transformStep definition, got %#v", metadataDefs["transformStep"])
	}
	if len(arrayProperty(t, transformStep, "oneOf")) != 3 {
		t.Fatalf("expected transformStep.oneOf to contain three variants, got %#v", transformStep["oneOf"])
	}
	resourceDef, ok := metadataDefs["resource"].(map[string]any)
	if !ok {
		t.Fatalf("expected resource definition, got %#v", metadataDefs["resource"])
	}
	resourceProperties := objectProperty(t, resourceDef, "properties")
	if _, ok := resourceProperties["id"]; !ok {
		t.Fatal("expected metadata resource schema to define id")
	}
	if _, ok := resourceProperties["alias"]; !ok {
		t.Fatal("expected metadata resource schema to define alias")
	}
	if _, ok := resourceProperties["format"]; !ok {
		t.Fatal("expected metadata resource schema to define format")
	}
	if _, ok := resourceProperties["externalizedAttributes"]; !ok {
		t.Fatal("expected metadata resource schema to define externalizedAttributes")
	}
	preferencesDef := objectProperty(t, contextDefs, "preferences")
	if _, ok := preferencesDef["properties"]; ok {
		t.Fatalf("expected context preferences schema to avoid documented format-specific properties, got %#v", preferencesDef["properties"])
	}
}

func loadSchemaDocument(t *testing.T, name string) map[string]any {
	t.Helper()

	path := filepath.Join("..", "schemas", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", name, err)
	}

	document := map[string]any{}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode schema %s: %v", name, err)
	}
	return document
}

func objectProperty(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()

	item, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %#v", key, value[key])
	}
	return item
}

func arrayProperty(t *testing.T, value map[string]any, key string) []any {
	t.Helper()

	item, ok := value[key].([]any)
	if !ok {
		t.Fatalf("expected %s to be an array, got %#v", key, value[key])
	}
	return item
}

func stringSliceContains(t *testing.T, value map[string]any, key string, expected string) bool {
	t.Helper()

	items, ok := value[key].([]any)
	if !ok {
		t.Fatalf("expected %s to be an array, got %#v", key, value[key])
	}
	for _, item := range items {
		if text, ok := item.(string); ok && text == expected {
			return true
		}
	}
	return false
}
