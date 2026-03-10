package validation

import (
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func TestEffectiveResourceRequiredAttributesIncludesAliasAttribute(t *testing.T) {
	t.Parallel()

	md := metadatadomain.ResourceMetadata{
		AliasAttribute:     "/clientId",
		RequiredAttributes: []string{"/realm"},
	}

	effective := EffectiveResourceRequiredAttributes(md)
	if len(effective) != 2 || effective[0] != "/realm" || effective[1] != "/clientId" {
		t.Fatalf("expected aliasAttribute to be appended to required attributes, got %#v", effective)
	}
}

func TestValidateResourceRequiredAttributesRequiresAliasAttribute(t *testing.T) {
	t.Parallel()

	err := ValidateResourceRequiredAttributes(
		map[string]any{"realm": "platform"},
		metadatadomain.ResourceMetadata{
			AliasAttribute:     "/clientId",
			RequiredAttributes: []string{"/realm"},
		},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "/clientId") {
		t.Fatalf("expected missing aliasAttribute in validation error, got %v", err)
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
