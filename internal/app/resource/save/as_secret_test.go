package save

import (
	"context"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
)

type fakeSaveWriter struct {
	logicalPath string
	content     resourcedomain.Content
	callCount   int
}

func (f *fakeSaveWriter) Save(_ context.Context, logicalPath string, content resourcedomain.Content) error {
	f.logicalPath = logicalPath
	f.content = content
	f.callCount++
	return nil
}

func TestSaveResolvedPathAsSecretStoresWholeResourceAndWritesPlaceholder(t *testing.T) {
	t.Parallel()

	writer := &fakeSaveWriter{}
	metadataStore := &fakeSaveMetadataStore{}
	secretProvider := &fakeSaveSecretProvider{}
	descriptor := resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{
		Extension: ".key",
	})

	err := saveResolvedPathAsSecret(
		context.Background(),
		Dependencies{Metadata: metadataStore, Secrets: secretProvider},
		writer,
		"/projects/platform/secrets/private-key",
		resourcedomain.Content{
			Value:      resourcedomain.BinaryValue{Bytes: []byte("private-key-bytes")},
			Descriptor: descriptor,
		},
	)
	if err != nil {
		t.Fatalf("saveResolvedPathAsSecret returned error: %v", err)
	}

	if got := secretProvider.values["/projects/platform/secrets/private-key:."]; got != "private-key-bytes" {
		t.Fatalf("expected whole-resource secret to be stored under root key, got %#v", secretProvider.values)
	}
	if writer.callCount != 1 {
		t.Fatalf("expected one repository write, got %d", writer.callCount)
	}
	if writer.logicalPath != "/projects/platform/secrets/private-key" {
		t.Fatalf("expected placeholder save at original path, got %q", writer.logicalPath)
	}
	if writer.content.Descriptor.Extension != ".key" {
		t.Fatalf("expected placeholder save to preserve extension, got %q", writer.content.Descriptor.Extension)
	}
	placeholder, ok := writer.content.Value.(resourcedomain.BinaryValue)
	if !ok {
		t.Fatalf("expected binary placeholder content, got %T", writer.content.Value)
	}
	if string(placeholder.Bytes) != "{{secret .}}" {
		t.Fatalf("expected whole-resource placeholder bytes, got %q", string(placeholder.Bytes))
	}
	if !metadataStore.items["/projects/platform/secrets/private-key"].IsWholeResourceSecret() {
		t.Fatalf("expected secret metadata to be persisted, got %#v", metadataStore.items["/projects/platform/secrets/private-key"])
	}
}

func TestSaveResolvedPathAsSecretRequiresMetadataServiceBeforeSideEffects(t *testing.T) {
	t.Parallel()

	writer := &fakeSaveWriter{}
	secretProvider := &fakeSaveSecretProvider{}

	err := saveResolvedPathAsSecret(
		context.Background(),
		Dependencies{Secrets: secretProvider},
		writer,
		"/customers/acme",
		testSaveContent(map[string]any{"id": "acme"}),
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if err.Error() != "metadata service is not configured" {
		t.Fatalf("expected metadata service validation error, got %q", err)
	}
	if writer.callCount != 0 {
		t.Fatalf("expected no repository writes when metadata service is missing, got %d", writer.callCount)
	}
	if len(secretProvider.values) != 0 {
		t.Fatalf("expected no secret writes when metadata service is missing, got %#v", secretProvider.values)
	}
}

func TestSaveResolvedPathAsSecretRequiresSecretProvider(t *testing.T) {
	t.Parallel()

	err := saveResolvedPathAsSecret(
		context.Background(),
		Dependencies{Metadata: &fakeSaveMetadataStore{}},
		&fakeSaveWriter{},
		"/customers/acme",
		testSaveContent(map[string]any{"id": "acme"}),
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if err.Error() != "secret provider is not configured" {
		t.Fatalf("expected secret provider validation error, got %q", err)
	}
}

type fakeSaveMetadataStore struct {
	items map[string]metadatadomain.ResourceMetadata
}

func (f *fakeSaveMetadataStore) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if f.items != nil {
		if metadata, found := f.items[logicalPath]; found {
			return metadata, nil
		}
	}
	return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
}

func (f *fakeSaveMetadataStore) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	if f.items == nil {
		f.items = map[string]metadatadomain.ResourceMetadata{}
	}
	f.items[logicalPath] = metadata
	return nil
}

func (f *fakeSaveMetadataStore) Unset(context.Context, string) error { return nil }

func (f *fakeSaveMetadataStore) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (f *fakeSaveMetadataStore) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}
