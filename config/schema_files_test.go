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
	if _, ok := contextProperties["managedServer"]; !ok {
		t.Fatal("expected context schema to define managedServer")
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
	if _, ok := resourceProperties["defaultFormat"]; !ok {
		t.Fatal("expected metadata resource schema to define defaultFormat")
	}
	if _, ok := resourceProperties["externalizedAttributes"]; !ok {
		t.Fatal("expected metadata resource schema to define externalizedAttributes")
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
