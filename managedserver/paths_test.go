package managedserver

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
