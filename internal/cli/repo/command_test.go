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

package repo

import "testing"

func TestRenderRepoTreeText(t *testing.T) {
	t.Parallel()

	got := renderRepoTreeText([]string{
		"admin/realms/acme/clients/test",
		"admin/realms",
		"admin/realms/acme",
		"admin",
		"admin/realms/acme/clients",
		"admin/realms/acme/user-registry/AD PRD",
		"admin/realms/acme/user-registry",
		"",
		" ",
		"./invalid",
		"../invalid",
	})

	want := "admin\n" +
		"└── realms\n" +
		"    └── acme\n" +
		"        ├── clients\n" +
		"        │   └── test\n" +
		"        └── user-registry\n" +
		"            └── AD PRD"
	if got != want {
		t.Fatalf("renderRepoTreeText() = %q, want %q", got, want)
	}
}
