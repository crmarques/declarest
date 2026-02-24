package read

import (
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestNormalizeSource(t *testing.T) {
	t.Parallel()

	t.Run("remote_by_default", func(t *testing.T) {
		t.Parallel()
		got, err := NormalizeSource(false, false)
		if err != nil {
			t.Fatalf("NormalizeSource returned error: %v", err)
		}
		if got != SourceRemoteServer {
			t.Fatalf("expected default source %q, got %q", SourceRemoteServer, got)
		}
	})

	t.Run("repository_when_selected", func(t *testing.T) {
		t.Parallel()
		got, err := NormalizeSource(true, false)
		if err != nil {
			t.Fatalf("NormalizeSource returned error: %v", err)
		}
		if got != SourceRepository {
			t.Fatalf("expected source %q, got %q", SourceRepository, got)
		}
	})

	t.Run("rejects_conflicting_flags", func(t *testing.T) {
		t.Parallel()
		_, err := NormalizeSource(true, true)
		if err == nil {
			t.Fatal("expected error")
		}
		if !faults.IsCategory(err, faults.ValidationError) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}

func TestHasCollectionTargetMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "", want: false},
		{path: "   ", want: false},
		{path: "/", want: false},
		{path: "/customers", want: false},
		{path: "/customers/", want: true},
		{path: " /customers/ ", want: true},
	}

	for _, tt := range tests {
		if got := HasCollectionTargetMarker(tt.path); got != tt.want {
			t.Fatalf("unexpected collection marker detection for %q: got=%v want=%v", tt.path, got, tt.want)
		}
	}
}

func TestRenderTextLinesAndResultHelpers(t *testing.T) {
	t.Parallel()

	lines := []string{"/a", "/b"}
	render := RenderTextLines(lines)
	got := render(nil)
	if strings.Join(got, ",") != "/a,/b" {
		t.Fatalf("unexpected rendered lines: %#v", got)
	}

	result := Result{OutputValue: map[string]any{"id": "a"}, TextLines: lines}
	if !result.HasTextLines() {
		t.Fatal("expected HasTextLines=true")
	}
	if !strings.Contains(result.String(), "read.Result") {
		t.Fatalf("expected Result.String to include type label, got %q", result.String())
	}
	if !strings.Contains(result.String(), "lines:2") {
		t.Fatalf("expected Result.String to include line count, got %q", result.String())
	}
}
