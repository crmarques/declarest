// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cliutil

import (
	"reflect"
	"testing"
)

func TestParseDotNotationSimpleKey(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("name=test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{"name": "test"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationNestedKey(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("metadata.name=test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{"metadata": map[string]any{"name": "test"}}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationDeepNested(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("a.b.c.d=value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "value",
				},
			},
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationQuotedKeySegment(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject(`testA."testB.testC"=bla`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{
		"testA": map[string]any{
			"testB.testC": "bla",
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationMultipleAssignments(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("name=test,metadata.labels.env=prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{
		"name": "test",
		"metadata": map[string]any{
			"labels": map[string]any{
				"env": "prod",
			},
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationMultipleNestedSameParent(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("spec.name=test,spec.tier=gold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{
		"spec": map[string]any{
			"name": "test",
			"tier": "gold",
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseDotNotationRejectsEmptyKey(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("=value")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestParseDotNotationRejectsTrailingDot(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("a.=value")
	if err == nil {
		t.Fatal("expected error for trailing dot")
	}
}

func TestParseDotNotationRejectsUnclosedQuote(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject(`a."unclosed=value`)
	if err == nil {
		t.Fatal("expected error for unclosed quote")
	}
}

func TestParseDotNotationRejectsConflictingScalar(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("a=x,a.b=y")
	if err == nil {
		t.Fatal("expected error for conflicting key")
	}
}

func TestParseDotNotationRejectsEmptySegment(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("a..b=value")
	if err == nil {
		t.Fatal("expected error for empty key segment")
	}
}

func TestParseDotNotationRejectsMissingEquals(t *testing.T) {
	t.Parallel()

	_, err := ParseDotNotationAssignmentsObject("noequals")
	if err == nil {
		t.Fatal("expected error for missing equals")
	}
}

func TestIsDotNotationAssignmentPositive(t *testing.T) {
	t.Parallel()

	cases := []string{
		"name=test",
		"a.b=value",
		`a."b.c"=value`,
		"a=x,b=y",
	}
	for _, input := range cases {
		if !IsDotNotationAssignment(input) {
			t.Errorf("expected IsDotNotationAssignment(%q) = true", input)
		}
	}
}

func TestIsDotNotationAssignmentNegative(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"/a=b",
		`{"key":"value"}`,
		`[1,2,3]`,
		"noequals",
	}
	for _, input := range cases {
		if IsDotNotationAssignment(input) {
			t.Errorf("expected IsDotNotationAssignment(%q) = false", input)
		}
	}
}

func TestParseDotNotationValueWithEquals(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("key=value=with=equals")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{"key": "value=with=equals"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationEmptyValue(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject("key=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{"key": ""}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestParseDotNotationCommaInsideQuotedKey(t *testing.T) {
	t.Parallel()

	result, err := ParseDotNotationAssignmentsObject(`"a,b"=value`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]any{"a,b": "value"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}
