package secrets

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestNormalizePlaceholders(t *testing.T) {
	t.Parallel()

	t.Run("normalizes_current_key_placeholders", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"apiToken": "{{ secret . }}",
			"nested": map[string]any{
				"clientSecret": "{{secret \"clientSecret\"}}",
			},
			"literal": "prefix {{secret .}} suffix",
		}

		got, err := NormalizePlaceholders(input)
		if err != nil {
			t.Fatalf("NormalizePlaceholders returned error: %v", err)
		}

		expected := map[string]any{
			"apiToken": "{{secret .}}",
			"nested": map[string]any{
				"clientSecret": "{{secret \"clientSecret\"}}",
			},
			"literal": "prefix {{secret .}} suffix",
		}

		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
	})

	t.Run("rejects_current_key_placeholder_without_scope", func(t *testing.T) {
		t.Parallel()

		_, err := NormalizePlaceholders("{{secret .}}")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("accepts_unquoted_explicit_key", func(t *testing.T) {
		t.Parallel()

		got, err := NormalizePlaceholders(map[string]any{
			"apiToken": "{{ secret apiToken }}",
		})
		if err != nil {
			t.Fatalf("NormalizePlaceholders returned error: %v", err)
		}

		expected := map[string]any{
			"apiToken": "{{secret .}}",
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
	})
}

func TestMaskResolveAndDetect(t *testing.T) {
	t.Parallel()

	t.Run("masks_resolves_and_detects_candidates", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"name":                               "acme",
			"apiToken":                           "token-value",
			"password":                           "pass-value",
			"actionTokenGeneratedByUserLifespan": "300",
			"notSecret":                          "{{secret \"already-masked\"}}",
		}

		stored := map[string]string{
			"already-masked": "already-value",
		}
		masked, err := MaskPayload(input, func(key string, value string) error {
			stored[key] = value
			return nil
		})
		if err != nil {
			t.Fatalf("MaskPayload returned error: %v", err)
		}

		expectedMasked := map[string]any{
			"name":                               "acme",
			"apiToken":                           "{{secret .}}",
			"password":                           "{{secret .}}",
			"actionTokenGeneratedByUserLifespan": "300",
			"notSecret":                          "{{secret \"already-masked\"}}",
		}
		if !reflect.DeepEqual(masked, expectedMasked) {
			t.Fatalf("expected masked %#v, got %#v", expectedMasked, masked)
		}

		if !reflect.DeepEqual(stored, map[string]string{
			"already-masked": "already-value",
			"apiToken":       "token-value",
			"password":       "pass-value",
		}) {
			t.Fatalf("unexpected stored secrets: %#v", stored)
		}

		candidates, err := DetectSecretCandidates(input)
		if err != nil {
			t.Fatalf("DetectSecretCandidates returned error: %v", err)
		}
		expectedCandidates := []string{"apiToken", "password"}
		if !reflect.DeepEqual(candidates, expectedCandidates) {
			t.Fatalf("expected candidates %#v, got %#v", expectedCandidates, candidates)
		}

		resolved, err := ResolvePayload(masked, func(key string) (string, error) {
			value, ok := stored[key]
			if !ok {
				return "", faults.NewTypedError(faults.NotFoundError, "secret key not found", nil)
			}
			return value, nil
		})
		if err != nil {
			t.Fatalf("ResolvePayload returned error: %v", err)
		}

		expectedResolved := map[string]any{
			"name":                               "acme",
			"apiToken":                           "token-value",
			"password":                           "pass-value",
			"actionTokenGeneratedByUserLifespan": "300",
			"notSecret":                          "already-value",
		}
		if !reflect.DeepEqual(resolved, expectedResolved) {
			t.Fatalf("expected resolved %#v, got %#v", expectedResolved, resolved)
		}
	})

	t.Run("rejects_ambiguous_key_scope_for_masking", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"source-a": map[string]any{"token": "a"},
			"source-b": map[string]any{"token": "b"},
		}

		_, err := MaskPayload(input, func(string, string) error { return nil })
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("propagates_missing_key_when_resolving", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{"apiToken": "{{secret \"apiToken\"}}"}
		_, err := ResolvePayload(input, func(string) (string, error) {
			return "", faults.NewTypedError(faults.NotFoundError, "missing", nil)
		})
		assertTypedCategory(t, err, faults.NotFoundError)
	})

	t.Run("supports_nested_attribute_paths_for_dot_placeholders", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"credentials": map[string]any{
				"clientSecret": "super-secret",
			},
		}

		stored := map[string]string{}
		masked, err := MaskPayload(input, func(key string, value string) error {
			stored[key] = value
			return nil
		})
		if err != nil {
			t.Fatalf("MaskPayload returned error: %v", err)
		}

		expectedMasked := map[string]any{
			"credentials": map[string]any{
				"clientSecret": "{{secret .}}",
			},
		}
		if !reflect.DeepEqual(masked, expectedMasked) {
			t.Fatalf("expected masked %#v, got %#v", expectedMasked, masked)
		}
		if got := stored["credentials.clientSecret"]; got != "super-secret" {
			t.Fatalf("expected stored nested key credentials.clientSecret, got %#v", stored)
		}

		resolved, err := ResolvePayload(masked, func(key string) (string, error) {
			value, found := stored[key]
			if !found {
				return "", faults.NewTypedError(faults.NotFoundError, "missing", nil)
			}
			return value, nil
		})
		if err != nil {
			t.Fatalf("ResolvePayload returned error: %v", err)
		}
		if !reflect.DeepEqual(resolved, input) {
			t.Fatalf("expected resolved %#v, got %#v", input, resolved)
		}
	})
}

func TestResolvePayloadForResource(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"apiToken": "{{secret .}}",
		"credentials": map[string]any{
			"authValue": "{{secret custom-auth}}",
			"legacy":    "{{secret \"/customers/acme:legacy\"}}",
		},
	}

	resolved, err := ResolvePayloadForResource(input, "/customers/acme", func(key string) (string, error) {
		switch key {
		case "/customers/acme:apiToken":
			return "api-token-value", nil
		case "/customers/acme:custom-auth":
			return "custom-auth-value", nil
		case "/customers/acme:legacy":
			return "legacy-value", nil
		default:
			return "", faults.NewTypedError(faults.NotFoundError, "missing", nil)
		}
	})
	if err != nil {
		t.Fatalf("ResolvePayloadForResource returned error: %v", err)
	}

	expected := map[string]any{
		"apiToken": "api-token-value",
		"credentials": map[string]any{
			"authValue": "custom-auth-value",
			"legacy":    "legacy-value",
		},
	}
	if !reflect.DeepEqual(resolved, expected) {
		t.Fatalf("expected %#v, got %#v", expected, resolved)
	}
}

func TestResolvePayloadDirectivesForResource(t *testing.T) {
	t.Parallel()

	t.Run("resolves_resource_format_and_secrets", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"format": "{{resource_format .}}",
			"token":  "{{secret .}}",
		}

		resolved, err := ResolvePayloadDirectivesForResource(
			input,
			"/customers/acme",
			"yaml",
			func(key string) (string, error) {
				if key != "/customers/acme:token" {
					return "", faults.NewTypedError(faults.NotFoundError, "missing", nil)
				}
				return "secret-value", nil
			},
		)
		if err != nil {
			t.Fatalf("ResolvePayloadDirectivesForResource returned error: %v", err)
		}

		expected := map[string]any{
			"format": "yaml",
			"token":  "secret-value",
		}
		if !reflect.DeepEqual(resolved, expected) {
			t.Fatalf("expected %#v, got %#v", expected, resolved)
		}
	})

	t.Run("resolves_resource_format_without_secret_getter", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"format":  "{{resource_format .}}",
			"token":   "{{secret .}}",
			"literal": "prefix {{resource_format .}}",
		}

		resolved, err := ResolvePayloadDirectivesForResource(input, "/customers/acme", "", nil)
		if err != nil {
			t.Fatalf("ResolvePayloadDirectivesForResource returned error: %v", err)
		}

		expected := map[string]any{
			"format":  "json",
			"token":   "{{secret .}}",
			"literal": "prefix {{resource_format .}}",
		}
		if !reflect.DeepEqual(resolved, expected) {
			t.Fatalf("expected %#v, got %#v", expected, resolved)
		}
	})

	t.Run("rejects_invalid_resource_format_placeholder_arguments", func(t *testing.T) {
		t.Parallel()

		_, err := ResolvePayloadDirectivesForResource(
			map[string]any{"format": "{{resource_format}}"},
			"/customers/acme",
			"json",
			nil,
		)
		assertTypedCategory(t, err, faults.ValidationError)

		_, err = ResolvePayloadDirectivesForResource(
			map[string]any{"format": "{{resource_format \"yaml\"}}"},
			"/customers/acme",
			"json",
			nil,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestSplitIdentifierTokens(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"apiToken":      {"api", "token"},
		"client_secret": {"client", "secret"},
		"private-key":   {"private", "key"},
		"APIKey":        {"apikey"},
	}

	for input, expected := range cases {
		input := input
		expected := expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got := splitIdentifierTokens(input)
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("expected %#v, got %#v", expected, got)
			}
		})
	}
}

func TestIsLikelySecretKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		key      string
		expected bool
	}{
		{name: "plain password key", key: "password", expected: true},
		{name: "plain token key", key: "token", expected: true},
		{name: "api token suffix", key: "apiToken", expected: true},
		{name: "client secret suffix", key: "clientSecret", expected: true},
		{name: "token value suffix", key: "tokenValue", expected: true},
		{name: "ciba delivery mode", key: "cibaBackchannelTokenDeliveryMode", expected: false},
		{name: "webauthn passwordless preference", key: "webAuthnPolicyPasswordlessAttestationConveyancePreference", expected: false},
		{name: "token endpoint", key: "tokenEndpoint", expected: false},
		{name: "password policy", key: "passwordPolicy", expected: false},
		{name: "refresh token expiry", key: "refreshTokenExpiresIn", expected: false},
		{name: "action token lifespan", key: "actionTokenGeneratedByUserLifespan", expected: false},
		{name: "access token claim flag", key: "access.token.claim", expected: false},
		{name: "access token header type", key: "access.token.header.type.rfc9068", expected: false},
		{name: "client secret creation time", key: "client.secret.creation.time", expected: false},
		{name: "client credentials use refresh token", key: "client_credentials.use_refresh_token", expected: false},
		{name: "id token claim", key: "id.token.claim", expected: false},
		{name: "introspection token claim", key: "introspection.token.claim", expected: false},
		{name: "standard token exchange refresh requested type", key: "standard.token.exchange.enableRefreshRequestedTokenType", expected: false},
		{name: "standard token exchange enabled", key: "standard.token.exchange.enabled", expected: false},
		{name: "token response type bearer lower case", key: "token.response.type.bearer.lower-case", expected: false},
		{name: "userinfo token claim", key: "userinfo.token.claim", expected: false},
		{name: "private key pem", key: "privateKeyPem", expected: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isLikelySecretKey(tc.key); got != tc.expected {
				t.Fatalf("expected %v for %q, got %v", tc.expected, tc.key, got)
			}
		})
	}
}

func TestIsLikelySecretValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "secret text", value: "abc123", expected: true},
		{name: "numeric policy value", value: "43200", expected: false},
		{name: "boolean true", value: "true", expected: false},
		{name: "boolean false", value: "false", expected: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isLikelySecretValue(tc.value); got != tc.expected {
				t.Fatalf("expected %v for %q, got %v", tc.expected, tc.value, got)
			}
		})
	}
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}

func TestHelpersRejectNilCallbacks(t *testing.T) {
	t.Parallel()

	_, err := MaskPayload(map[string]any{"token": "x"}, nil)
	assertTypedCategory(t, err, faults.ValidationError)

	_, err = ResolvePayload(map[string]any{"token": "{{secret \"token\"}}"}, nil)
	assertTypedCategory(t, err, faults.ValidationError)

	_, err = NormalizePlaceholders(context.Background())
	assertTypedCategory(t, err, faults.ValidationError)
}
