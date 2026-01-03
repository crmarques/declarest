package cmd

import (
	"testing"

	"declarest/internal/resource"
)

func TestFindUnmappedSecretPathsCollectionIgnoresItemPrefix(t *testing.T) {
	res, err := resource.NewResource([]any{
		map[string]any{
			"config": map[string]any{
				"bindCredential": []any{"value"},
			},
		},
	})
	if err != nil {
		t.Fatalf("new resource: %v", err)
	}

	mapped := []string{"config.bindCredential[0]"}
	unmapped := findUnmappedSecretPaths(res, mapped, true)
	if len(unmapped) != 0 {
		t.Fatalf("expected no unmapped secrets, got %v", unmapped)
	}
}
