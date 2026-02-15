package core

import (
	"testing"

	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	defaultreconciler "github.com/crmarques/declarest/internal/providers/reconciler/default"
)

func TestNewAppState(t *testing.T) {
	t.Parallel()

	appState := NewAppState(BootstrapConfig{})
	if appState.Contexts == nil {
		t.Fatal("expected non-nil contexts service")
	}
	if appState.Reconciler == nil {
		t.Fatal("expected non-nil resource reconciler")
	}

	if _, ok := appState.Contexts.(*configfile.FileContextService); !ok {
		t.Fatalf("expected FileContextService, got %T", appState.Contexts)
	}
	if _, ok := appState.Reconciler.(*defaultreconciler.DefaultResourceReconciler); !ok {
		t.Fatalf("expected DefaultResourceReconciler, got %T", appState.Reconciler)
	}
}
