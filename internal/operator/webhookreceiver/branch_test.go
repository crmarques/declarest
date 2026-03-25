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

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

func TestRefToBranch(t *testing.T) {
	tests := []struct {
		ref    string
		branch string
	}{
		{"refs/heads/main", "main"},
		{"refs/heads/release/v1", "release/v1"},
		{"main", "main"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := RefToBranch(tt.ref); got != tt.branch {
			t.Errorf("RefToBranch(%q) = %q, want %q", tt.ref, got, tt.branch)
		}
	}
}

func TestMatchBranchFilter(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		filter *declarestv1alpha1.RepositoryWebhookBranchFilter
		want   bool
	}{
		{"nil filter matches all", "main", nil, true},
		{"empty filter matches all", "main", &declarestv1alpha1.RepositoryWebhookBranchFilter{}, true},
		{"include matches", "main", &declarestv1alpha1.RepositoryWebhookBranchFilter{Include: []string{"main"}}, true},
		{"include no match", "develop", &declarestv1alpha1.RepositoryWebhookBranchFilter{Include: []string{"main"}}, false},
		{"include glob", "release/v1", &declarestv1alpha1.RepositoryWebhookBranchFilter{Include: []string{"release/*"}}, true},
		{"exclude overrides", "main", &declarestv1alpha1.RepositoryWebhookBranchFilter{Exclude: []string{"main"}}, false},
		{"exclude with include", "develop", &declarestv1alpha1.RepositoryWebhookBranchFilter{
			Include: []string{"main", "develop"},
			Exclude: []string{"develop"},
		}, false},
		{"not in exclude passes", "feature/x", &declarestv1alpha1.RepositoryWebhookBranchFilter{
			Exclude: []string{"main"},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchBranchFilter(tt.branch, tt.filter); got != tt.want {
				t.Errorf("MatchBranchFilter(%q, ...) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}
