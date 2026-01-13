package cmd

import "testing"

func TestSplitCommaCompletionParts(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantBase  string
		wantToken string
	}{
		{name: "empty", value: "", wantBase: "", wantToken: ""},
		{name: "single", value: "token", wantBase: "", wantToken: "token"},
		{name: "single with spaces", value: "  token  ", wantBase: "", wantToken: "token"},
		{name: "simple comma", value: "foo,bar", wantBase: "foo,", wantToken: "bar"},
		{name: "comma with space", value: "foo, bar", wantBase: "foo, ", wantToken: "bar"},
		{name: "comma with trailing space", value: "foo,   ", wantBase: "foo,   ", wantToken: ""},
		{name: "multiple entries", value: "a,b,c", wantBase: "a,b,", wantToken: "c"},
		{name: "multiple entries with spaces", value: "a, b,   ", wantBase: "a, b,   ", wantToken: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, token := splitCommaCompletionParts(tc.value)
			if base != tc.wantBase {
				t.Fatalf("base = %q, want %q", base, tc.wantBase)
			}
			if token != tc.wantToken {
				t.Fatalf("token = %q, want %q", token, tc.wantToken)
			}
		})
	}
}
