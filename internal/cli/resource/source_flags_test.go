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

package resource

import (
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/spf13/cobra"
)

func TestNormalizeReadSourceSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceFlag     string
		want           string
		wantValidation bool
	}{
		{name: "default_managed_server", want: sourceManagedServer},
		{name: "new_flag_repository", sourceFlag: sourceRepository, want: sourceRepository},
		{name: "new_flag_managed_server", sourceFlag: sourceManagedServer, want: sourceManagedServer},
		{name: "rejects_both_value", sourceFlag: sourceBoth, wantValidation: true},
		{name: "rejects_invalid_value", sourceFlag: "invalid", wantValidation: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeReadSourceSelection(tt.sourceFlag)
			if tt.wantValidation {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if !faults.IsCategory(err, faults.ValidationError) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected source: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDeleteSourceSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceFlag     string
		want           string
		wantValidation bool
	}{
		{name: "default_managed_server", want: sourceManagedServer},
		{name: "new_flag_both", sourceFlag: sourceBoth, want: sourceBoth},
		{name: "rejects_invalid", sourceFlag: "invalid", wantValidation: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeDeleteSourceSelection(tt.sourceFlag)
			if tt.wantValidation {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if !faults.IsCategory(err, faults.ValidationError) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected source: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestParseExcludeFlag(t *testing.T) {
	t.Parallel()

	t.Run("unset_flag_returns_nil", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw []string
		bindExcludeFlag(command, &raw)

		got, err := parseExcludeFlag(command, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil items for unset flag, got %#v", got)
		}
	})

	t.Run("parses_trimmed_unique_values", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw []string
		bindExcludeFlag(command, &raw)
		if err := command.Flags().Set("exclude", " master, realm1 ,master "); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}

		got, err := parseExcludeFlag(command, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"master", "realm1"}
		if len(got) != len(want) {
			t.Fatalf("unexpected parsed items: got=%#v want=%#v", got, want)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("unexpected parsed item at %d: got=%q want=%q", idx, got[idx], want[idx])
			}
		}
	})

	t.Run("rejects_empty_item", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw []string
		bindExcludeFlag(command, &raw)
		if err := command.Flags().Set("exclude", "master,,realm1"); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}

		_, err := parseExcludeFlag(command, raw)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !faults.IsCategory(err, faults.ValidationError) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("supports_repeatable_flag_values", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw []string
		bindExcludeFlag(command, &raw)
		if err := command.Flags().Set("exclude", "master"); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}
		if err := command.Flags().Set("exclude", "realm1"); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}

		got, err := parseExcludeFlag(command, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"master", "realm1"}
		if len(got) != len(want) {
			t.Fatalf("unexpected parsed items: got=%#v want=%#v", got, want)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("unexpected parsed item at %d: got=%q want=%q", idx, got[idx], want[idx])
			}
		}
	})
}
