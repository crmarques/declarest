package resource

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	t.Run("normalizes_nested_payload", func(t *testing.T) {
		t.Parallel()

		input := map[string]any{
			"id":     json.Number("42"),
			"active": true,
			"limits": []any{
				uint16(3),
				json.Number("1.5"),
			},
			"profile": map[string]any{
				"age": int8(9),
			},
		}

		got, err := Normalize(input)
		if err != nil {
			t.Fatalf("Normalize returned error: %v", err)
		}

		expected := map[string]any{
			"id":     int64(42),
			"active": true,
			"limits": []any{
				int64(3),
				float64(1.5),
			},
			"profile": map[string]any{
				"age": int64(9),
			},
		}

		if !deepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
	})

	t.Run("rejects_non_string_map_keys", func(t *testing.T) {
		t.Parallel()

		_, err := Normalize(map[int]string{1: "x"})
		assertValidationErrorNormalize(t, err)
	})

	t.Run("rejects_non_finite_float", func(t *testing.T) {
		t.Parallel()

		_, err := Normalize(math.Inf(1))
		assertValidationErrorNormalize(t, err)
	})

	t.Run("rejects_out_of_range_integer", func(t *testing.T) {
		t.Parallel()

		_, err := Normalize(uint64(math.MaxInt64) + 1)
		assertValidationErrorNormalize(t, err)
	})

	t.Run("rejects_unsupported_type", func(t *testing.T) {
		t.Parallel()

		type payload struct {
			ID string
		}
		_, err := Normalize(payload{ID: "x"})
		assertValidationErrorNormalize(t, err)
	})
}

func deepEqual(a any, b any) bool {
	encodedA, err := json.Marshal(a)
	if err != nil {
		return false
	}
	encodedB, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(encodedA) == string(encodedB)
}

func assertValidationErrorNormalize(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != faults.ValidationError {
		t.Fatalf("expected %q category, got %q", faults.ValidationError, typed.Category)
	}
}
