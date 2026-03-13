package resource

import (
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestMergeWithDefaults(t *testing.T) {
	t.Parallel()

	merged, err := MergeWithDefaults(
		map[string]any{
			"id": "generated",
			"spec": map[string]any{
				"enabled": true,
				"tags":    []any{"default"},
				"nested": map[string]any{
					"region": "us-east-1",
				},
			},
		},
		map[string]any{
			"spec": map[string]any{
				"enabled": false,
				"tags":    []any{"override"},
				"nested": map[string]any{
					"tier": "gold",
				},
			},
			"notes": nil,
		},
	)
	if err != nil {
		t.Fatalf("MergeWithDefaults() error = %v", err)
	}

	want := map[string]any{
		"id": "generated",
		"spec": map[string]any{
			"enabled": false,
			"tags":    []any{"override"},
			"nested": map[string]any{
				"region": "us-east-1",
				"tier":   "gold",
			},
		},
		"notes": nil,
	}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("unexpected merged payload: got %#v want %#v", merged, want)
	}
}

func TestMergeWithDefaultsRejectsNonObjectShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		defaults  any
		overrides any
	}{
		{
			name:      "array defaults",
			defaults:  []any{"x"},
			overrides: map[string]any{"a": "b"},
		},
		{
			name:      "scalar overrides",
			defaults:  map[string]any{"a": "b"},
			overrides: "value",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := MergeWithDefaults(tc.defaults, tc.overrides)
			if !faults.IsCategory(err, faults.ValidationError) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestCompactAgainstDefaults(t *testing.T) {
	t.Parallel()

	compacted, err := CompactAgainstDefaults(
		map[string]any{
			"id": "generated",
			"spec": map[string]any{
				"enabled": false,
				"tags":    []any{"override"},
				"nested": map[string]any{
					"region": "us-east-1",
					"tier":   "gold",
				},
			},
			"notes": nil,
		},
		map[string]any{
			"id": "generated",
			"spec": map[string]any{
				"enabled": true,
				"tags":    []any{"default"},
				"nested": map[string]any{
					"region": "us-east-1",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("CompactAgainstDefaults() error = %v", err)
	}

	want := map[string]any{
		"spec": map[string]any{
			"enabled": false,
			"tags":    []any{"override"},
			"nested": map[string]any{
				"tier": "gold",
			},
		},
		"notes": nil,
	}
	if !reflect.DeepEqual(compacted, want) {
		t.Fatalf("unexpected compacted payload: got %#v want %#v", compacted, want)
	}
}

func TestCompactAgainstDefaultsReturnsNilWhenPayloadMatchesDefaults(t *testing.T) {
	t.Parallel()

	compacted, err := CompactAgainstDefaults(
		map[string]any{
			"spec": map[string]any{
				"enabled": true,
			},
		},
		map[string]any{
			"spec": map[string]any{
				"enabled": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("CompactAgainstDefaults() error = %v", err)
	}
	if compacted != nil {
		t.Fatalf("expected nil compacted payload, got %#v", compacted)
	}
}

func TestValidateDefaultsSidecarDescriptor(t *testing.T) {
	t.Parallel()

	if err := ValidateDefaultsSidecarDescriptor(
		PayloadDescriptor{Extension: ".yaml"},
		PayloadDescriptor{Extension: ".yml"},
	); err != nil {
		t.Fatalf("expected yaml descriptors to be compatible, got %v", err)
	}

	if err := ValidateDefaultsSidecarDescriptor(
		PayloadDescriptor{Extension: ".properties"},
		PayloadDescriptor{Extension: ".properties"},
	); err != nil {
		t.Fatalf("expected properties descriptors to be compatible, got %v", err)
	}

	err := ValidateDefaultsSidecarDescriptor(
		PayloadDescriptor{Extension: ".txt"},
		PayloadDescriptor{Extension: ".txt"},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error for text defaults sidecar, got %v", err)
	}

	err = ValidateDefaultsSidecarDescriptor(
		PayloadDescriptor{Extension: ".yaml"},
		PayloadDescriptor{Extension: ".properties"},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error for mismatched payload types, got %v", err)
	}
}

func TestValidateDefaultsSidecarValue(t *testing.T) {
	t.Parallel()

	if err := ValidateDefaultsSidecarValue(map[string]any{"spec": map[string]any{"enabled": true}}); err != nil {
		t.Fatalf("expected object defaults sidecar to validate, got %v", err)
	}

	err := ValidateDefaultsSidecarValue(nil)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error for nil defaults sidecar, got %v", err)
	}
}

func TestInferDefaultsFromValues(t *testing.T) {
	t.Parallel()

	inferred, err := InferDefaultsFromValues(
		map[string]any{
			"name": "acme",
			"labels": map[string]any{
				"team":   "platform",
				"region": "us-east-1",
			},
			"enabled": true,
		},
		map[string]any{
			"name": "beta",
			"labels": map[string]any{
				"team":   "platform",
				"region": "eu-west-1",
			},
			"enabled": false,
		},
	)
	if err != nil {
		t.Fatalf("InferDefaultsFromValues() error = %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
	}
	if !reflect.DeepEqual(inferred, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", inferred, want)
	}
}

func TestInferDefaultsFromValuesKeepsSharedEmptyObjects(t *testing.T) {
	t.Parallel()

	inferred, err := InferDefaultsFromValues(
		map[string]any{
			"name":       "acme",
			"smtpServer": map[string]any{},
		},
		map[string]any{
			"name":       "beta",
			"smtpServer": map[string]any{},
		},
	)
	if err != nil {
		t.Fatalf("InferDefaultsFromValues() error = %v", err)
	}

	want := map[string]any{
		"smtpServer": map[string]any{},
	}
	if !reflect.DeepEqual(inferred, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", inferred, want)
	}
}

func TestInferCreatedDefaultsSubtractsExplicitInputs(t *testing.T) {
	t.Parallel()

	inferred, err := InferCreatedDefaults(
		[]any{
			map[string]any{"name": "probe-1", "displayName": "Customer", "enabled": true},
			map[string]any{"name": "probe-2", "displayName": "Customer", "enabled": true},
		},
		[]any{
			map[string]any{"name": "probe-1", "displayName": "Customer", "enabled": true, "status": "active"},
			map[string]any{"name": "probe-2", "displayName": "Customer", "enabled": true, "status": "active"},
		},
	)
	if err != nil {
		t.Fatalf("InferCreatedDefaults() error = %v", err)
	}

	want := map[string]any{"status": "active"}
	if !reflect.DeepEqual(inferred, want) {
		t.Fatalf("unexpected inferred created defaults: got %#v want %#v", inferred, want)
	}
}
