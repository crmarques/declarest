package cmd

import (
	"errors"
	"strings"
	"testing"

	"declarest/internal/secrets"
)

func TestWrapSecretStoreError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantContains string
	}{
		{
			name:         "nil error",
			err:          nil,
			wantContains: "",
		},
		{
			name:         "not initialized",
			err:          secrets.ErrSecretStoreNotInitialized,
			wantContains: "declarest secret init",
		},
		{
			name:         "not configured",
			err:          secrets.ErrSecretStoreNotConfigured,
			wantContains: "secret_store section",
		},
		{
			name:         "other error",
			err:          errors.New("other"),
			wantContains: "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapSecretStoreError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(got.Error(), tt.wantContains) {
				t.Fatalf("error %q does not contain %q", got.Error(), tt.wantContains)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("wrapped error does not wrap original: %v", got)
			}
		})
	}
}
