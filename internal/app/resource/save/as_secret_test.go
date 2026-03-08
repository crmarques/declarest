package save

import (
	"context"
	"testing"

	"github.com/crmarques/declarest/faults"
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
	secretProvider := &fakeSaveSecretProvider{}
	descriptor := resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{
		Extension: ".key",
	})

	err := saveResolvedPathAsSecret(
		context.Background(),
		Dependencies{Secrets: secretProvider},
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
}

func TestSaveResolvedPathAsSecretRequiresSecretProvider(t *testing.T) {
	t.Parallel()

	err := saveResolvedPathAsSecret(
		context.Background(),
		Dependencies{},
		&fakeSaveWriter{},
		"/customers/acme",
		testSaveContent(map[string]any{"id": "acme"}),
	)
	assertTypedCategory(t, err, faults.ValidationError)
}
