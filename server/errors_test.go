package server

import (
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestListPayloadShapeError(t *testing.T) {
	t.Parallel()

	err := NewListPayloadShapeError(`list response "items" must be an array`, nil)
	if !IsListPayloadShapeError(err) {
		t.Fatalf("expected list payload shape error predicate to match")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected list payload shape error to preserve validation category")
	}
}
