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
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/crmarques/declarest/resource"
	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/term"
)

type diffColorMode string

const (
	diffColorAuto   diffColorMode = "auto"
	diffColorAlways diffColorMode = "always"
	diffColorNever  diffColorMode = "never"
)

type diffDocument struct {
	ResourcePath string
	Local        resource.Content
	Remote       resource.Content
	Entries      []resource.DiffEntry
}

type diffStatus string

const (
	diffStatusChanged   diffStatus = "changed"
	diffStatusAdded     diffStatus = "added"
	diffStatusRemoved   diffStatus = "removed"
	diffStatusUnchanged diffStatus = "unchanged"
)

type diffRenderOptions struct {
	RequestedPath string
	ColorMode     diffColorMode
}

type diffReport struct {
	Sections []diffSection
	Summary  diffSummary
}

type diffSection struct {
	ResourcePath string
	Status       diffStatus
	UnifiedDiff  string
	Note         string
}

type diffSummary struct {
	Added     int
	Changed   int
	Removed   int
	Unchanged int
}

func buildDiffReport(documents []diffDocument) (diffReport, error) {
	sorted := make([]diffDocument, len(documents))
	copy(sorted, documents)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].ResourcePath < sorted[j].ResourcePath
	})

	report := diffReport{
		Sections: make([]diffSection, 0, len(sorted)),
	}
	for _, document := range sorted {
		status := diffStatusForDocument(document)
		report.Summary.increment(status)
		if status == diffStatusUnchanged {
			continue
		}

		section, err := buildDiffSection(document, status)
		if err != nil {
			return diffReport{}, err
		}
		report.Sections = append(report.Sections, section)
	}

	return report, nil
}

func buildDiffSection(document diffDocument, status diffStatus) (diffSection, error) {
	unifiedDiff, err := buildUnifiedDiffText(document)
	if err != nil {
		return diffSection{}, err
	}

	section := diffSection{
		ResourcePath: document.ResourcePath,
		Status:       status,
		UnifiedDiff:  unifiedDiff,
	}
	if strings.TrimSpace(unifiedDiff) == "" {
		switch status {
		case diffStatusAdded:
			section.Note = "Resource exists on managed service only."
		case diffStatusRemoved:
			section.Note = "Resource is missing on the managed service."
		case diffStatusChanged:
			section.Note = "Resource differs after normalization."
		}
	}

	return section, nil
}

func buildUnifiedDiffText(document diffDocument) (string, error) {
	localText, err := encodeNormalizedDiffContent(document.Local, document.Remote.Descriptor)
	if err != nil {
		return "", err
	}
	remoteText, err := encodeNormalizedDiffContent(document.Remote, document.Local.Descriptor)
	if err != nil {
		return "", err
	}
	if localText == remoteText {
		return "", nil
	}

	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(localText),
		B:        difflib.SplitLines(remoteText),
		FromFile: "repository",
		ToFile:   "managed-service",
		Context:  3,
		Eol:      "\n",
	})
	if err != nil {
		return "", err
	}

	return strings.TrimRight(diff, "\n"), nil
}

func encodeNormalizedDiffContent(content resource.Content, fallback resource.PayloadDescriptor) (string, error) {
	if content.Value == nil {
		return "", nil
	}

	descriptor := preferredDiffDescriptor(content, fallback)
	normalized, err := resource.Normalize(content.Value)
	if err != nil {
		return "", err
	}

	if bytesValue, ok := resource.BinaryBytes(normalized); ok {
		return ensureTrailingNewline(renderBinaryDiffSummary(bytesValue, descriptor)), nil
	}

	if resource.IsStructuredPayloadType(descriptor.PayloadType) || isStructuredDiffValue(normalized) {
		return encodeStructuredDiffValue(normalized, descriptor)
	}

	if resource.IsTextPayloadType(descriptor.PayloadType) {
		switch typed := normalized.(type) {
		case string:
			return ensureTrailingNewline(typed), nil
		case []byte:
			return ensureTrailingNewline(string(typed)), nil
		}
	}

	encoded, err := resource.EncodePayloadPretty(normalized, resource.PayloadTypeJSON)
	if err != nil {
		return "", err
	}
	return ensureTrailingNewline(string(encoded)), nil
}

func preferredDiffDescriptor(content resource.Content, fallback resource.PayloadDescriptor) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return resource.NormalizePayloadDescriptor(content.Descriptor)
	}

	if resource.IsBinaryValue(content.Value) {
		return resource.DefaultOctetStreamDescriptor()
	}

	if resource.IsPayloadDescriptorExplicit(fallback) {
		return resource.NormalizePayloadDescriptor(fallback)
	}

	if isStructuredDiffValue(content.Value) {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}

	if _, ok := content.Value.(string); ok {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	}

	return resource.NormalizePayloadDescriptor(content.Descriptor)
}

func encodeStructuredDiffValue(value resource.Value, descriptor resource.PayloadDescriptor) (string, error) {
	payloadType := strings.TrimSpace(descriptor.PayloadType)
	if payloadType == "" || !resource.IsStructuredPayloadType(payloadType) {
		payloadType = resource.PayloadTypeJSON
	}

	encoded, err := resource.EncodePayloadPretty(value, payloadType)
	if err != nil {
		return "", err
	}
	return ensureTrailingNewline(string(encoded)), nil
}

func renderBinaryDiffSummary(bytesValue []byte, descriptor resource.PayloadDescriptor) string {
	label := strings.TrimSpace(descriptor.MediaType)
	if label == "" {
		if payloadType := strings.TrimSpace(descriptor.PayloadType); payloadType != "" {
			label = payloadType
		} else {
			label = "application/octet-stream"
		}
	}
	return fmt.Sprintf("<binary %s, %d bytes>", label, len(bytesValue))
}

func ensureTrailingNewline(value string) string {
	if value == "" || strings.HasSuffix(value, "\n") {
		return value
	}
	return value + "\n"
}

func isStructuredDiffValue(value resource.Value) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func diffStatusForDocument(document diffDocument) diffStatus {
	if len(document.Entries) == 0 && reflect.DeepEqual(document.Local.Value, document.Remote.Value) {
		return diffStatusUnchanged
	}

	localMissing := document.Local.Value == nil
	remoteMissing := document.Remote.Value == nil
	switch {
	case localMissing && !remoteMissing:
		return diffStatusAdded
	case !localMissing && remoteMissing:
		return diffStatusRemoved
	default:
		return diffStatusChanged
	}
}

func collectChangedDiffPaths(documents []diffDocument) []string {
	paths := make([]string, 0, len(documents))
	for _, document := range documents {
		if diffStatusForDocument(document) == diffStatusUnchanged {
			continue
		}
		paths = append(paths, document.ResourcePath)
	}
	sort.Strings(paths)
	return paths
}

func renderDiffPathList(w io.Writer, paths []string) error {
	for _, path := range paths {
		if _, err := fmt.Fprintln(w, path); err != nil {
			return err
		}
	}
	return nil
}

func renderDiffReportText(w io.Writer, report diffReport, options diffRenderOptions) error {
	styler := newDiffStyler(options.ColorMode, w)

	if len(report.Sections) == 0 {
		if report.Summary.total() <= 1 {
			_, err := fmt.Fprintf(w, "No differences for %s.\n", options.RequestedPath)
			return err
		}
		_, err := fmt.Fprintf(w, "No differences under %s.\n", options.RequestedPath)
		return err
	}

	for idx, section := range report.Sections {
		if idx > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		header := fmt.Sprintf("%s [%s]", section.ResourcePath, strings.ToUpper(string(section.Status)))
		if _, err := fmt.Fprintln(w, styler.header(header)); err != nil {
			return err
		}

		if strings.TrimSpace(section.UnifiedDiff) != "" {
			for _, line := range strings.Split(section.UnifiedDiff, "\n") {
				if _, err := fmt.Fprintln(w, styler.diffLine(line)); err != nil {
					return err
				}
			}
		} else if section.Note != "" {
			if _, err := fmt.Fprintln(w, styler.secondary(section.Note)); err != nil {
				return err
			}
		}
	}

	if report.Summary.total() > 1 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, styler.secondary(report.Summary.String()))
		return err
	}

	return nil
}

func (s diffSummary) total() int {
	return s.Added + s.Changed + s.Removed + s.Unchanged
}

func (s *diffSummary) increment(status diffStatus) {
	switch status {
	case diffStatusAdded:
		s.Added++
	case diffStatusChanged:
		s.Changed++
	case diffStatusRemoved:
		s.Removed++
	case diffStatusUnchanged:
		s.Unchanged++
	}
}

func (s diffSummary) String() string {
	parts := make([]string, 0, 4)
	if s.Changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", s.Changed))
	}
	if s.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", s.Added))
	}
	if s.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", s.Removed))
	}
	if s.Unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", s.Unchanged))
	}
	if len(parts) == 0 {
		return "Summary: 0 resources"
	}
	return "Summary: " + strings.Join(parts, ", ")
}

type diffStyler struct {
	enabled bool
}

func newDiffStyler(mode diffColorMode, writer io.Writer) diffStyler {
	switch mode {
	case diffColorAlways:
		return diffStyler{enabled: true}
	case diffColorNever:
		return diffStyler{}
	default:
		return diffStyler{enabled: supportsANSIDiff(writer)}
	}
}

func (s diffStyler) header(value string) string {
	return s.wrap("\x1b[1;36m", value)
}

func (s diffStyler) secondary(value string) string {
	return s.wrap("\x1b[2;36m", value)
}

func (s diffStyler) added(value string) string {
	return s.wrap("\x1b[32m", value)
}

func (s diffStyler) removed(value string) string {
	return s.wrap("\x1b[31m", value)
}

func (s diffStyler) diffLine(value string) string {
	switch {
	case strings.HasPrefix(value, "@@"):
		return s.secondary(value)
	case strings.HasPrefix(value, "---") || strings.HasPrefix(value, "+++"):
		return s.secondary(value)
	case strings.HasPrefix(value, "+"):
		return s.added(value)
	case strings.HasPrefix(value, "-"):
		return s.removed(value)
	default:
		return value
	}
}

func (s diffStyler) wrap(code string, value string) string {
	if !s.enabled || strings.TrimSpace(value) == "" {
		return value
	}
	return code + value + "\x1b[0m"
}

func supportsANSIDiff(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	if !term.IsTerminal(int(file.Fd())) {
		return false
	}

	termEnv := strings.TrimSpace(strings.ToLower(os.Getenv("TERM")))
	return termEnv != "" && termEnv != "dumb"
}
