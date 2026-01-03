package resource

import "testing"

func TestValidateLogicalPath(t *testing.T) {
	valid := []string{
		"/",
		"/items",
		"/items/",
		"/alpha/beta",
		"/alpha/beta/",
	}
	for _, path := range valid {
		if err := ValidateLogicalPath(path); err != nil {
			t.Fatalf("expected path %q to be valid, got error: %v", path, err)
		}
	}

	invalid := []string{
		"",
		"items",
		"/items//",
		"/items//foo",
		"/items/../foo",
		"/_/items",
		"/items/_",
		`/items\foo`,
	}
	for _, path := range invalid {
		if err := ValidateLogicalPath(path); err == nil {
			t.Fatalf("expected path %q to be invalid", path)
		}
	}
}

func TestValidateMetadataPath(t *testing.T) {
	valid := []string{
		"/",
		"/items",
		"/items/",
		"/_/items",
		"/items/_",
		"/admin/realms/_/clients",
		"/admin/realms/_/clients/",
	}
	for _, path := range valid {
		if err := ValidateMetadataPath(path); err != nil {
			t.Fatalf("expected path %q to be valid, got error: %v", path, err)
		}
	}

	invalid := []string{
		"",
		"items",
		"/items//",
		"/items//foo",
		"/items/../foo",
		`/items\foo`,
	}
	for _, path := range invalid {
		if err := ValidateMetadataPath(path); err == nil {
			t.Fatalf("expected path %q to be invalid", path)
		}
	}
}
