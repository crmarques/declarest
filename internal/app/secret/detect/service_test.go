package detect

import (
	"context"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type fakeOrchestrator struct {
	orchestratordomain.Orchestrator
	listItems   []resource.Resource
	getValues   map[string]resource.Value
	listErr     error
	getErr      error
	lastList    string
	lastPolicy  orchestratordomain.ListPolicy
	getRequests []string
}

func (f *fakeOrchestrator) ListLocal(_ context.Context, logicalPath string, policy orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	f.lastList = logicalPath
	f.lastPolicy = policy
	if f.listErr != nil {
		return nil, f.listErr
	}
	items := make([]resource.Resource, len(f.listItems))
	copy(items, f.listItems)
	return items, nil
}

func (f *fakeOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Value, error) {
	f.getRequests = append(f.getRequests, logicalPath)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValues[logicalPath], nil
}

type fakeSecretProvider struct {
	secretdomain.SecretProvider
	detected map[string][]string
	err      error
}

func (f *fakeSecretProvider) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := ""
	if payload, ok := value.(map[string]any); ok {
		if id, ok := payload["id"].(string); ok {
			key = id
		}
	}
	items := f.detected[key]
	out := make([]string, len(items))
	copy(out, items)
	return out, nil
}

type fakeMetadata struct {
	metadatadomain.MetadataService
	setCalls []struct {
		path string
		meta metadatadomain.ResourceMetadata
	}
}

func (f *fakeMetadata) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
}

func (f *fakeMetadata) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	f.setCalls = append(f.setCalls, struct {
		path string
		meta metadatadomain.ResourceMetadata
	}{path: logicalPath, meta: metadata})
	return nil
}

func TestExecuteRequiresSecretProvider(t *testing.T) {
	t.Parallel()

	_, err := Execute(context.Background(), Dependencies{}, Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExecuteRepositoryScanDefaultsToRootAndSortsResults(t *testing.T) {
	t.Parallel()

	orch := &fakeOrchestrator{
		listItems: []resource.Resource{
			{LogicalPath: "/b"},
			{LogicalPath: "/a"},
		},
		getValues: map[string]resource.Value{
			"/a": map[string]any{"id": "a"},
			"/b": map[string]any{"id": "b"},
		},
	}
	secrets := &fakeSecretProvider{
		detected: map[string][]string{
			"a": {"token", "password", "token"},
			"b": {"apikey"},
		},
	}

	result, err := Execute(context.Background(), Dependencies{
		Orchestrator:   orch,
		SecretProvider: secrets,
	}, Request{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if orch.lastList != "/" {
		t.Fatalf("expected default scan path '/', got %q", orch.lastList)
	}
	if !orch.lastPolicy.Recursive {
		t.Fatal("expected recursive repository scan")
	}
	if !reflect.DeepEqual(orch.getRequests, []string{"/a", "/b"}) {
		t.Fatalf("expected sorted get order, got %#v", orch.getRequests)
	}

	items, ok := result.Output.([]DetectedResourceSecrets)
	if !ok {
		t.Fatalf("expected detected resource list output, got %T", result.Output)
	}
	want := []DetectedResourceSecrets{
		{LogicalPath: "/a", Attributes: []string{"password", "token"}},
		{LogicalPath: "/b", Attributes: []string{"apikey"}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("unexpected detect results: got=%#v want=%#v", items, want)
	}
}

func TestExecuteInputModeFixRequiresPath(t *testing.T) {
	t.Parallel()

	secrets := &fakeSecretProvider{detected: map[string][]string{"": {"password"}}}
	_, err := Execute(context.Background(), Dependencies{
		SecretProvider: secrets,
	}, Request{
		HasInput: true,
		Fix:      true,
		Value:    map[string]any{"id": "x", "password": "secret"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExecuteRepositoryScanRejectsUndetectedSecretAttributeFilter(t *testing.T) {
	t.Parallel()

	orch := &fakeOrchestrator{
		listItems: []resource.Resource{{LogicalPath: "/a"}},
		getValues: map[string]resource.Value{
			"/a": map[string]any{"id": "a"},
		},
	}
	secrets := &fakeSecretProvider{
		detected: map[string][]string{
			"a": {"password"},
		},
	}

	_, err := Execute(context.Background(), Dependencies{
		Orchestrator:   orch,
		SecretProvider: secrets,
	}, Request{
		SecretAttribute: "token",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestExecuteFixPersistsDetectedAttributes(t *testing.T) {
	t.Parallel()

	orch := &fakeOrchestrator{
		listItems: []resource.Resource{{LogicalPath: "/a"}},
		getValues: map[string]resource.Value{
			"/a": map[string]any{"id": "a"},
		},
	}
	secrets := &fakeSecretProvider{detected: map[string][]string{"a": {"token"}}}
	meta := &fakeMetadata{}

	_, err := Execute(context.Background(), Dependencies{
		Orchestrator:   orch,
		Metadata:       meta,
		SecretProvider: secrets,
	}, Request{Fix: true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(meta.setCalls) != 1 {
		t.Fatalf("expected one metadata set call, got %d", len(meta.setCalls))
	}
	if meta.setCalls[0].path != "/a" {
		t.Fatalf("expected metadata set for /a, got %q", meta.setCalls[0].path)
	}
	if !reflect.DeepEqual(meta.setCalls[0].meta.SecretsFromAttributes, []string{"token"}) {
		t.Fatalf("unexpected metadata secret attributes: %#v", meta.setCalls[0].meta.SecretsFromAttributes)
	}
}
