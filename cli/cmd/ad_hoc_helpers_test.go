package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAdHocHeader(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
		wantErr bool
	}{
		{"Foo: bar", "Foo", "bar", false},
		{"Foo=bar", "Foo", "bar", false},
		{"invalid", "", "", true},
	}

	for _, tt := range tests {
		key, value, err := parseAdHocHeader(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error parsing %q", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseAdHocHeader(%q) unexpected error: %v", tt.input, err)
		}
		if key != tt.wantKey || value != tt.wantVal {
			t.Fatalf("parseAdHocHeader(%q) = %q/%q, want %q/%q", tt.input, key, value, tt.wantKey, tt.wantVal)
		}
	}
}

func TestLoadAdHocPayload(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "data.json")
	if err := os.WriteFile(filePath, []byte("{\"ok\":true}"), 0o600); err != nil {
		t.Fatalf("write payload file: %v", err)
	}

	tests := []struct {
		input        string
		wantContains string
		wantErr      bool
	}{
		{"{\"value\": \"inline\"}", "inline", false},
		{"@" + filePath, "true", false},
		{"@ ", "", true},
	}

	for _, tt := range tests {
		data, err := loadAdHocPayload(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error loading %q", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("loadAdHocPayload(%q) unexpected error: %v", tt.input, err)
		}
		if !strings.Contains(string(data), tt.wantContains) {
			t.Fatalf("loadAdHocPayload(%q) produced %q, want to contain %q", tt.input, string(data), tt.wantContains)
		}
	}

	if _, err := loadAdHocPayload("@" + filepath.Join(tempDir, "missing")); err == nil {
		t.Fatal("expected error for missing payload file")
	}
}
