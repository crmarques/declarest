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

package defaults

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/managedservice"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func managedServiceInferRequest() InferRequest {
	return InferRequest{Sources: []InferSource{InferSourceManagedService}}
}

func managedServiceCheckRequest() CheckRequest {
	return CheckRequest{Sources: []InferSource{InferSourceManagedService}}
}

func TestGetReturnsEmptyObjectWhenDefaultsSidecarIsMissing(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Get(context.Background(), deps, "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	want := map[string]any{}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected defaults payload: got %#v want %#v", result.Content.Value, want)
	}
	if result.Content.Descriptor.PayloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json defaults descriptor, got %#v", result.Content.Descriptor)
	}
}

func TestInferFromRepositoryUsesCommonSiblingValues(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServiceCreatesAndDeletesTemporaryResources(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/_": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	result, err := Infer(context.Background(), deps, "/customers/acme", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}

	if len(orch.createCalls) != 4 {
		t.Fatalf("expected four temporary creates, got %#v", orch.createCalls)
	}
	if len(orch.deleteCalls) != 4 {
		t.Fatalf("expected four cleanup deletes, got %#v", orch.deleteCalls)
	}
	if orch.deleteCalls[0] != orch.createCalls[3].logicalPath ||
		orch.deleteCalls[1] != orch.createCalls[2].logicalPath ||
		orch.deleteCalls[2] != orch.createCalls[1].logicalPath ||
		orch.deleteCalls[3] != orch.createCalls[0].logicalPath {
		t.Fatalf("expected cleanup deletes in reverse order, got creates=%#v deletes=%#v", orch.createCalls, orch.deleteCalls)
	}
}

func TestInferAcceptsCollectionAndResourcePathsForSameCollection(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"id":          "acme-id",
				"realm":       "acme",
				"enabled":     true,
				"sslRequired": "external",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"id":          "master-id",
				"realm":       "master",
				"enabled":     true,
				"sslRequired": "external",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	tests := []struct {
		name             string
		requestedPath    string
		wantResolvedPath string
	}{
		{name: "collection_without_trailing_slash", requestedPath: "/admin/realms", wantResolvedPath: "/admin/realms"},
		{name: "collection_with_trailing_slash", requestedPath: "/admin/realms/", wantResolvedPath: "/admin/realms"},
		{name: "specific_resource", requestedPath: "/admin/realms/master", wantResolvedPath: "/admin/realms"},
	}

	want := map[string]any{
		"enabled":     true,
		"sslRequired": "external",
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Infer(context.Background(), deps, tc.requestedPath, InferRequest{})
			if err != nil {
				t.Fatalf("Infer returned error: %v", err)
			}
			if result.ResolvedPath != tc.wantResolvedPath {
				t.Fatalf("expected resolved path %q, got %q", tc.wantResolvedPath, result.ResolvedPath)
			}
			if !reflect.DeepEqual(result.Content.Value, want) {
				t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
			}
		})
	}
}

func TestInferManagedServiceAcceptsCollectionPathWithOrWithoutTrailingSlash(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "acme",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "master",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/admin/realms/_": {
				ID:    "{{/realm}}",
				Alias: "{{/realm}}",
			},
		},
	}
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)

	tests := []struct {
		name             string
		requestedPath    string
		wantResolvedPath string
	}{
		{name: "collection_without_trailing_slash", requestedPath: "/admin/realms", wantResolvedPath: "/admin/realms"},
		{name: "collection_with_trailing_slash", requestedPath: "/admin/realms/", wantResolvedPath: "/admin/realms"},
		{name: "specific_resource", requestedPath: "/admin/realms/master", wantResolvedPath: "/admin/realms"},
	}

	want := map[string]any{"status": "active"}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := Infer(context.Background(), deps, tc.requestedPath, managedServiceInferRequest())
			if err != nil {
				t.Fatalf("Infer returned error: %v", err)
			}
			if result.ResolvedPath != tc.wantResolvedPath {
				t.Fatalf("expected resolved path %q, got %q", tc.wantResolvedPath, result.ResolvedPath)
			}
			if !reflect.DeepEqual(result.Content.Value, want) {
				t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
			}
		})
	}

	if len(orch.createCalls) != len(tests)*4 {
		t.Fatalf("expected %d temporary creates, got %#v", len(tests)*4, orch.createCalls)
	}
	for idx, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for create %d (%q), got %#v", tempName, idx, call.logicalPath, got)
		}
	}
}

func TestInferManagedServiceRewritesCollectionIdentityFieldWhenMetadataMissing(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "acme",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "master",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)

	result, err := Infer(context.Background(), deps, "/admin/realms/acme", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if result.ResolvedPath != "/admin/realms" {
		t.Fatalf("expected resolved path /admin/realms, got %q", result.ResolvedPath)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}

	if len(orch.createCalls) != 4 {
		t.Fatalf("expected four temporary creates, got %#v", orch.createCalls)
	}
	for _, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for %q, got %#v", tempName, call.logicalPath, got)
		}
		if _, exists := payload["displayName"]; exists {
			t.Fatalf("expected probe payload to keep only inferred identity fields when metadata is missing, got %#v", payload)
		}
		if _, exists := payload["organizationsEnabled"]; exists {
			t.Fatalf("expected probe payload to exclude non-identity fields when metadata is missing, got %#v", payload)
		}
	}
}

func TestInferManagedServiceUsesCreateValidateRequiredAttributesOnlyForProbePayload(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "Acme",
				"enabled":              true,
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "Master",
				"enabled":              true,
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/admin/realms/_": {
				Alias:              "{{/realm}}",
				RequiredAttributes: []string{"/enabled"},
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationCreate): {
						Validate: &metadata.OperationValidationSpec{
							RequiredAttributes: []string{"/displayName"},
						},
					},
				},
			},
		},
	}

	result, err := Infer(context.Background(), deps, "/admin/realms", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Content.Value, map[string]any{"status": "active"}) {
		t.Fatalf("unexpected managed-service defaults: got %#v", result.Content.Value)
	}

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	for _, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for %q, got %#v", tempName, call.logicalPath, got)
		}
		displayName, ok := payload["displayName"].(string)
		if !ok || (displayName != "Acme" && displayName != "Master") {
			t.Fatalf("expected displayName to be preserved from create validate.requiredAttributes, got %#v", payload["displayName"])
		}
		if _, exists := payload["enabled"]; exists {
			t.Fatalf("expected resource.requiredAttributes to be ignored when create validate.requiredAttributes is set, got %#v", payload)
		}
		if _, exists := payload["organizationsEnabled"]; exists {
			t.Fatalf("expected non-required fields to be excluded from probe payload, got %#v", payload)
		}
	}
}

func TestInferManagedServiceFallsBackToResourceRequiredAttributesForProbePayload(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "Acme",
				"enabled":              true,
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "Master",
				"enabled":              true,
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/admin/realms/_": {
				Alias:              "{{/realm}}",
				RequiredAttributes: []string{"/displayName"},
			},
		},
	}

	result, err := Infer(context.Background(), deps, "/admin/realms", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if !reflect.DeepEqual(result.Content.Value, map[string]any{"status": "active"}) {
		t.Fatalf("unexpected managed-service defaults: got %#v", result.Content.Value)
	}

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	for _, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		tempName := path.Base(call.logicalPath)
		if got := payload["realm"]; got != tempName {
			t.Fatalf("expected realm %q for %q, got %#v", tempName, call.logicalPath, got)
		}
		displayName, ok := payload["displayName"].(string)
		if !ok || (displayName != "Acme" && displayName != "Master") {
			t.Fatalf("expected displayName to be preserved from resource.requiredAttributes, got %#v", payload["displayName"])
		}
		if _, exists := payload["enabled"]; exists {
			t.Fatalf("expected probe payload to exclude unrelated fields, got %#v", payload)
		}
		if _, exists := payload["organizationsEnabled"]; exists {
			t.Fatalf("expected probe payload to exclude non-required fields, got %#v", payload)
		}
	}
}

func TestInferFromManagedServiceIgnoresStoredDefaultsValues(t *testing.T) {
	t.Parallel()

	rawContent := map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":   "acme",
				"name": "acme",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	}
	localContent := map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":     "acme",
				"name":   "acme",
				"status": "active",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	}
	deps := testDefaultsDepsWithRepositoryAndLocalContent(rawContent, localContent)
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/_"] = resource.Content{
		Value: map[string]any{
			"status": "active",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, call int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["id"] = path.Base(item.logicalPath)
		payload["name"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/customers/acme", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
	for _, call := range orch.createCalls {
		payload, ok := call.content.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", call.content.Value)
		}
		if _, exists := payload["status"]; exists {
			t.Fatalf("expected probe payload to ignore stored defaults, got %#v", payload)
		}
	}
}

func TestInferManagedServiceRetriesCleanupDeleteAfterAuthError(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/_": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.deleteErr = faults.NewTypedError(faults.AuthError, "remote request failed with status 403: forbidden", nil)

	managedServiceClient := &fakeDefaultsManagedServiceClient{}
	serviceAccessor := deps.Services.(*fakeDefaultsServiceAccessor)
	serviceAccessor.managedService = managedServiceClient

	result, err := Infer(context.Background(), deps, "/customers/acme", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
	if len(orch.deleteCalls) != 4 {
		t.Fatalf("expected four orchestrator delete attempts, got %#v", orch.deleteCalls)
	}
	if managedServiceClient.invalidateCalls != 5 {
		t.Fatalf("expected one probe-read invalidation plus four delete-retry invalidations, got %d", managedServiceClient.invalidateCalls)
	}
	if len(managedServiceClient.requestCalls) != 4 {
		t.Fatalf("expected four direct managed-service delete retries, got %#v", managedServiceClient.requestCalls)
	}
	for _, call := range managedServiceClient.requestCalls {
		if call.Method != "DELETE" {
			t.Fatalf("expected DELETE retry request, got %#v", call)
		}
		if !strings.HasPrefix(call.Path, "/customers/declarest-defaults-probe-") {
			t.Fatalf("unexpected retry path %q", call.Path)
		}
	}
}

func TestInferFromManagedServiceWaitsForStableProbeRead(t *testing.T) {
	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/projects/tenant": {
			Value: map[string]any{
				"id":          "tenant",
				"displayName": "Tenant",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, call int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		if call >= 3 {
			payload["tier"] = "standard"
		}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
		"tier":   "standard",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServiceWaitsBeforeFirstProbeRead(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/projects/tenant": {
			Value: map[string]any{
				"id":          "tenant",
				"displayName": "Tenant",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	wait := 25 * time.Millisecond

	result, err := Infer(context.Background(), deps, "/projects/platform", InferRequest{
		Sources: []InferSource{InferSourceManagedService},
		Wait:    wait,
	})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}
	if orch.lastCreateAt.IsZero() {
		t.Fatal("expected create timestamp to be recorded")
	}
	if orch.firstGetRemoteAt.IsZero() {
		t.Fatal("expected first remote read timestamp to be recorded")
	}
	if delay := orch.firstGetRemoteAt.Sub(orch.lastCreateAt); delay < wait {
		t.Fatalf("expected first remote read delay >= %s, got %s", wait, delay)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServiceIncludesSharedEmptyObjectDefaults(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/projects/tenant": {
			Value: map[string]any{
				"id":          "tenant",
				"displayName": "Tenant",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, _ int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["smtpServer"] = map[string]any{}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"smtpServer": map[string]any{},
		"status":     "active",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServiceInvalidatesAuthCacheBeforeProbeRead(t *testing.T) {
	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/projects/platform": {
			Value: map[string]any{
				"id":          "platform",
				"displayName": "Platform",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/projects/tenant": {
			Value: map[string]any{
				"id":          "tenant",
				"displayName": "Tenant",
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	managedServiceClient := &fakeDefaultsManagedServiceClient{}
	serviceAccessor := deps.Services.(*fakeDefaultsServiceAccessor)
	serviceAccessor.managedService = managedServiceClient

	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	orch.getRemoteFn = func(item savedResource, _ int) (resource.Content, error) {
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		if managedServiceClient.invalidateCalls > 0 {
			payload["tier"] = "standard"
		}
		payload["id"] = path.Base(item.logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}

	result, err := Infer(context.Background(), deps, "/projects/platform", managedServiceInferRequest())
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
		"tier":   "standard",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults: got %#v want %#v", result.Content.Value, want)
	}
	if managedServiceClient.invalidateCalls != 1 {
		t.Fatalf("expected one auth cache invalidation before probe reads, got %d", managedServiceClient.invalidateCalls)
	}
}

func TestInferCollectionPathWithMultipleDirectChildrenUsesSharedDefaults(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Infer(context.Background(), deps, "/customers/", InferRequest{})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{"team": "platform"},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferItemsRestrictsRepositorySamplesByAlias(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":     "acme",
				"name":   "acme",
				"labels": map[string]any{"team": "platform", "region": "east"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/customers/beta": {
			Value: map[string]any{
				"id":     "beta",
				"name":   "beta",
				"labels": map[string]any{"team": "platform", "region": "west"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/customers/gamma": {
			Value: map[string]any{
				"id":     "gamma",
				"name":   "gamma",
				"labels": map[string]any{"team": "security", "region": "west"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{
		Items: []string{"acme", "beta"},
	})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{"team": "platform"},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults for selected aliases: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferManagedServiceUsesSelectedAliasesOnly(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/admin/realms/acme": {
			Value: map[string]any{
				"realm":                "acme",
				"displayName":          "Acme",
				"organizationsEnabled": false,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/master": {
			Value: map[string]any{
				"realm":                "master",
				"displayName":          "Master",
				"organizationsEnabled": false,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/admin/realms/other": {
			Value: map[string]any{
				"realm":                "other",
				"displayName":          "Other",
				"organizationsEnabled": true,
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/admin/realms/_": {
				ID:    "{{/realm}}",
				Alias: "{{/realm}}",
			},
		},
	}
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)

	result, err := Infer(context.Background(), deps, "/admin/realms", InferRequest{
		Sources: []InferSource{InferSourceManagedService},
		Items:   []string{"acme", "master"},
	})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-service defaults for selected aliases: got %#v want %#v", result.Content.Value, want)
	}
	if len(orch.createCalls) != 4 {
		t.Fatalf("expected four temporary creates for two selected aliases, got %#v", orch.createCalls)
	}
}

func TestInferItemsFailsWhenAliasDoesNotExist(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	_, err := Infer(context.Background(), deps, "/customers", InferRequest{
		Items: []string{"missing"},
	})
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "items alias not found") {
		t.Fatalf("expected missing alias validation message, got %v", err)
	}
}

func TestCompactContentAgainstStoredDefaultsReturnsOnlyOverrides(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/_"] = resource.Content{
		Value: map[string]any{
			"status": "active",
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	content, pruned, err := CompactContentAgainstStoredDefaults(context.Background(), deps, "/customers/acme", resource.Content{
		Value: map[string]any{
			"id":     "acme",
			"name":   "acme",
			"status": "active",
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	})
	if err != nil {
		t.Fatalf("CompactContentAgainstStoredDefaults returned error: %v", err)
	}
	if !pruned {
		t.Fatal("expected defaults pruning to be applied")
	}

	want := map[string]any{
		"id":   "acme",
		"name": "acme",
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected pruned payload: got %#v want %#v", content.Value, want)
	}
}

func TestCompactContentAgainstStoredDefaultsRemovesSharedEmptyObjects(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/_"] = resource.Content{
		Value: map[string]any{
			"smtpServer": map[string]any{},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	content, pruned, err := CompactContentAgainstStoredDefaults(context.Background(), deps, "/customers/acme", resource.Content{
		Value: map[string]any{
			"name":       "acme",
			"smtpServer": map[string]any{},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	})
	if err != nil {
		t.Fatalf("CompactContentAgainstStoredDefaults returned error: %v", err)
	}
	if !pruned {
		t.Fatal("expected defaults pruning to be applied")
	}

	want := map[string]any{
		"name": "acme",
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected pruned payload: got %#v want %#v", content.Value, want)
	}
}

func TestCheckMatchesStoredDefaultsWhenInferredDefaultsAreEqual(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/_"] = resource.Content{
		Value: map[string]any{
			"labels": map[string]any{"team": "platform"},
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}

	result, err := Check(context.Background(), deps, "/customers/acme", CheckRequest{})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !result.Matches {
		t.Fatalf("expected matching defaults, got %#v vs %#v", result.CurrentContent.Value, result.InferredContent.Value)
	}
}

func TestCheckDetectsMismatchAgainstManagedServiceInference(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	repo := deps.Repository.(*fakeDefaultsRepository)
	repo.defaults["/customers/_"] = resource.Content{
		Value: map[string]any{
			"status": "inactive",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/_": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	result, err := Check(context.Background(), deps, "/customers/acme", managedServiceCheckRequest())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Matches {
		t.Fatalf("expected mismatching defaults, got %#v vs %#v", result.CurrentContent.Value, result.InferredContent.Value)
	}
	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(result.InferredContent.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.InferredContent.Value, want)
	}
}

type fakeDefaultsOrchestrator struct {
	orchestratordomain.Orchestrator
	localContent     map[string]resource.Content
	createCalls      []savedResource
	deleteCalls      []string
	deleteErr        error
	getRemoteFn      func(item savedResource, call int) (resource.Content, error)
	getCalls         map[string]int
	lastCreateAt     time.Time
	firstGetRemoteAt time.Time
}

type savedResource struct {
	logicalPath string
	content     resource.Content
}

func (f *fakeDefaultsOrchestrator) ResolveLocalResource(_ context.Context, logicalPath string) (resource.Resource, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Resource{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return resource.Resource{
		LogicalPath:       logicalPath,
		CollectionPath:    collectionPathFor(logicalPath),
		LocalAlias:        path.Base(logicalPath),
		Payload:           content.Value,
		PayloadDescriptor: content.Descriptor,
	}, nil
}

func (f *fakeDefaultsOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return content, nil
}

func (f *fakeDefaultsOrchestrator) ListLocal(_ context.Context, logicalPath string, _ orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0, len(f.localContent))
	for candidate := range f.localContent {
		if collectionPathFor(candidate) != logicalPath {
			continue
		}
		items = append(items, resource.Resource{LogicalPath: candidate})
	}
	return items, nil
}

func (f *fakeDefaultsOrchestrator) Create(_ context.Context, logicalPath string, content resource.Content) (resource.Resource, error) {
	f.createCalls = append(f.createCalls, savedResource{logicalPath: logicalPath, content: content})
	f.lastCreateAt = time.Now()
	return resource.Resource{LogicalPath: logicalPath}, nil
}

func (f *fakeDefaultsOrchestrator) GetRemote(_ context.Context, logicalPath string) (resource.Content, error) {
	if f.firstGetRemoteAt.IsZero() {
		f.firstGetRemoteAt = time.Now()
	}
	for _, item := range f.createCalls {
		if item.logicalPath != logicalPath {
			continue
		}
		if f.getCalls == nil {
			f.getCalls = map[string]int{}
		}
		f.getCalls[logicalPath]++
		if f.getRemoteFn != nil {
			return f.getRemoteFn(item, f.getCalls[logicalPath])
		}
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["id"] = path.Base(logicalPath)
		payload["name"] = path.Base(logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}
	return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
}

func (f *fakeDefaultsOrchestrator) Delete(_ context.Context, logicalPath string, _ orchestratordomain.DeletePolicy) error {
	f.deleteCalls = append(f.deleteCalls, logicalPath)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

type fakeDefaultsManagedServiceClient struct {
	managedservice.ManagedServiceClient
	invalidateCalls int
	requestCalls    []managedservice.RequestSpec
	requestErr      error
}

func (f *fakeDefaultsManagedServiceClient) InvalidateAuthCache() {
	f.invalidateCalls++
}

func (f *fakeDefaultsManagedServiceClient) Request(
	_ context.Context,
	spec managedservice.RequestSpec,
) (resource.Content, error) {
	f.requestCalls = append(f.requestCalls, spec)
	if f.requestErr != nil {
		return resource.Content{}, f.requestErr
	}
	return resource.Content{}, nil
}

type fakeDefaultsRepository struct {
	repository.ResourceStore
	content  map[string]resource.Content
	defaults map[string]resource.Content
}

func (f *fakeDefaultsRepository) Save(_ context.Context, logicalPath string, content resource.Content) error {
	if f.content == nil {
		f.content = map[string]resource.Content{}
	}
	f.content[logicalPath] = content
	return nil
}
func (f *fakeDefaultsRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.content[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
	}
	return content, nil
}
func (f *fakeDefaultsRepository) Delete(_ context.Context, logicalPath string, _ repository.DeletePolicy) error {
	delete(f.content, logicalPath)
	return nil
}
func (f *fakeDefaultsRepository) List(_ context.Context, logicalPath string, _ repository.ListPolicy) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0, len(f.content))
	for candidate := range f.content {
		if collectionPathFor(candidate) != logicalPath {
			continue
		}
		items = append(items, resource.Resource{LogicalPath: candidate})
	}
	return items, nil
}
func (f *fakeDefaultsRepository) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeDefaultsRepository) GetDefaults(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.defaults[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "defaults not found", nil)
	}
	return content, nil
}
func (f *fakeDefaultsRepository) SaveDefaults(_ context.Context, logicalPath string, content resource.Content) error {
	if f.defaults == nil {
		f.defaults = map[string]resource.Content{}
	}
	f.defaults[logicalPath] = content
	return nil
}

type fakeDefaultsMetadata struct {
	metadata.MetadataService
	items    map[string]metadata.ResourceMetadata
	defaults *map[string]resource.Content
}

func (f *fakeDefaultsMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadata.ResourceMetadata, error) {
	resolved := metadata.ResourceMetadata{}
	if inheritedPath, ok := fakeCollectionMetadataPath(logicalPath); ok {
		if item, found := f.items[inheritedPath]; found {
			resolved = metadata.MergeResourceMetadata(resolved, item)
		}
		if f.defaults != nil {
			if value, found := (*f.defaults)[inheritedPath]; found {
				resolved = metadata.MergeResourceMetadata(resolved, metadata.ResourceMetadata{
					Defaults: &metadata.DefaultsSpec{Value: value.Value},
				})
			}
		}
	}
	if item, found := f.items[logicalPath]; found {
		resolved = metadata.MergeResourceMetadata(resolved, item)
	}
	if f.defaults != nil {
		if value, found := (*f.defaults)[logicalPath]; found {
			resolved = metadata.MergeResourceMetadata(resolved, metadata.ResourceMetadata{
				Defaults: &metadata.DefaultsSpec{Value: value.Value},
			})
		}
	}
	return resolved, nil
}

func fakeCollectionMetadataPath(logicalPath string) (string, bool) {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" || trimmed == "/" || strings.HasSuffix(trimmed, "/_") {
		return "", false
	}
	collectionPath := path.Dir(trimmed)
	if collectionPath == "." {
		collectionPath = "/"
	}
	return collectionMetadataPath(collectionPath), true
}

type fakeDefaultsServiceAccessor struct {
	store          repository.ResourceStore
	metadata       metadata.MetadataService
	secrets        secretdomain.SecretProvider
	managedService managedservice.ManagedServiceClient
}

func (f *fakeDefaultsServiceAccessor) RepositoryStore() repository.ResourceStore   { return f.store }
func (f *fakeDefaultsServiceAccessor) RepositorySync() repository.RepositorySync   { return nil }
func (f *fakeDefaultsServiceAccessor) MetadataService() metadata.MetadataService   { return f.metadata }
func (f *fakeDefaultsServiceAccessor) SecretProvider() secretdomain.SecretProvider { return f.secrets }
func (f *fakeDefaultsServiceAccessor) ManagedServiceClient() managedservice.ManagedServiceClient {
	return f.managedService
}

func testDefaultsDeps() appdeps.Dependencies {
	return testDefaultsDepsWithLocalContent(map[string]resource.Content{
		"/customers/acme": {
			Value: map[string]any{
				"id":     "acme",
				"name":   "acme",
				"status": "custom-a",
				"labels": map[string]any{"team": "platform"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
		"/customers/beta": {
			Value: map[string]any{
				"id":     "beta",
				"name":   "beta",
				"status": "custom-b",
				"labels": map[string]any{"team": "platform"},
			},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		},
	})
}

func testDefaultsDepsWithLocalContent(localContent map[string]resource.Content) appdeps.Dependencies {
	return testDefaultsDepsWithRepositoryAndLocalContent(localContent, localContent)
}

func testDefaultsDepsWithRepositoryAndLocalContent(
	repositoryContent map[string]resource.Content,
	localContent map[string]resource.Content,
) appdeps.Dependencies {
	orch := &fakeDefaultsOrchestrator{
		localContent: localContent,
	}
	repo := &fakeDefaultsRepository{
		content:  repositoryContent,
		defaults: map[string]resource.Content{},
	}
	md := &fakeDefaultsMetadata{items: map[string]metadata.ResourceMetadata{}, defaults: &repo.defaults}

	return appdeps.Dependencies{
		Orchestrator: orch,
		Repository:   repo,
		Metadata:     md,
		Services: &fakeDefaultsServiceAccessor{
			store:    repo,
			metadata: md,
		},
	}
}
