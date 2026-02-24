package workflow

import (
	"reflect"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestDedupeAndSortAttributes(t *testing.T) {
	t.Parallel()

	got := DedupeAndSortAttributes([]string{" password ", "token", "password", "", " token "})
	want := []string{"password", "token"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected attributes: got=%#v want=%#v", got, want)
	}
}

func TestIsPlaceholderValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "default_placeholder", value: "{{secret .}}", want: true},
		{name: "quoted_custom_key", value: `{{secret "path:key"}}`, want: true},
		{name: "bare_custom_key", value: "{{secret path:key}}", want: true},
		{name: "wrong_function", value: "{{notsecret .}}", want: false},
		{name: "empty_arg", value: "{{secret \"\"}}", want: false},
		{name: "missing_braces", value: "secret .", want: false},
		{name: "extra_tokens", value: "{{secret a b}}", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPlaceholderValue(tt.value); got != tt.want {
				t.Fatalf("unexpected placeholder detection for %q: got=%v want=%v", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsLikelyPlaintextValueFalsePositiveGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "123456", want: false},
		{value: "true", want: false},
		{value: "Enabled", want: false},
		{value: "s3cr3t", want: true},
		{value: "token-value", want: true},
	}

	for _, tt := range tests {
		if got := IsLikelyPlaintextValue(tt.value); got != tt.want {
			t.Fatalf("unexpected plaintext heuristic for %q: got=%v want=%v", tt.value, got, tt.want)
		}
	}
}

func TestResolveAttributePathsAndMaskValue(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"name": "alpha",
		"auth": map[string]any{
			"password": "plain",
			"token":    "{{secret .}}",
		},
		"items": []any{
			map[string]any{"password": "array-secret-ignored-by-map-path-discovery"},
		},
	}

	paths := ResolveAttributePaths(payload, []string{"password", "auth.token"})
	wantPaths := []string{"auth.password", "auth.token"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("unexpected attribute paths: got=%#v want=%#v", paths, wantPaths)
	}

	masked, err := MaskValue(resource.Value(payload), []string{"password"})
	if err != nil {
		t.Fatalf("MaskValue returned error: %v", err)
	}

	maskedMap, ok := masked.(map[string]any)
	if !ok {
		t.Fatalf("expected masked payload map, got %T", masked)
	}
	auth, ok := maskedMap["auth"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested auth map, got %T", maskedMap["auth"])
	}
	if got := auth["password"]; got != PlaceholderValue() {
		t.Fatalf("expected nested password to be masked, got %#v", got)
	}
	if got := auth["token"]; got != "{{secret .}}" {
		t.Fatalf("expected existing placeholder to be preserved, got %#v", got)
	}
}

func TestResolveAttributePathsForCandidates(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"password": "root-secret",
		"db": map[string]any{
			"password": "db-secret",
			"user":     "declarest",
		},
		"empty": "",
	}

	got := ResolveAttributePathsForCandidates(payload, []string{"password", "db.user", "missing"})
	want := []string{"db.password", "db.user", "password"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected candidate paths: got=%#v want=%#v", got, want)
	}
}
