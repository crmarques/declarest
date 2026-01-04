package secrets

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type vaultTestStore struct {
	mu   sync.Mutex
	data map[string]map[string]string
}

func TestVaultSecretsManagerTokenRoundTripKV2(t *testing.T) {
	store := &vaultTestStore{
		data: map[string]map[string]string{},
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listening not permitted: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/secret/data/"):
			path := strings.TrimPrefix(r.URL.Path, "/v1/secret/data/")
			switch r.Method {
			case http.MethodGet:
				store.mu.Lock()
				value, ok := store.data[path]
				store.mu.Unlock()
				if !ok {
					http.NotFound(w, r)
					return
				}
				resp := map[string]any{"data": map[string]any{"data": value}}
				_ = json.NewEncoder(w).Encode(resp)
				return
			case http.MethodPost:
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				var payload struct {
					Data map[string]string `json:"data"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				store.mu.Lock()
				store.data[path] = payload.Data
				store.mu.Unlock()
				w.WriteHeader(http.StatusOK)
				return
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
		case strings.HasPrefix(r.URL.Path, "/v1/secret/metadata/"):
			path := strings.TrimPrefix(r.URL.Path, "/v1/secret/metadata/")
			if r.Method != http.MethodDelete {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			store.mu.Lock()
			delete(store.data, path)
			store.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	manager := NewVaultSecretsManager(&VaultSecretsManagerConfig{
		Address:   server.URL,
		Mount:     "secret",
		KVVersion: 2,
		Auth: &VaultSecretsManagerAuthConfig{
			Token: "test-token",
		},
	})

	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := manager.UpdateSecret("/apps/app1", "password", "s3cr3t"); err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	value, err := manager.GetSecret("/apps/app1", "password")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if value != "s3cr3t" {
		t.Fatalf("expected secret value, got %q", value)
	}
	if err := manager.DeleteSecret("/apps/app1", "password", ""); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	_, err = manager.GetSecret("/apps/app1", "password")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}
