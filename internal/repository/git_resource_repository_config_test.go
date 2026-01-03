package repository

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGitResourceRepositoryRemoteConfigUnmarshalAutoSyncCompat(t *testing.T) {
	input := []byte(`
remote:
  url: https://example.com/repo.git
  auto-sync: true
`)

	var cfg GitResourceRepositoryConfig
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Remote == nil || cfg.Remote.AutoSync == nil {
		t.Fatalf("expected autosync to be set")
	}
	if !*cfg.Remote.AutoSync {
		t.Fatalf("expected autosync to be true")
	}
}
