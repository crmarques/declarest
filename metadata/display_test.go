package metadata

import (
	"reflect"
	"testing"
)

func TestDisplayResourceMetadataViewExpandsUnsetFields(t *testing.T) {
	t.Parallel()

	view := DisplayResourceMetadataView(ResourceMetadata{
		ID:    "{{/realm}}",
		Alias: "{{/realm}}",
	})

	if view.Resource.ID != "{{/realm}}" {
		t.Fatalf("expected id override, got %q", view.Resource.ID)
	}
	if view.Selector.Descendants {
		t.Fatalf("expected default selector.descendants=false, got %#v", view.Selector)
	}
	if view.Resource.Alias != "{{/realm}}" {
		t.Fatalf("expected alias override, got %q", view.Resource.Alias)
	}
	if len(view.Resource.RequiredAttributes) != 0 {
		t.Fatalf("expected empty requiredAttributes, got %#v", view.Resource.RequiredAttributes)
	}
	if view.Resource.RemoteCollectionPath != "" {
		t.Fatalf("expected empty remoteCollectionPath, got %q", view.Resource.RemoteCollectionPath)
	}
	if view.Resource.Format != "" {
		t.Fatalf("expected empty format, got %q", view.Resource.Format)
	}
	if view.Resource.Secret {
		t.Fatalf("expected default secret=false, got %#v", view.Resource.Secret)
	}
	if len(view.Resource.SecretAttributes) != 0 {
		t.Fatalf("expected empty secretAttributes, got %#v", view.Resource.SecretAttributes)
	}
	if len(view.Resource.ExternalizedAttributes) != 0 {
		t.Fatalf("expected empty externalizedAttributes, got %#v", view.Resource.ExternalizedAttributes)
	}

	if len(view.Operations.Defaults.Transforms) != 0 {
		t.Fatalf("expected empty defaults transforms, got %#v", view.Operations.Defaults.Transforms)
	}
	if view.Operations.Get.Method != "GET" {
		t.Fatalf("expected default get method GET, got %q", view.Operations.Get.Method)
	}
	if view.Operations.Get.Path != "./{{/id}}" {
		t.Fatalf("expected default get path, got %q", view.Operations.Get.Path)
	}
	if len(view.Operations.Get.Query) != 0 {
		t.Fatalf("expected empty get query, got %#v", view.Operations.Get.Query)
	}
	if len(view.Operations.Get.Headers) != 0 {
		t.Fatalf("expected empty default get headers, got %#v", view.Operations.Get.Headers)
	}
	if view.Operations.Get.Body != nil {
		t.Fatalf("expected nil default body, got %#v", view.Operations.Get.Body)
	}
	if len(view.Operations.Get.Transforms) != 0 {
		t.Fatalf("expected empty get transforms, got %#v", view.Operations.Get.Transforms)
	}
	if len(view.Operations.Get.Validate.RequiredAttributes) != 0 {
		t.Fatalf("expected empty validate.requiredAttributes, got %#v", view.Operations.Get.Validate.RequiredAttributes)
	}
	if len(view.Operations.Get.Validate.Assertions) != 0 {
		t.Fatalf("expected empty validate.assertions, got %#v", view.Operations.Get.Validate.Assertions)
	}
	if view.Operations.Get.Validate.SchemaRef != "" {
		t.Fatalf("expected empty validate.schemaRef, got %q", view.Operations.Get.Validate.SchemaRef)
	}
	if len(view.Operations.Create.Headers) != 0 {
		t.Fatalf("expected empty default create headers, got %#v", view.Operations.Create.Headers)
	}
}

func TestDisplayResourceMetadataViewPreservesConfiguredValues(t *testing.T) {
	t.Parallel()

	view := DisplayResourceMetadataView(ResourceMetadata{
		Selector: &SelectorSpec{Descendants: boolPointer(true)},
		Format:   "yaml",
		Secret:   boolPointer(true),
		RequiredAttributes: []string{
			"/realm",
		},
		SecretAttributes: []string{
			"/credentials/password",
		},
		ExternalizedAttributes: []ExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       "{{include .}}",
				Mode:           "string",
				SaveBehavior:   "inline",
				RenderBehavior: "expand",
				Enabled:        boolPointer(true),
			},
		},
		Operations: map[string]OperationSpec{
			string(OperationUpdate): {
				Method: "PATCH",
				Path:   "/api/customers/{{/id}}",
				Query: map[string]string{
					"refresh": "true",
				},
				Validate: &OperationValidationSpec{
					RequiredAttributes: []string{"/realm"},
					Assertions: []ValidationAssertion{
						{Message: "must be enabled", JQ: ".enabled == true"},
					},
					SchemaRef: "openapi:request-body",
				},
			},
		},
	})

	if view.Resource.Format != "yaml" {
		t.Fatalf("expected format yaml, got %q", view.Resource.Format)
	}
	if !view.Selector.Descendants {
		t.Fatalf("expected selector.descendants=true, got %#v", view.Selector)
	}
	if !view.Resource.Secret {
		t.Fatalf("expected secret=true, got %#v", view.Resource.Secret)
	}
	if len(view.Resource.RequiredAttributes) != 1 || view.Resource.RequiredAttributes[0] != "/realm" {
		t.Fatalf("expected requiredAttributes override, got %#v", view.Resource.RequiredAttributes)
	}
	if len(view.Resource.SecretAttributes) != 1 || view.Resource.SecretAttributes[0] != "/credentials/password" {
		t.Fatalf("expected secretAttributes override, got %#v", view.Resource.SecretAttributes)
	}
	if len(view.Resource.ExternalizedAttributes) != 1 {
		t.Fatalf("expected externalized attribute, got %#v", view.Resource.ExternalizedAttributes)
	}
	if view.Operations.Update.Method != "PATCH" {
		t.Fatalf("expected update method PATCH, got %q", view.Operations.Update.Method)
	}
	if view.Operations.Update.Query["refresh"] != "true" {
		t.Fatalf("expected update query override, got %#v", view.Operations.Update.Query)
	}
	if len(view.Operations.Update.Validate.RequiredAttributes) != 1 ||
		view.Operations.Update.Validate.RequiredAttributes[0] != "/realm" {
		t.Fatalf("expected validate.requiredAttributes override, got %#v", view.Operations.Update.Validate.RequiredAttributes)
	}
	if len(view.Operations.Update.Validate.Assertions) != 1 ||
		view.Operations.Update.Validate.Assertions[0].JQ != ".enabled == true" {
		t.Fatalf("expected validate.assertions override, got %#v", view.Operations.Update.Validate.Assertions)
	}
	if view.Operations.Update.Validate.SchemaRef != "openapi:request-body" {
		t.Fatalf("expected validate.schemaRef override, got %q", view.Operations.Update.Validate.SchemaRef)
	}
}

func TestDisplayTypesMatchCanonicalFieldCount(t *testing.T) {
	t.Parallel()

	// ResourceMetadata has Selector, Operations, and Transforms outside the
	// displayResourceWire section, so displayResourceWire should have
	// NumField(ResourceMetadata) - 3 fields.
	resourceFields := reflect.TypeOf(ResourceMetadata{}).NumField()
	displayResourceFields := reflect.TypeOf(displayResourceWire{}).NumField()
	if displayResourceFields != resourceFields-3 {
		t.Fatalf("displayResourceWire has %d fields but ResourceMetadata has %d (expected %d display fields); update display types",
			displayResourceFields, resourceFields, resourceFields-3)
	}

	// TransformStep ↔ displayTransformStepWire should match exactly.
	transformFields := reflect.TypeOf(TransformStep{}).NumField()
	displayTransformFields := reflect.TypeOf(displayTransformStepWire{}).NumField()
	if displayTransformFields != transformFields {
		t.Fatalf("displayTransformStepWire has %d fields but TransformStep has %d; update display types",
			displayTransformFields, transformFields)
	}

	// OperationValidationSpec ↔ displayOperationValidationWire should match exactly.
	validationFields := reflect.TypeOf(OperationValidationSpec{}).NumField()
	displayValidationFields := reflect.TypeOf(displayOperationValidationWire{}).NumField()
	if displayValidationFields != validationFields {
		t.Fatalf("displayOperationValidationWire has %d fields but OperationValidationSpec has %d; update display types",
			displayValidationFields, validationFields)
	}

	// ExternalizedAttribute ↔ displayExternalizedAttributeWire should match exactly.
	extAttrFields := reflect.TypeOf(ExternalizedAttribute{}).NumField()
	displayExtAttrFields := reflect.TypeOf(displayExternalizedAttributeWire{}).NumField()
	if displayExtAttrFields != extAttrFields {
		t.Fatalf("displayExternalizedAttributeWire has %d fields but ExternalizedAttribute has %d; update display types",
			displayExtAttrFields, extAttrFields)
	}

	// OperationSpec has Accept and ContentType which are promoted into Headers
	// on the wire, so displayOperationWire should have
	// NumField(OperationSpec) - 2 (Accept, ContentType) fields.
	opSpecFields := reflect.TypeOf(OperationSpec{}).NumField()
	displayOpFields := reflect.TypeOf(displayOperationWire{}).NumField()
	if displayOpFields != opSpecFields-2 {
		t.Fatalf("displayOperationWire has %d fields but OperationSpec has %d (expected %d display fields); update display types",
			displayOpFields, opSpecFields, opSpecFields-2)
	}
}
