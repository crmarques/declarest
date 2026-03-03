package controllers

import "testing"

func TestHasPathOverlap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  string
		right string
		match bool
	}{
		{name: "same path", left: "/customers/acme", right: "/customers/acme", match: true},
		{name: "parent child", left: "/customers", right: "/customers/acme", match: true},
		{name: "sibling", left: "/customers/acme", right: "/customers/beta", match: false},
		{name: "root overlap", left: "/", right: "/customers", match: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := hasPathOverlap(tc.left, tc.right); got != tc.match {
				t.Fatalf("hasPathOverlap(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.match)
			}
		})
	}
}
