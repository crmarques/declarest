package notimpl

import (
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestErrorReturnsTypedInternalError(t *testing.T) {
	t.Parallel()

	err := Error("TypeName", "MethodName")
	if err == nil {
		t.Fatal("expected error")
	}

	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != faults.InternalError {
		t.Fatalf("expected category %q, got %q", faults.InternalError, typed.Category)
	}
	if typed.Message != "TypeName.MethodName not implemented" {
		t.Fatalf("unexpected message %q", typed.Message)
	}
	if !errors.Is(err, faults.ErrToBeImplemented) {
		t.Fatalf("expected wrapped ErrToBeImplemented, got %v", err)
	}
}
