package stub

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestStubMetadataServiceReturnsTypedNotImplemented(t *testing.T) {
	t.Parallel()

	service := &StubMetadataService{}
	_, err := service.Get(context.Background(), "/path")
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
	if !strings.Contains(err.Error(), "StubMetadataService.Get not implemented") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
