package faults

import (
	"errors"
	"testing"
)

func TestIsCategory(t *testing.T) {
	t.Parallel()

	err := NewTypedError(ValidationError, "invalid input", nil)
	if !IsCategory(err, ValidationError) {
		t.Fatalf("expected validation category match")
	}
	if IsCategory(err, NotFoundError) {
		t.Fatalf("expected not-found category mismatch")
	}

	wrapped := errors.New("wrap: " + err.Error())
	if IsCategory(wrapped, ValidationError) {
		t.Fatalf("plain wrapped string error must not match typed category")
	}

	joined := errors.Join(err, errors.New("other"))
	if !IsCategory(joined, ValidationError) {
		t.Fatalf("expected category match through errors.Join")
	}
}
