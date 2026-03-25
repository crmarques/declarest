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

package webhookreceiver

import (
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		path      string
		namespace string
		name      string
		wantErr   bool
	}{
		{"/hooks/v1/repositorywebhooks/default/my-webhook", "default", "my-webhook", false},
		{"/hooks/v1/repositorywebhooks/ns/name/", "ns", "name", false},
		{"/hooks/v1/repositorywebhooks/", "", "", true},
		{"/hooks/v1/repositorywebhooks/only-name", "", "", true},
		{"/hooks/v1/repositorywebhooks/a/b/c", "", "", true},
		{"/wrong/path", "", "", true},
	}
	for _, tt := range tests {
		ns, name, err := parsePath(tt.path)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePath(%q) expected error", tt.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePath(%q) unexpected error: %v", tt.path, err)
			continue
		}
		if ns != tt.namespace || name != tt.name {
			t.Errorf("parsePath(%q) = (%q, %q), want (%q, %q)", tt.path, ns, name, tt.namespace, tt.name)
		}
	}
}
