package vault

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestVaultSecretServiceKV2TokenAuth(t *testing.T) {
	t.Parallel()

	fake := newFakeVault()
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	service, err := NewVaultSecretService(config.VaultSecretStore{
		Address:   server.URL,
		KVVersion: 2,
		Auth:      &config.VaultAuth{Token: fake.clientToken},
	})
	if err != nil {
		t.Fatalf("NewVaultSecretService returned error: %v", err)
	}

	if err := service.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if err := service.Store(context.Background(), "apiToken", "token-value"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	value, err := service.Get(context.Background(), "apiToken")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "token-value" {
		t.Fatalf("expected token-value, got %q", value)
	}

	keys, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !reflect.DeepEqual(keys, []string{"apiToken"}) {
		t.Fatalf("expected [apiToken], got %#v", keys)
	}

	masked, err := service.MaskPayload(context.Background(), map[string]any{
		"apiToken": "masked-token",
	})
	if err != nil {
		t.Fatalf("MaskPayload returned error: %v", err)
	}
	expectedMasked := map[string]any{"apiToken": "{{secret .}}"}
	if !reflect.DeepEqual(masked, expectedMasked) {
		t.Fatalf("expected masked %#v, got %#v", expectedMasked, masked)
	}

	resolved, err := service.ResolvePayload(context.Background(), masked)
	if err != nil {
		t.Fatalf("ResolvePayload returned error: %v", err)
	}
	expectedResolved := map[string]any{"apiToken": "masked-token"}
	if !reflect.DeepEqual(resolved, expectedResolved) {
		t.Fatalf("expected resolved %#v, got %#v", expectedResolved, resolved)
	}

	if err := service.Delete(context.Background(), "apiToken"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	_, err = service.Get(context.Background(), "apiToken")
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestVaultSecretServiceUserPassAuth(t *testing.T) {
	t.Parallel()

	fake := newFakeVault()
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	service, err := NewVaultSecretService(config.VaultSecretStore{
		Address:   server.URL,
		KVVersion: 2,
		Auth: &config.VaultAuth{
			Password: &config.VaultUserPasswordAuth{
				Username: fake.userName,
				Password: fake.password,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewVaultSecretService returned error: %v", err)
	}

	if err := service.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if err := service.Store(context.Background(), "password", "p4ss"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}
	value, err := service.Get(context.Background(), "password")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "p4ss" {
		t.Fatalf("expected p4ss, got %q", value)
	}
}

func TestVaultSecretServiceValidationAndAuth(t *testing.T) {
	t.Parallel()

	_, err := NewVaultSecretService(config.VaultSecretStore{
		Address: "bad-url",
		Auth:    &config.VaultAuth{Token: "token"},
	})
	assertTypedCategory(t, err, faults.ValidationError)

	fake := newFakeVault()
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	service, err := NewVaultSecretService(config.VaultSecretStore{
		Address: server.URL,
		Auth:    &config.VaultAuth{Token: "wrong-token"},
	})
	if err != nil {
		t.Fatalf("NewVaultSecretService returned error: %v", err)
	}

	err = service.Store(context.Background(), "apiToken", "x")
	assertTypedCategory(t, err, faults.AuthError)
}

func TestVaultSecretServiceRejectsOversizedResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Exceeds maxVaultResponseBytes and should fail before JSON decode.
		_, _ = w.Write([]byte(strings.Repeat("x", maxVaultResponseBytes+1)))
	}))
	defer server.Close()

	service, err := NewVaultSecretService(config.VaultSecretStore{
		Address: server.URL,
		Auth:    &config.VaultAuth{Token: "token"},
	})
	if err != nil {
		t.Fatalf("NewVaultSecretService returned error: %v", err)
	}

	_, err = service.List(context.Background())
	assertTypedCategory(t, err, faults.TransportError)
	if !strings.Contains(err.Error(), "failed to read vault response body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeVaultServer struct {
	mu sync.Mutex

	clientToken string
	userName    string
	password    string

	secrets map[string]string
}

func newFakeVault() *fakeVaultServer {
	return &fakeVaultServer{
		clientToken: "root-token",
		userName:    "tester",
		password:    "change-me",
		secrets:     make(map[string]string),
	}
}

func (f *fakeVaultServer) handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.Trim(request.URL.Path, "/")
		if path == "v1/auth/userpass/login/"+f.userName {
			f.handleUserPassLogin(writer, request)
			return
		}

		if !f.authorized(request) {
			writeJSON(writer, http.StatusForbidden, map[string]any{
				"errors": []string{"permission denied"},
			})
			return
		}

		if path == "v1/secret/metadata" {
			f.handleList(writer, request)
			return
		}

		if strings.HasPrefix(path, "v1/secret/data/") {
			key := strings.TrimPrefix(path, "v1/secret/data/")
			f.handleSecret(writer, request, key)
			return
		}

		writeJSON(writer, http.StatusNotFound, map[string]any{
			"errors": []string{"not found"},
		})
	})
}

func (f *fakeVaultServer) authorized(request *http.Request) bool {
	return request.Header.Get("X-Vault-Token") == f.clientToken
}

func (f *fakeVaultServer) handleUserPassLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{
			"errors": []string{"method not allowed"},
		})
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{
			"errors": []string{"invalid payload"},
		})
		return
	}
	if payload["password"] != f.password {
		writeJSON(writer, http.StatusForbidden, map[string]any{
			"errors": []string{"invalid credentials"},
		})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"auth": map[string]any{
			"client_token": f.clientToken,
		},
	})
}

func (f *fakeVaultServer) handleSecret(writer http.ResponseWriter, request *http.Request, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch request.Method {
	case http.MethodPost:
		var payload struct {
			Data map[string]string `json:"data"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]any{
				"errors": []string{"invalid payload"},
			})
			return
		}
		value, found := payload.Data["value"]
		if !found {
			writeJSON(writer, http.StatusBadRequest, map[string]any{
				"errors": []string{"missing value"},
			})
			return
		}
		f.secrets[key] = value
		writeJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{}})
	case http.MethodGet:
		value, found := f.secrets[key]
		if !found {
			writeJSON(writer, http.StatusNotFound, map[string]any{
				"errors": []string{"not found"},
			})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{
			"data": map[string]any{
				"data": map[string]any{
					"value": value,
				},
			},
		})
	case http.MethodDelete:
		delete(f.secrets, key)
		writer.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{
			"errors": []string{"method not allowed"},
		})
	}
}

func (f *fakeVaultServer) handleList(writer http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if request.Method != "LIST" && !(request.Method == http.MethodGet && request.URL.Query().Get("list") == "true") {
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{
			"errors": []string{"method not allowed"},
		})
		return
	}

	keys := make([]string, 0, len(f.secrets))
	for key := range f.secrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	anyKeys := make([]any, len(keys))
	for idx, key := range keys {
		anyKeys[idx] = key
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"data": map[string]any{
			"keys": anyKeys,
		},
	})
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
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
