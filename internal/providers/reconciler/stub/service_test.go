package stub

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestStubResourceReconcilerReturnsTypedNotImplemented(t *testing.T) {
	t.Parallel()

	reconciler := &StubResourceReconciler{}
	err := reconciler.Save(context.Background(), "/path", map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected error")
	}

	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != faults.InternalError {
		t.Fatalf("expected %q category, got %q", faults.InternalError, typed.Category)
	}
	if !strings.Contains(err.Error(), "StubResourceReconciler.Save not implemented") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
