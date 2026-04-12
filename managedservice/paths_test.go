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

package managedservice

import "testing"

func TestNormalizeRequestPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "whitespace", input: "   ", want: ""},
		{name: "adds leading slash", input: "api/test", want: "/api/test"},
		{name: "trims whitespace", input: " /api/test/ ", want: "/api/test"},
		{name: "root remains root", input: "/", want: "/"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeRequestPath(test.input); got != test.want {
				t.Fatalf("NormalizeRequestPath(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
