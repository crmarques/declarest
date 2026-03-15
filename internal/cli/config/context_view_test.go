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

package config

import (
	"errors"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestSelectContextForViewCompactsDefaultMetadataBaseDir(t *testing.T) {
	t.Parallel()

	contexts := []configdomain.Context{
		{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
		},
	}

	selected, idx, err := selectContextForView(contexts, "dev")
	if err != nil {
		t.Fatalf("selectContextForView returned error: %v", err)
	}
	if idx != 0 {
		t.Fatalf("expected selected index 0, got %d", idx)
	}
	if selected.Metadata.BaseDir != "" {
		t.Fatalf("expected default metadata base-dir to be compacted, got %q", selected.Metadata.BaseDir)
	}
}

func TestSelectContextForViewReturnsNotFound(t *testing.T) {
	t.Parallel()

	_, _, err := selectContextForView([]configdomain.Context{{Name: "dev"}}, "prod")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != faults.NotFoundError {
		t.Fatalf("expected not found category, got %q", typedErr.Category)
	}
}

func TestCompactContextCatalogForViewCompactsEntries(t *testing.T) {
	t.Parallel()

	catalog := configdomain.ContextCatalog{
		Contexts: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
		},
		CurrentContext: "dev",
	}

	compacted := compactContextCatalogForView(catalog)
	if compacted.Contexts[0].Metadata.BaseDir != "" {
		t.Fatalf("expected compacted metadata base-dir, got %q", compacted.Contexts[0].Metadata.BaseDir)
	}
}
