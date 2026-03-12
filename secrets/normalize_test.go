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
