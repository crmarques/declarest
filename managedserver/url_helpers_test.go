package managedserver

import "testing"

func TestIsHTTPURL(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		match bool
	}{
		{name: "http", raw: "http://example.com", match: true},
		{name: "https", raw: "https://example.com", match: true},
		{name: "trimmed", raw: "  https://example.com/spec  ", match: true},
		{name: "file", raw: "./openapi.yaml", match: false},
		{name: "empty", raw: "", match: false},
	}
	for _, tt := range tests {
		if got := IsHTTPURL(tt.raw); got != tt.match {
			t.Fatalf("%s: expected %v, got %v", tt.name, tt.match, got)
		}
	}
}
