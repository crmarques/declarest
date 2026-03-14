package read

import (
	"context"
	"strings"
	"testing"

	appdeps "github.com/crmarques/declarest/internal/app/deps"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

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

func TestRenderMetadataSnapshotPrefersMetadataServiceSnapshotRenderer(t *testing.T) {
	t.Parallel()

	service := &fakeMetadataService{
		resolved: metadatadomain.ResourceMetadata{
			RemoteCollectionPath: "/ignored/by/fallback",
		},
		snapshot: metadatadomain.ResourceMetadata{
			RemoteCollectionPath: "/rendered/from/service",
		},
	}

	rendered, err := renderMetadataSnapshot(
		context.Background(),
		appdeps.Dependencies{Metadata: service},
		"/projects/platform/secrets/path/to/db-password",
		map[string]any{"name": "db-password"},
		resource.PayloadDescriptor{},
	)
	if err != nil {
		t.Fatalf("renderMetadataSnapshot returned error: %v", err)
	}
	if rendered.RemoteCollectionPath != "/rendered/from/service" {
		t.Fatalf("expected snapshot renderer output, got %#v", rendered)
	}
	if service.snapshotCalls != 1 {
		t.Fatalf("expected snapshot renderer to be called once, got %d", service.snapshotCalls)
	}
}

type fakeMetadataService struct {
	resolved      metadatadomain.ResourceMetadata
	snapshot      metadatadomain.ResourceMetadata
	snapshotCalls int
}

func (f *fakeMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (f *fakeMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f *fakeMetadataService) Unset(context.Context, string) error {
	return nil
}

func (f *fakeMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return f.resolved, nil
}

func (f *fakeMetadataService) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

func (f *fakeMetadataService) RenderMetadataSnapshot(
	context.Context,
	string,
	resource.Value,
	resource.PayloadDescriptor,
) (metadatadomain.ResourceMetadata, error) {
	f.snapshotCalls++
	return f.snapshot, nil
}
