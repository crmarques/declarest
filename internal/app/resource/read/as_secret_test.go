// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package read

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
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
		return "", faults.NotFound("secret not found", nil)
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

func TestMaskSecretsForOutputMasksWholeResourceSecretValue(t *testing.T) {
	t.Parallel()

	wholeSecret := true
	masked, err := maskSecretsForOutput(
		context.Background(),
		Dependencies{
			Metadata: fakeReadMetadataService{
				resolved: metadatadomain.ResourceMetadata{Secret: &wholeSecret},
			},
		},
		"/projects/platform/secrets/private-key",
		resourcedomain.BinaryValue{Bytes: []byte("private-key-bytes")},
	)
	if err != nil {
		t.Fatalf("maskSecretsForOutput returned error: %v", err)
	}
	if masked != "{{secret .}}" {
		t.Fatalf("expected whole-resource secret to be masked to placeholder, got %#v", masked)
	}
}

func TestMaskSecretsForOutputPreservesWholeResourcePlaceholder(t *testing.T) {
	t.Parallel()

	wholeSecret := true
	value := resourcedomain.BinaryValue{Bytes: []byte("{{secret .}}")}
	masked, err := maskSecretsForOutput(
		context.Background(),
		Dependencies{
			Metadata: fakeReadMetadataService{
				resolved: metadatadomain.ResourceMetadata{Secret: &wholeSecret},
			},
		},
		"/projects/platform/secrets/private-key",
		value,
	)
	if err != nil {
		t.Fatalf("maskSecretsForOutput returned error: %v", err)
	}
	binaryValue, ok := masked.(resourcedomain.BinaryValue)
	if !ok {
		t.Fatalf("expected binary placeholder to be preserved, got %T", masked)
	}
	if !bytes.Equal(binaryValue.Bytes, value.Bytes) {
		t.Fatalf("expected preserved placeholder bytes, got %q", string(binaryValue.Bytes))
	}
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

type fakeReadMetadataService struct {
	resolved metadatadomain.ResourceMetadata
}

func (f fakeReadMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, faults.NotFound("metadata not found", nil)
}

func (f fakeReadMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f fakeReadMetadataService) Unset(context.Context, string) error { return nil }

func (f fakeReadMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return f.resolved, nil
}

func (f fakeReadMetadataService) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}
