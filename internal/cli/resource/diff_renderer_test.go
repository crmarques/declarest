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

package resource

import (
	"bytes"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestBuildUnifiedDiffTextNormalizesStructuredContent(t *testing.T) {
	t.Parallel()

	document := diffDocument{
		ResourcePath: "/customers/acme",
		Local: resource.Content{
			Value: map[string]any{
				"b": 2,
				"a": map[string]any{
					"d": 4,
					"c": 3,
				},
			},
			Descriptor: resource.PayloadDescriptor{PayloadType: resource.PayloadTypeYAML},
		},
		Remote: resource.Content{
			Value: map[string]any{
				"a": map[string]any{
					"c": 3,
					"d": 4,
				},
				"b": 2,
			},
			Descriptor: resource.PayloadDescriptor{PayloadType: resource.PayloadTypeYAML},
		},
	}

	diff, err := buildUnifiedDiffText(document)
	if err != nil {
		t.Fatalf("unexpected unified diff error: %v", err)
	}
	if diff != "" {
		t.Fatalf("expected no unified diff after normalization, got %q", diff)
	}
}

func TestRenderDiffReportTextOrdersSectionsAndSummarizes(t *testing.T) {
	t.Parallel()

	report, err := buildDiffReport([]diffDocument{
		{
			ResourcePath: "/customers/beta",
			Local:        resource.Content{Value: map[string]any{"name": "Beta"}},
			Remote:       resource.Content{Value: map[string]any{"name": "Beta Prime"}},
			Entries: []resource.DiffEntry{
				{ResourcePath: "/customers/beta", Path: "/name", Operation: "replace"},
			},
		},
		{
			ResourcePath: "/customers/gamma",
			Local:        resource.Content{Value: map[string]any{"name": "Gamma"}},
			Remote:       resource.Content{Value: map[string]any{"name": "Gamma"}},
		},
		{
			ResourcePath: "/customers/acme",
			Local:        resource.Content{Value: map[string]any{"name": "Acme"}},
			Remote:       resource.Content{},
			Entries: []resource.DiffEntry{
				{ResourcePath: "/customers/acme", Path: "", Operation: "replace"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected diff report error: %v", err)
	}

	var output bytes.Buffer
	if err := renderDiffReportText(&output, report, diffRenderOptions{
		RequestedPath: "/customers",
		ColorMode:     diffColorNever,
	}); err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	rendered := output.String()
	acmeIndex := strings.Index(rendered, "/customers/acme [REMOVED]")
	betaIndex := strings.Index(rendered, "/customers/beta [CHANGED]")
	if acmeIndex < 0 || betaIndex < 0 {
		t.Fatalf("expected ordered resource sections, got %q", rendered)
	}
	if acmeIndex > betaIndex {
		t.Fatalf("expected /customers/acme before /customers/beta, got %q", rendered)
	}
	if strings.Contains(rendered, "/customers/gamma") {
		t.Fatalf("expected unchanged resource to be omitted, got %q", rendered)
	}
	if !strings.Contains(rendered, "Summary: 1 changed, 1 removed, 1 unchanged") {
		t.Fatalf("expected summary counts, got %q", rendered)
	}
}

func TestRenderDiffReportTextColorAlwaysAddsANSI(t *testing.T) {
	t.Parallel()

	report, err := buildDiffReport([]diffDocument{
		{
			ResourcePath: "/customers/acme",
			Local:        resource.Content{Value: map[string]any{"name": "Acme"}},
			Remote:       resource.Content{Value: map[string]any{"name": "Acme Corp"}},
			Entries: []resource.DiffEntry{
				{ResourcePath: "/customers/acme", Path: "/name", Operation: "replace"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected diff report error: %v", err)
	}

	var output bytes.Buffer
	if err := renderDiffReportText(&output, report, diffRenderOptions{
		RequestedPath: "/customers/acme",
		ColorMode:     diffColorAlways,
	}); err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	rendered := output.String()
	if !strings.Contains(rendered, "\x1b[1;36m/customers/acme [CHANGED]\x1b[0m") {
		t.Fatalf("expected colored header, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[2;36m@@") {
		t.Fatalf("expected colored hunk header, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[31m-") || !strings.Contains(rendered, "\x1b[32m+") {
		t.Fatalf("expected colored add/remove lines, got %q", rendered)
	}
}
