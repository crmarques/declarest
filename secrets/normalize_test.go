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

package secrets

import "testing"

func TestNormalizeKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "trims slash", input: "/path/to/key/", want: "path/to/key"},
		{name: "trims whitespace", input: "  path/to/key  ", want: "path/to/key"},
		{name: "empty rejected", input: " ", wantErr: true},
		{name: "dot rejected", input: "path/./key", wantErr: true},
		{name: "dotdot rejected", input: "path/../key", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeKey(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeKey(%q) expected error", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeKey(%q) returned error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("NormalizeKey(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
