package read

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	resourcedomain "github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type fakeReadSecretProvider struct {
	values map[string]string
}

func (f *fakeReadSecretProvider) Init(context.Context) error { return nil }
func (f *fakeReadSecretProvider) Store(context.Context, string, string) error {
	return nil
}
func (f *fakeReadSecretProvider) Get(_ context.Context, key string) (string, error) {
	value, found := f.values[key]
	if !found {
		return "", faults.NewTypedError(faults.NotFoundError, "secret not found", nil)
	}
	return value, nil
}
func (f *fakeReadSecretProvider) Delete(context.Context, string) error   { return nil }
func (f *fakeReadSecretProvider) List(context.Context) ([]string, error) { return nil, nil }
func (f *fakeReadSecretProvider) MaskPayload(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeReadSecretProvider) ResolvePayload(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeReadSecretProvider) NormalizeSecretPlaceholders(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeReadSecretProvider) DetectSecretCandidates(context.Context, resourcedomain.Value) ([]string, error) {
	return nil, nil
}

func TestResolveSecretsForOutputResolvesWholeResourcePlaceholder(t *testing.T) {
	t.Parallel()

	descriptor := resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{
		Extension: ".key",
	})
	secretValue, err := secretdomain.EncodeWholeResourceSecret(resourcedomain.Content{
		Value:      resourcedomain.BinaryValue{Bytes: []byte("private-key-bytes")},
		Descriptor: descriptor,
	})
	if err != nil {
		t.Fatalf("EncodeWholeResourceSecret returned error: %v", err)
	}

	resolved, err := resolveSecretsForOutput(
		context.Background(),
		Dependencies{
			Secrets: &fakeReadSecretProvider{
				values: map[string]string{
					"/projects/platform/secrets/private-key:.": secretValue,
				},
			},
		},
		"/projects/platform/secrets/private-key",
		resourcedomain.BinaryValue{Bytes: []byte("{{secret .}}")},
		descriptor,
	)
	if err != nil {
		t.Fatalf("resolveSecretsForOutput returned error: %v", err)
	}

	binaryValue, ok := resolved.(resourcedomain.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue, got %T", resolved)
	}
	if !bytes.Equal(binaryValue.Bytes, []byte("private-key-bytes")) {
		t.Fatalf("expected decoded whole-resource secret bytes, got %q", string(binaryValue.Bytes))
	}
}

func TestResolveSecretsForOutputWholeResourcePlaceholderRequiresSecretProvider(t *testing.T) {
	t.Parallel()

	descriptor := resourcedomain.NormalizePayloadDescriptor(resourcedomain.PayloadDescriptor{
		PayloadType: resourcedomain.PayloadTypeJSON,
	})

	_, err := resolveSecretsForOutput(
		context.Background(),
		Dependencies{},
		"/customers/acme",
		"{{secret .}}",
		descriptor,
	)
	assertTypedCategory(t, err, faults.ValidationError)
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
