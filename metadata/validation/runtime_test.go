package validation

import (
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func TestEffectiveResourceRequiredAttributesIncludesIdentityTemplatePointers(t *testing.T) {
	t.Parallel()

	md := metadatadomain.ResourceMetadata{
		Alias:              "{{/clientId}}",
		ID:                 "{{default /metadata/externalId /id}}",
		RequiredAttributes: []string{"/realm"},
	}

	effective := EffectiveResourceRequiredAttributes(md)
	want := []string{"/realm", "/clientId", "/metadata/externalId", "/id"}
	if len(effective) != len(want) {
		t.Fatalf("unexpected required attributes %#v", effective)
	}
	for idx := range want {
		if effective[idx] != want[idx] {
			t.Fatalf("unexpected required attributes %#v", effective)
		}
	}
}

func TestValidateResourceRequiredAttributesRequiresIdentityTemplatePointers(t *testing.T) {
	t.Parallel()

	err := ValidateResourceRequiredAttributes(
		map[string]any{"realm": "platform"},
		metadatadomain.ResourceMetadata{
			Alias:              "{{/clientId}}",
			ID:                 "{{/id}}",
			RequiredAttributes: []string{"/realm"},
		},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "/clientId") || !strings.Contains(err.Error(), "/id") {
		t.Fatalf("expected missing identity pointers in validation error, got %v", err)
	}
}

func TestEffectiveResourceRequiredAttributesForCreateExcludesIDTemplatePointers(t *testing.T) {
	t.Parallel()

	md := metadatadomain.ResourceMetadata{
		Alias:              "{{/clientId}}",
		ID:                 "{{/id}}",
		RequiredAttributes: []string{"/realm"},
	}

	effective := EffectiveResourceRequiredAttributesForOperation(md, metadatadomain.OperationCreate)
	want := []string{"/realm", "/clientId"}
	if len(effective) != len(want) {
		t.Fatalf("unexpected create required attributes %#v", effective)
	}
	for idx := range want {
		if effective[idx] != want[idx] {
			t.Fatalf("unexpected create required attributes %#v", effective)
		}
	}
}

func TestValidateResourceRequiredAttributesForCreateSkipsImplicitIDPointers(t *testing.T) {
	t.Parallel()

	err := ValidateResourceRequiredAttributesForOperation(
		map[string]any{"realm": "platform", "clientId": "declarest-cli"},
		metadatadomain.ResourceMetadata{
			Alias:              "{{/clientId}}",
			ID:                 "{{/id}}",
			RequiredAttributes: []string{"/realm"},
		},
		metadatadomain.OperationCreate,
	)
	if err != nil {
		t.Fatalf("expected create validation to accept server-assigned ids, got %v", err)
	}

	err = ValidateResourceRequiredAttributesForOperation(
		map[string]any{"realm": "platform"},
		metadatadomain.ResourceMetadata{
			Alias:              "{{/clientId}}",
			ID:                 "{{/id}}",
			RequiredAttributes: []string{"/realm"},
		},
		metadatadomain.OperationCreate,
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "/clientId") || strings.Contains(err.Error(), "/id") {
		t.Fatalf("expected create validation error to require alias but not id, got %v", err)
	}
}

func TestValidateRequiredAttributesDeduplicatesPointers(t *testing.T) {
	t.Parallel()

	err := ValidateRequiredAttributes(
		map[string]any{"realm": "platform"},
		"resource.requiredAttributes",
		[]string{"/realm", "/realm"},
		"resource payload validation",
	)
	if err != nil {
		t.Fatalf("expected duplicate required attributes to be ignored, got %v", err)
	}
}
