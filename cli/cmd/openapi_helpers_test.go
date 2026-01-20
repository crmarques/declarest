package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"
)

const sampleOpenAPISpec = `
openapi: 3.0.0
paths:
  /items:
    post:
      responses:
        "200":
          description: ok
`

type testOpenAPISpecProvider struct {
	spec *openapi.Spec
}

func (p *testOpenAPISpecProvider) GetResourceRecord(path string) (resource.ResourceRecord, error) {
	return resource.ResourceRecord{}, nil
}

func (p *testOpenAPISpecProvider) GetMergedMetadata(path string) (resource.ResourceMetadata, error) {
	return resource.ResourceMetadata{}, nil
}

func (p *testOpenAPISpecProvider) OpenAPISpec() *openapi.Spec {
	return p.spec
}

func TestResolveOpenAPISpecUsesProvider(t *testing.T) {
	spec := mustParseSampleSpec(t, sampleOpenAPISpec)
	recon := &reconciler.DefaultReconciler{
		ResourceRecordProvider: &testOpenAPISpecProvider{spec: spec},
	}

	got, err := resolveOpenAPISpec(recon, "")
	if err != nil {
		t.Fatalf("resolveOpenAPISpec returned error: %v", err)
	}
	if got != spec {
		t.Fatalf("expected provider spec to be returned")
	}
}

func TestResolveOpenAPISpecMissing(t *testing.T) {
	_, err := resolveOpenAPISpec(&reconciler.DefaultReconciler{}, "")
	if !errors.Is(err, errOpenAPISpecNotConfigured) {
		t.Fatalf("expected errOpenAPISpecNotConfigured, got %v", err)
	}
}

func TestResolveOpenAPISpecFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(path, []byte(sampleOpenAPISpec), 0o600); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	spec, err := resolveOpenAPISpec(&reconciler.DefaultReconciler{}, path)
	if err != nil {
		t.Fatalf("resolveOpenAPISpec returned error: %v", err)
	}
	if spec == nil || len(spec.Paths) == 0 {
		t.Fatalf("expected spec to be parsed from %s", path)
	}
}

func TestResolveOpenAPISpecHTTPRequiresManager(t *testing.T) {
	_, err := resolveOpenAPISpec(&reconciler.DefaultReconciler{}, "http://example.com/openapi.yaml")
	if err == nil || !strings.Contains(err.Error(), "http managed server") {
		t.Fatalf("expected http managed server error, got %v", err)
	}
}

func mustParseSampleSpec(t *testing.T, data string) *openapi.Spec {
	t.Helper()
	spec, err := openapi.ParseSpec([]byte(data))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	return spec
}
