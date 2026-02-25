package save

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	repositorydomain "github.com/crmarques/declarest/repository"
	resourcedomain "github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestExtractSaveListItems(t *testing.T) {
	t.Parallel()

	t.Run("array_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems([]any{
			map[string]any{"id": "a"},
			map[string]any{"id": "b"},
		})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if !isList {
			t.Fatal("expected list payload to be detected")
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("items_object_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems(map[string]any{
			"items": []any{
				map[string]any{"id": "a"},
			},
		})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if !isList {
			t.Fatal("expected object with items to be detected as list payload")
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
	})

	t.Run("items_object_invalid_shape", func(t *testing.T) {
		t.Parallel()

		_, _, err := extractSaveListItems(map[string]any{
			"items": map[string]any{"id": "a"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("non_list_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems(map[string]any{"id": "a"})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if isList {
			t.Fatal("expected non-list payload to not be detected as list")
		}
		if items != nil {
			t.Fatalf("expected nil items, got %#v", items)
		}
	})
}

func TestResolveSaveRemoteValue(t *testing.T) {
	t.Parallel()

	t.Run("explicit_collection_target_uses_list_before_get", func(t *testing.T) {
		t.Parallel()

		remoteReader := &fakeSaveRemoteReader{
			listValue: []resourcedomain.Resource{
				{
					LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha",
					Payload:     map[string]any{"id": "mapper-a", "name": "alpha"},
				},
			},
		}

		value, err := resolveSaveRemoteValue(
			context.Background(),
			remoteReader,
			&fakeSaveMetadataService{},
			"/admin/realms/master/user-registry/AD PRD/mappers",
			true,
		)
		if err != nil {
			t.Fatalf("resolveSaveRemoteValue returned error: %v", err)
		}
		items, ok := value.([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("expected one list payload item, got %#v", value)
		}
		if len(remoteReader.getCalls) != 0 {
			t.Fatalf("expected no remote get calls, got %#v", remoteReader.getCalls)
		}
		if !reflect.DeepEqual(remoteReader.listCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one list call, got %#v", remoteReader.listCalls)
		}
	})

	t.Run("not_found_get_uses_collection_branch_fallback_for_non_empty_list", func(t *testing.T) {
		t.Parallel()

		remoteReader := &fakeSaveRemoteReader{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			listValue: []resourcedomain.Resource{
				{
					LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha",
					Payload:     map[string]any{"id": "mapper-a", "name": "alpha"},
				},
				{
					LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/beta",
					Payload:     map[string]any{"id": "mapper-b", "name": "beta"},
				},
			},
		}
		metadataService := &fakeSaveMetadataService{
			collectionChildren: map[string][]string{
				"/admin/realms/master/user-registry/AD PRD": {"mappers"},
			},
		}

		value, err := resolveSaveRemoteValue(
			context.Background(),
			remoteReader,
			metadataService,
			"/admin/realms/master/user-registry/AD PRD/mappers",
			false,
		)
		if err != nil {
			t.Fatalf("resolveSaveRemoteValue returned error: %v", err)
		}
		items, ok := value.([]any)
		if !ok || len(items) != 2 {
			t.Fatalf("expected two list payload items, got %#v", value)
		}
		if !reflect.DeepEqual(remoteReader.getCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one get call, got %#v", remoteReader.getCalls)
		}
		if !reflect.DeepEqual(remoteReader.listCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one list call, got %#v", remoteReader.listCalls)
		}
	})

	t.Run("not_found_get_keeps_not_found_when_non_empty_list_is_not_metadata_collection_branch", func(t *testing.T) {
		t.Parallel()

		notFoundErr := faults.NewTypedError(faults.NotFoundError, "resource not found", nil)
		remoteReader := &fakeSaveRemoteReader{
			getErr: notFoundErr,
			listValue: []resourcedomain.Resource{
				{
					LogicalPath: "/admin/realms/master/user-registry/ldap-1",
					Payload:     map[string]any{"id": "ldap-id-1", "name": "ldap-1"},
				},
			},
		}

		_, err := resolveSaveRemoteValue(
			context.Background(),
			remoteReader,
			&fakeSaveMetadataService{},
			"/admin/realms/master/user-registry",
			false,
		)
		assertTypedCategory(t, err, faults.NotFoundError)
	})
}

func TestResolveSaveEntriesForItems(t *testing.T) {
	t.Parallel()

	t.Run("metadata_alias_resolution_and_deterministic_order", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{AliasFromAttribute: "alias"},
			},
		}

		entries, err := resolveSaveEntriesForItems(context.Background(), deps, "/customers", []any{
			map[string]any{"alias": "zeta", "tier": "pro"},
			map[string]any{"alias": "alpha", "tier": "free"},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/customers/alpha" || entries[1].LogicalPath != "/customers/zeta" {
			t.Fatalf("expected deterministic sorted paths, got %#v", entries)
		}
	})

	t.Run("resource_entry_shape_bypasses_metadata", func(t *testing.T) {
		t.Parallel()

		entries, err := resolveSaveEntriesForItems(context.Background(), Dependencies{}, "/ignored", []any{
			map[string]any{"LogicalPath": "/customers/zeta", "Payload": map[string]any{"id": "zeta"}},
			map[string]any{"LogicalPath": "/customers/alpha", "Payload": map[string]any{"id": "alpha"}},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/customers/alpha" || entries[1].LogicalPath != "/customers/zeta" {
			t.Fatalf("expected deterministic sorted paths, got %#v", entries)
		}
	})

	t.Run("duplicate_resolved_path_fails", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{AliasFromAttribute: "alias"},
			},
		}

		_, err := resolveSaveEntriesForItems(context.Background(), deps, "/customers", []any{
			map[string]any{"alias": "dup"},
			map[string]any{"alias": "dup"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("falls_back_to_common_identity_attributes_when_metadata_is_missing", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{},
			},
		}

		entries, err := resolveSaveEntriesForItems(context.Background(), deps, "/admin/realms/master/clients", []any{
			map[string]any{"id": "uuid-a", "clientId": "alpha"},
			map[string]any{"id": "uuid-b", "clientId": "beta"},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/admin/realms/master/clients/alpha" {
			t.Fatalf("expected first resolved path to use clientId fallback, got %q", entries[0].LogicalPath)
		}
		if entries[1].LogicalPath != "/admin/realms/master/clients/beta" {
			t.Fatalf("expected second resolved path to use clientId fallback, got %q", entries[1].LogicalPath)
		}
	})

	t.Run("falls_back_to_id_when_metadata_attribute_is_missing_in_payload", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{AliasFromAttribute: "clientId"},
			},
		}

		entries, err := resolveSaveEntriesForItems(context.Background(), deps, "/customers", []any{
			map[string]any{"id": "acme"},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/customers/acme" {
			t.Fatalf("expected id fallback path /customers/acme, got %q", entries[0].LogicalPath)
		}
	})

	t.Run("resource_entry_shape_missing_payload_fails", func(t *testing.T) {
		t.Parallel()

		_, err := resolveSaveEntriesForItems(context.Background(), Dependencies{}, "/customers", []any{
			map[string]any{"LogicalPath": "/customers/acme"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestDetectSaveSecretCandidates(t *testing.T) {
	t.Parallel()

	t.Run("metadata_secrets_from_attributes_detects_plaintext", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"credentials": map[string]any{"authValue": "plain-secret"},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}

		if len(candidates) != 1 || candidates[0] != "credentials.authValue" {
			t.Fatalf("expected metadata candidate, got %#v", candidates)
		}
	})

	t.Run("metadata_secrets_from_attributes_ignores_placeholders", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"credentials": map[string]any{"authValue": `{{secret "authValue"}}`},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("expected no candidates for placeholder, got %#v", candidates)
		}
	})

	t.Run("metadata_secrets_from_attributes_ignores_unquoted_placeholders", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"credentials": map[string]any{"authValue": `{{secret custom-auth-key}}`},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("expected no candidates for placeholder, got %#v", candidates)
		}
	})

	t.Run("metadata_secrets_from_attributes_ignores_numeric_values", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{
						"actionTokenGeneratedByUserLifespan.idp-verify-account-via-email",
						"actionTokenGeneratedByUserLifespan.reset-credentials",
						"actionTokenGeneratedByUserLifespan.verify-email",
					},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/admin/realms", map[string]any{
			"actionTokenGeneratedByUserLifespan": map[string]any{
				"idp-verify-account-via-email": "43200",
				"reset-credentials":            "43200",
				"verify-email":                 "43200",
			},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("expected no candidates for numeric-only lifespan values, got %#v", candidates)
		}
	})

	t.Run("metadata_secrets_from_attributes_ignores_boolean_policy_values", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{
						"access.token.claim",
						"access.token.header.type.rfc9068",
						"client.secret.creation.time",
						"client_credentials.use_refresh_token",
						"id.token.claim",
						"introspection.token.claim",
						"standard.token.exchange.enableRefreshRequestedTokenType",
						"standard.token.exchange.enabled",
						"token.response.type.bearer.lower-case",
						"userinfo.token.claim",
					},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/admin/realms/bndes/clients", map[string]any{
			"access": map[string]any{
				"token": map[string]any{
					"claim": true,
					"header": map[string]any{
						"type": map[string]any{
							"rfc9068": false,
						},
					},
				},
			},
			"client": map[string]any{
				"secret": map[string]any{
					"creation": map[string]any{
						"time": 1733075351,
					},
				},
			},
			"client_credentials": map[string]any{
				"use_refresh_token": true,
			},
			"id": map[string]any{
				"token": map[string]any{
					"claim": false,
				},
			},
			"introspection": map[string]any{
				"token": map[string]any{
					"claim": true,
				},
			},
			"standard": map[string]any{
				"token": map[string]any{
					"exchange": map[string]any{
						"enableRefreshRequestedTokenType": false,
						"enabled":                         true,
					},
				},
			},
			"token": map[string]any{
				"response": map[string]any{
					"type": map[string]any{
						"bearer": map[string]any{
							"lower-case": true,
						},
					},
				},
			},
			"userinfo": map[string]any{
				"token": map[string]any{
					"claim": false,
				},
			},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("expected no candidates for boolean/toggle policy values, got %#v", candidates)
		}
	})

	t.Run("falls_back_to_builtin_detection_without_secret_provider", func(t *testing.T) {
		t.Parallel()

		candidates, err := detectSaveSecretCandidates(context.Background(), Dependencies{}, "/customers/acme", map[string]any{
			"password": "plain-secret",
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 1 || candidates[0] != "password" {
			t.Fatalf("expected password candidate, got %#v", candidates)
		}
	})

	t.Run("secret_provider_error_is_returned", func(t *testing.T) {
		t.Parallel()

		expectedErr := faults.NewTypedError(faults.TransportError, "detect failed", nil)
		deps := Dependencies{
			Secrets: &fakeSaveSecretProvider{detectErr: expectedErr},
		}

		_, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"password": "plain-secret",
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected detect error %v, got %v", expectedErr, err)
		}
	})
}

func TestDetectSaveSecretCandidatesForCollection(t *testing.T) {
	t.Parallel()

	t.Run("unions_detected_candidates_across_collection_items", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
		}

		candidates, err := detectSaveSecretCandidatesForCollection(
			context.Background(),
			deps,
			"/customers",
			[]saveEntry{
				{
					LogicalPath: "/customers/acme",
					Payload: map[string]any{
						"id":          "acme",
						"credentials": map[string]any{"authValue": "plain-secret"},
					},
				},
				{
					LogicalPath: "/customers/beta",
					Payload: map[string]any{
						"id":       "beta",
						"password": "pw-123",
					},
				},
			},
		)
		if err != nil {
			t.Fatalf("detectSaveSecretCandidatesForCollection returned error: %v", err)
		}

		expected := []string{"credentials.authValue", "password"}
		if !reflect.DeepEqual(candidates, expected) {
			t.Fatalf("expected candidates %#v, got %#v", expected, candidates)
		}
	})
}

func TestEnforceSaveSecretSafety(t *testing.T) {
	t.Parallel()

	t.Run("fails_without_ignore_when_plaintext_secret_detected", func(t *testing.T) {
		t.Parallel()

		err := enforceSaveSecretSafety(
			context.Background(),
			Dependencies{},
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
			false,
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "warning: potential plaintext secrets detected") {
			t.Fatalf("expected warning in error message, got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "--ignore") {
			t.Fatalf("expected --ignore hint in error message, got %q", err.Error())
		}
	})

	t.Run("allows_plaintext_secret_when_ignore_is_enabled", func(t *testing.T) {
		t.Parallel()

		err := enforceSaveSecretSafety(
			context.Background(),
			Dependencies{},
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
			true,
		)
		if err != nil {
			t.Fatalf("enforceSaveSecretSafety returned error: %v", err)
		}
	})

	t.Run("allows_metadata_declared_plaintext_secret_without_ignore", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"password"},
				},
			},
		}

		err := enforceSaveSecretSafety(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
			false,
		)
		if err != nil {
			t.Fatalf("expected metadata-declared plaintext to pass safety check, got %v", err)
		}
	})
}

func TestAutoHandleDeclaredSaveSecrets(t *testing.T) {
	t.Parallel()

	t.Run("masks_and_stores_metadata_declared_candidates", func(t *testing.T) {
		t.Parallel()

		secretProvider := &fakeSaveSecretProvider{}
		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: secretProvider,
		}

		updatedValue, err := autoHandleDeclaredSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{
				"id": "acme",
				"credentials": map[string]any{
					"authValue": "plain-secret",
				},
			},
		)
		if err != nil {
			t.Fatalf("autoHandleDeclaredSaveSecrets returned error: %v", err)
		}

		payload, ok := updatedValue.(map[string]any)
		if !ok {
			t.Fatalf("expected map payload, got %T", updatedValue)
		}
		credentials, ok := payload["credentials"].(map[string]any)
		if !ok {
			t.Fatalf("expected credentials map, got %T", payload["credentials"])
		}
		if got := credentials["authValue"]; got != `{{secret .}}` {
			t.Fatalf("expected masked metadata-declared secret placeholder, got %#v", got)
		}
		if secretProvider.values["/customers/acme:credentials.authValue"] != "plain-secret" {
			t.Fatalf("expected stored metadata-declared secret, got %#v", secretProvider.values)
		}
	})

	t.Run("fails_when_secret_provider_is_missing_for_metadata_declared_candidates", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"password"},
				},
			},
		}

		_, err := autoHandleDeclaredSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "secret provider is not configured") {
			t.Fatalf("expected missing secret provider error, got %q", err.Error())
		}
	})
}

func TestHandleSaveSecrets(t *testing.T) {
	t.Parallel()

	t.Run("masks_payload_stores_secrets_and_updates_metadata", func(t *testing.T) {
		t.Parallel()

		metadataService := &fakeSaveMetadataService{
			resolved: metadatadomain.ResourceMetadata{
				SecretsFromAttributes: []string{"credentials.authValue"},
			},
			items: map[string]metadatadomain.ResourceMetadata{
				"/customers/acme": {
					IDFromAttribute:       "id",
					SecretsFromAttributes: []string{"existingSecret"},
				},
			},
		}
		secretProvider := &fakeSaveSecretProvider{
			detectedCandidates: []string{"apiToken"},
		}
		deps := Dependencies{
			Metadata: metadataService,
			Secrets:  secretProvider,
		}

		updatedValue, unhandled, err := handleSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{
				"apiToken": "token-123",
				"credentials": map[string]any{
					"authValue": "plain-secret",
				},
			},
			"",
			nil,
		)
		if err != nil {
			t.Fatalf("handleSaveSecrets returned error: %v", err)
		}
		if len(unhandled) != 0 {
			t.Fatalf("expected all candidates handled, got unhandled %#v", unhandled)
		}

		payload, ok := updatedValue.(map[string]any)
		if !ok {
			t.Fatalf("expected map payload, got %T", updatedValue)
		}
		if got := payload["apiToken"]; got != `{{secret .}}` {
			t.Fatalf("expected apiToken placeholder, got %#v", got)
		}
		credentials, ok := payload["credentials"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested credentials map, got %T", payload["credentials"])
		}
		if got := credentials["authValue"]; got != `{{secret .}}` {
			t.Fatalf("expected metadata-path placeholder, got %#v", got)
		}

		if secretProvider.values["/customers/acme:apiToken"] != "token-123" {
			t.Fatalf("expected apiToken stored, got %#v", secretProvider.values)
		}
		if secretProvider.values["/customers/acme:credentials.authValue"] != "plain-secret" {
			t.Fatalf("expected credentials.authValue stored, got %#v", secretProvider.values)
		}

		updatedMetadata := metadataService.items["/customers/acme"]
		expectedAttributes := []string{"apiToken", "credentials.authValue", "existingSecret"}
		if !reflect.DeepEqual(updatedMetadata.SecretsFromAttributes, expectedAttributes) {
			t.Fatalf("expected merged metadata attributes %#v, got %#v", expectedAttributes, updatedMetadata.SecretsFromAttributes)
		}
		if updatedMetadata.IDFromAttribute != "id" {
			t.Fatalf("expected existing metadata fields to be preserved, got %#v", updatedMetadata)
		}
	})

	t.Run("no_candidates_returns_input_and_skips_metadata_write", func(t *testing.T) {
		t.Parallel()

		metadataService := &fakeSaveMetadataService{
			items: map[string]metadatadomain.ResourceMetadata{},
		}
		secretProvider := &fakeSaveSecretProvider{}
		deps := Dependencies{
			Metadata: metadataService,
			Secrets:  secretProvider,
		}

		updatedValue, unhandled, err := handleSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{
				"name": "acme",
			},
			"",
			nil,
		)
		if err != nil {
			t.Fatalf("handleSaveSecrets returned error: %v", err)
		}
		if len(unhandled) != 0 {
			t.Fatalf("expected no unhandled candidates, got %#v", unhandled)
		}
		payload, ok := updatedValue.(map[string]any)
		if !ok {
			t.Fatalf("expected map payload, got %T", updatedValue)
		}
		if got := payload["name"]; got != "acme" {
			t.Fatalf("expected unchanged payload, got %#v", payload)
		}
		if len(secretProvider.values) != 0 {
			t.Fatalf("expected no stored secrets, got %#v", secretProvider.values)
		}
		if len(metadataService.items) != 0 {
			t.Fatalf("expected no metadata writes, got %#v", metadataService.items)
		}
	})

	t.Run("non_object_payload_with_candidates_fails_validation", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{},
			Secrets: &fakeSaveSecretProvider{
				detectedCandidates: []string{"password"},
			},
		}

		_, _, err := handleSaveSecrets(context.Background(), deps, "/customers", []any{
			map[string]any{"password": "plain-secret"},
		}, "", nil)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "--handle-secrets requires object payloads") {
			t.Fatalf("expected non-object validation message, got %q", err.Error())
		}
	})

	t.Run("requested_subset_handles_only_selected_and_returns_unhandled", func(t *testing.T) {
		t.Parallel()

		metadataService := &fakeSaveMetadataService{
			items: map[string]metadatadomain.ResourceMetadata{},
		}
		secretProvider := &fakeSaveSecretProvider{
			detectedCandidates: []string{"apiToken", "password"},
		}
		deps := Dependencies{
			Metadata: metadataService,
			Secrets:  secretProvider,
		}

		updatedValue, unhandled, err := handleSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{
				"apiToken": "token-123",
				"password": "pw-123",
			},
			"",
			[]string{"password"},
		)
		if err != nil {
			t.Fatalf("handleSaveSecrets returned error: %v", err)
		}

		if !reflect.DeepEqual(unhandled, []string{"apiToken"}) {
			t.Fatalf("expected unhandled candidates [apiToken], got %#v", unhandled)
		}

		payload, ok := updatedValue.(map[string]any)
		if !ok {
			t.Fatalf("expected map payload, got %T", updatedValue)
		}
		if got := payload["password"]; got != `{{secret .}}` {
			t.Fatalf("expected handled password placeholder, got %#v", got)
		}
		if got := payload["apiToken"]; got != "token-123" {
			t.Fatalf("expected unhandled apiToken to remain plaintext, got %#v", got)
		}
	})

	t.Run("requested_candidate_not_detected_fails_validation", func(t *testing.T) {
		t.Parallel()

		deps := Dependencies{
			Metadata: &fakeSaveMetadataService{
				items: map[string]metadatadomain.ResourceMetadata{},
			},
			Secrets: &fakeSaveSecretProvider{
				detectedCandidates: []string{"password"},
			},
		}

		_, _, err := handleSaveSecrets(
			context.Background(),
			deps,
			"/customers/acme",
			map[string]any{"password": "pw-123"},
			"",
			[]string{"apiToken"},
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), `requested --handle-secrets attribute "apiToken" was not detected`) {
			t.Fatalf("expected unknown requested candidate error, got %q", err.Error())
		}
	})

	t.Run("list_metadata_target_override_persists_attributes_to_collection_metadata", func(t *testing.T) {
		t.Parallel()

		metadataService := &fakeSaveMetadataService{
			items: map[string]metadatadomain.ResourceMetadata{},
		}
		deps := Dependencies{
			Metadata: metadataService,
			Secrets: &fakeSaveSecretProvider{
				detectedCandidates: []string{"secret"},
			},
		}

		_, unhandled, err := handleSaveSecrets(
			context.Background(),
			deps,
			"/admin/realms/master/clients/app-a",
			map[string]any{"secret": "s-1"},
			"/admin/realms/_/clients",
			[]string{"secret"},
		)
		if err != nil {
			t.Fatalf("handleSaveSecrets returned error: %v", err)
		}
		if len(unhandled) != 0 {
			t.Fatalf("expected no unhandled candidates, got %#v", unhandled)
		}

		metadata := metadataService.items["/admin/realms/_/clients"]
		if !reflect.DeepEqual(metadata.SecretsFromAttributes, []string{"secret"}) {
			t.Fatalf("expected metadata override path to be updated, got %#v", metadata.SecretsFromAttributes)
		}
	})
}

func TestSaveSecretMetadataPathForCollection(t *testing.T) {
	t.Parallel()

	t.Run("keycloak_realm_collection_path_uses_intermediary_placeholder", func(t *testing.T) {
		t.Parallel()

		got := saveSecretMetadataPathForCollection("/admin/realms/master/clients")
		if got != "/admin/realms/_/clients" {
			t.Fatalf("expected intermediary placeholder metadata path, got %q", got)
		}
	})

	t.Run("non_realm_collection_path_is_unchanged", func(t *testing.T) {
		t.Parallel()

		got := saveSecretMetadataPathForCollection("/customers")
		if got != "/customers" {
			t.Fatalf("expected unchanged collection metadata path, got %q", got)
		}
	})
}

func TestEnsureSaveTargetAllowed(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveRepository{
		values: map[string]resourcedomain.Value{
			"/customers/acme": map[string]any{"id": "acme"},
		},
	}

	err := ensureSaveTargetAllowed(context.Background(), repo, "/customers/acme", false)
	assertTypedCategory(t, err, faults.ValidationError)
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected --overwrite hint, got %v", err)
	}

	if err := ensureSaveTargetAllowed(context.Background(), repo, "/customers/acme", true); err != nil {
		t.Fatalf("expected force override to succeed, got %v", err)
	}
}

func TestEnsureSaveEntriesWritable(t *testing.T) {
	t.Parallel()

	repo := &fakeSaveRepository{
		values: map[string]resourcedomain.Value{
			"/customers/acme": map[string]any{"id": "acme"},
		},
	}

	entries := []saveEntry{
		{LogicalPath: "/customers/acme"},
	}

	err := ensureSaveEntriesWritable(context.Background(), repo, entries, false)
	assertTypedCategory(t, err, faults.ValidationError)

	if err := ensureSaveEntriesWritable(context.Background(), repo, entries, true); err != nil {
		t.Fatalf("expected force override to succeed, got %v", err)
	}
}

func TestResourceExistsPropagatesErrors(t *testing.T) {
	t.Parallel()

	expectedErr := faults.NewTypedError(faults.TransportError, "backend", nil)
	repo := &fakeSaveRepository{err: expectedErr}

	_, err := resourceExists(context.Background(), repo, "/customers/acme")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected transport error, got %v", err)
	}
}

func TestIsTypedErrorCategory(t *testing.T) {
	t.Parallel()

	if isTypedErrorCategory(nil, faults.ValidationError) {
		t.Fatal("expected nil error to not match typed category")
	}

	if isTypedErrorCategory(errors.New("plain error"), faults.ValidationError) {
		t.Fatal("expected non-typed error to not match typed category")
	}

	typedErr := faults.NewTypedError(faults.ValidationError, "bad input", nil)
	if !isTypedErrorCategory(typedErr, faults.ValidationError) {
		t.Fatal("expected typed error to match category")
	}
	if isTypedErrorCategory(typedErr, faults.NotFoundError) {
		t.Fatal("expected typed error to not match different category")
	}
}

type fakeSaveMetadataService struct {
	resolved           metadatadomain.ResourceMetadata
	resolveErr         error
	items              map[string]metadatadomain.ResourceMetadata
	collectionChildren map[string][]string
	wildcardChildren   map[string]bool
	wildcardErr        error
}

func (f *fakeSaveMetadataService) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if f.items != nil {
		if metadata, found := f.items[logicalPath]; found {
			return metadata, nil
		}
	}
	return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
}

func (f *fakeSaveMetadataService) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	if f.items == nil {
		f.items = map[string]metadatadomain.ResourceMetadata{}
	}
	f.items[logicalPath] = metadata
	return nil
}

func (f *fakeSaveMetadataService) Unset(context.Context, string) error { return nil }

func (f *fakeSaveMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	if f.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, f.resolveErr
	}
	return f.resolved, nil
}

func (f *fakeSaveMetadataService) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

func (f *fakeSaveMetadataService) Infer(
	context.Context,
	string,
	metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (f *fakeSaveMetadataService) ResolveCollectionChildren(_ context.Context, logicalPath string) ([]string, error) {
	if f.collectionChildren == nil {
		return nil, nil
	}

	children, found := f.collectionChildren[logicalPath]
	if !found {
		return nil, nil
	}
	items := make([]string, len(children))
	copy(items, children)
	return items, nil
}

func (f *fakeSaveMetadataService) HasCollectionWildcardChild(_ context.Context, logicalPath string) (bool, error) {
	if f.wildcardErr != nil {
		return false, f.wildcardErr
	}
	if f.wildcardChildren == nil {
		return false, nil
	}
	return f.wildcardChildren[logicalPath], nil
}

type fakeSaveRemoteReader struct {
	getValue  resourcedomain.Value
	getErr    error
	listValue []resourcedomain.Resource
	listErr   error
	getCalls  []string
	listCalls []string
}

func (f *fakeSaveRemoteReader) GetRemote(_ context.Context, logicalPath string) (resourcedomain.Value, error) {
	f.getCalls = append(f.getCalls, logicalPath)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeSaveRemoteReader) ListRemote(
	_ context.Context,
	logicalPath string,
	_ orchestratordomain.ListPolicy,
) ([]resourcedomain.Resource, error) {
	f.listCalls = append(f.listCalls, logicalPath)
	if f.listErr != nil {
		return nil, f.listErr
	}
	items := make([]resourcedomain.Resource, len(f.listValue))
	copy(items, f.listValue)
	return items, nil
}

type fakeSaveSecretProvider struct {
	detectedCandidates []string
	detectErr          error
	values             map[string]string
}

func (f *fakeSaveSecretProvider) Init(context.Context) error { return nil }
func (f *fakeSaveSecretProvider) Store(_ context.Context, key string, value string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value
	return nil
}
func (f *fakeSaveSecretProvider) Get(_ context.Context, key string) (string, error) {
	value, found := f.values[key]
	if !found {
		return "", faults.NewTypedError(faults.NotFoundError, "secret not found", nil)
	}
	return value, nil
}
func (f *fakeSaveSecretProvider) Delete(context.Context, string) error   { return nil }
func (f *fakeSaveSecretProvider) List(context.Context) ([]string, error) { return nil, nil }
func (f *fakeSaveSecretProvider) MaskPayload(ctx context.Context, value resourcedomain.Value) (resourcedomain.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return f.Store(ctx, key, secretValue)
	})
}
func (f *fakeSaveSecretProvider) ResolvePayload(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeSaveSecretProvider) NormalizeSecretPlaceholders(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeSaveSecretProvider) DetectSecretCandidates(context.Context, resourcedomain.Value) ([]string, error) {
	if f.detectErr != nil {
		return nil, f.detectErr
	}
	return f.detectedCandidates, nil
}

type fakeSaveRepository struct {
	values map[string]resourcedomain.Value
	err    error
}

func (f *fakeSaveRepository) Save(_ context.Context, logicalPath string, value resourcedomain.Value) error {
	if f.values == nil {
		f.values = map[string]resourcedomain.Value{}
	}
	f.values[logicalPath] = value
	return nil
}

func (f *fakeSaveRepository) Get(_ context.Context, logicalPath string) (resourcedomain.Value, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.values != nil {
		if value, found := f.values[logicalPath]; found {
			return value, nil
		}
	}
	return nil, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
}

func (f *fakeSaveRepository) Delete(_ context.Context, _ string, _ repositorydomain.DeletePolicy) error {
	return nil
}

func (f *fakeSaveRepository) List(_ context.Context, _ string, _ repositorydomain.ListPolicy) ([]resourcedomain.Resource, error) {
	return nil, nil
}

func (f *fakeSaveRepository) Exists(_ context.Context, logicalPath string) (bool, error) {
	if f.values == nil {
		return false, nil
	}
	_, found := f.values[logicalPath]
	return found, nil
}

func (f *fakeSaveRepository) Move(context.Context, string, string) error { return nil }
func (f *fakeSaveRepository) Init(context.Context) error                 { return nil }
func (f *fakeSaveRepository) Refresh(context.Context) error              { return nil }
func (f *fakeSaveRepository) Clean(context.Context) error                { return nil }
func (f *fakeSaveRepository) Reset(context.Context, repositorydomain.ResetPolicy) error {
	return nil
}
func (f *fakeSaveRepository) Check(context.Context) error { return nil }
func (f *fakeSaveRepository) Push(context.Context, repositorydomain.PushPolicy) error {
	return nil
}
func (f *fakeSaveRepository) SyncStatus(context.Context) (repositorydomain.SyncReport, error) {
	return repositorydomain.SyncReport{}, nil
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
