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
