package cmd

import "testing"

func TestParseDebugSettings(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		settings, err := parseDebugSettings("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.enabled {
			t.Fatalf("expected debug disabled")
		}
	})

	t.Run("debug-all", func(t *testing.T) {
		settings, err := parseDebugSettings(debugGroupAll)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !settings.enabled {
			t.Fatalf("expected debug enabled")
		}
		for _, group := range debugGroupOrder {
			if !settings.groups[group] {
				t.Fatalf("expected group %s enabled", group)
			}
		}
	})

	t.Run("specific-groups", func(t *testing.T) {
		settings, err := parseDebugSettings("network,repository")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !settings.groups[debugGroupNetwork] || !settings.groups[debugGroupRepository] {
			t.Fatalf("expected network and repository groups enabled")
		}
		if settings.groups[debugGroupResource] {
			t.Fatalf("did not expect resource group enabled")
		}
	})

	t.Run("unknown-group", func(t *testing.T) {
		if _, err := parseDebugSettings("nope"); err == nil {
			t.Fatalf("expected error for unknown group")
		}
	})
}
