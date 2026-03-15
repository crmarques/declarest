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
	"mime"
	"path/filepath"
	"strings"
)

const defaultOctetStreamMediaType = "application/octet-stream"

func DefaultOctetStreamDescriptor() PayloadDescriptor {
	return PayloadDescriptor{
		PayloadType: PayloadTypeOctetStream,
		MediaType:   defaultOctetStreamMediaType,
		Extension:   ".bin",
	}
}

func NormalizePayloadDescriptor(value PayloadDescriptor) PayloadDescriptor {
	extension := normalizePayloadExtension(value.Extension)
	if descriptor, ok := PayloadDescriptorForExtension(extension); ok {
		if strings.TrimSpace(value.MediaType) == "" && strings.TrimSpace(value.PayloadType) == "" {
			return descriptor
		}
	}

	if descriptor, ok := PayloadDescriptorForContentType(value.MediaType); ok {
		if extension != "" {
			descriptor.Extension = resolveConsistentExtension(extension, descriptor.PayloadType)
		}
		return descriptor
	}

	if strings.TrimSpace(value.PayloadType) != "" {
		payloadType := NormalizePayloadType(value.PayloadType)
		mediaType, _ := PayloadMediaType(payloadType)
		resolvedExtension, _ := PayloadExtension(payloadType)
		if extension != "" {
			resolvedExtension = resolveConsistentExtension(extension, payloadType)
		}
		return PayloadDescriptor{
			PayloadType: payloadType,
			MediaType:   mediaType,
			Extension:   resolvedExtension,
		}
	}

	if extension != "" {
		return PayloadDescriptor{
			PayloadType: PayloadTypeOctetStream,
			MediaType:   defaultOctetStreamMediaType,
			Extension:   extension,
		}
	}

	return DefaultOctetStreamDescriptor()
}

// resolveConsistentExtension returns the given extension if it is either
// unknown (custom) or belongs to the same payload type. When the extension
// maps to a different known payload type it is replaced with the canonical
// extension for the resolved type so that the descriptor stays internally
// consistent.
func resolveConsistentExtension(extension string, resolvedPayloadType string) string {
	extDescriptor, known := PayloadDescriptorForExtension(extension)
	if !known {
		// Custom/unknown extension – keep it as-is.
		return extension
	}
	if extDescriptor.PayloadType == resolvedPayloadType {
		// Extension is consistent with the resolved type (e.g. .yml for yaml).
		return extension
	}
	// Extension belongs to a different type – use the canonical extension
	// for the resolved type instead.
	canonical, err := PayloadExtension(resolvedPayloadType)
	if err != nil {
		return extension
	}
	return canonical
}

func IsPayloadDescriptorExplicit(value PayloadDescriptor) bool {
	return strings.TrimSpace(value.PayloadType) != "" ||
		strings.TrimSpace(value.MediaType) != "" ||
		strings.TrimSpace(value.Extension) != ""
}

func PayloadDescriptorForContentType(value string) (PayloadDescriptor, bool) {
	normalized := normalizeContentTypeOrShortname(value)
	if normalized == "" {
		return PayloadDescriptor{}, false
	}

	switch normalized {
	case PayloadTypeJSON:
		return canonicalDescriptor(PayloadTypeJSON), true
	case PayloadTypeYAML:
		return canonicalDescriptor(PayloadTypeYAML), true
	case PayloadTypeXML:
		return canonicalDescriptor(PayloadTypeXML), true
	case PayloadTypeHCL:
		return canonicalDescriptor(PayloadTypeHCL), true
	case PayloadTypeINI:
		return canonicalDescriptor(PayloadTypeINI), true
	case PayloadTypeProperties:
		return canonicalDescriptor(PayloadTypeProperties), true
	case PayloadTypeText, "txt":
		return canonicalDescriptor(PayloadTypeText), true
	case PayloadTypeBinary, PayloadTypeOctetStream:
		return canonicalDescriptor(PayloadTypeOctetStream), true
	case "application/json":
		return PayloadDescriptor{PayloadType: PayloadTypeJSON, MediaType: normalized, Extension: ".json"}, true
	case "application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml":
		return PayloadDescriptor{PayloadType: PayloadTypeYAML, MediaType: normalized, Extension: ".yaml"}, true
	case "application/xml", "text/xml":
		return PayloadDescriptor{PayloadType: PayloadTypeXML, MediaType: normalized, Extension: ".xml"}, true
	case "application/hcl", "text/hcl":
		return PayloadDescriptor{PayloadType: PayloadTypeHCL, MediaType: normalized, Extension: ".hcl"}, true
	case "application/ini", "text/ini":
		return PayloadDescriptor{PayloadType: PayloadTypeINI, MediaType: normalized, Extension: ".ini"}, true
	case "application/properties", "text/properties", "text/x-java-properties":
		return PayloadDescriptor{PayloadType: PayloadTypeProperties, MediaType: normalized, Extension: ".properties"}, true
	case "text/plain":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: normalized, Extension: ".txt"}, true
	case "text/csv":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: normalized, Extension: ".csv"}, true
	case "text/html":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: normalized, Extension: ".html"}, true
	case "text/css":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: normalized, Extension: ".css"}, true
	case "text/javascript", "application/javascript", "application/x-javascript":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: normalized, Extension: ".js"}, true
	case defaultOctetStreamMediaType:
		return PayloadDescriptor{PayloadType: PayloadTypeOctetStream, MediaType: normalized, Extension: ".bin"}, true
	default:
		switch {
		case strings.HasSuffix(normalized, "+json"):
			return PayloadDescriptor{PayloadType: PayloadTypeJSON, MediaType: normalized, Extension: ".json"}, true
		case strings.HasSuffix(normalized, "+xml"):
			return PayloadDescriptor{PayloadType: PayloadTypeXML, MediaType: normalized, Extension: ".xml"}, true
		default:
			return PayloadDescriptor{}, false
		}
	}
}

func PayloadDescriptorForExtension(extension string) (PayloadDescriptor, bool) {
	normalized := normalizePayloadExtension(extension)
	if normalized == "" {
		return PayloadDescriptor{}, false
	}

	switch normalized {
	case ".json":
		return PayloadDescriptor{PayloadType: PayloadTypeJSON, MediaType: "application/json", Extension: normalized}, true
	case ".yaml", ".yml":
		return PayloadDescriptor{PayloadType: PayloadTypeYAML, MediaType: "application/yaml", Extension: normalized}, true
	case ".xml":
		return PayloadDescriptor{PayloadType: PayloadTypeXML, MediaType: "application/xml", Extension: normalized}, true
	case ".hcl":
		return PayloadDescriptor{PayloadType: PayloadTypeHCL, MediaType: "application/hcl", Extension: normalized}, true
	case ".ini":
		return PayloadDescriptor{PayloadType: PayloadTypeINI, MediaType: "application/ini", Extension: normalized}, true
	case ".properties", ".props":
		return PayloadDescriptor{PayloadType: PayloadTypeProperties, MediaType: "text/x-java-properties", Extension: normalized}, true
	case ".txt", ".text":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: "text/plain", Extension: normalized}, true
	case ".csv":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: "text/csv", Extension: normalized}, true
	case ".html", ".htm":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: "text/html", Extension: normalized}, true
	case ".css":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: "text/css", Extension: normalized}, true
	case ".js":
		return PayloadDescriptor{PayloadType: PayloadTypeText, MediaType: "text/javascript", Extension: normalized}, true
	case ".bin":
		return PayloadDescriptor{PayloadType: PayloadTypeOctetStream, MediaType: defaultOctetStreamMediaType, Extension: normalized}, true
	default:
		return PayloadDescriptor{}, false
	}
}

func PayloadDescriptorForFileName(name string) (PayloadDescriptor, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return PayloadDescriptor{}, false
	}
	return PayloadDescriptorForExtension(filepath.Ext(trimmed))
}

func StructuredLookingPayload(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	switch trimmed[0] {
	case '{', '[':
		return true
	default:
		return false
	}
}

func canonicalDescriptor(payloadType string) PayloadDescriptor {
	normalized := NormalizePayloadType(payloadType)
	mediaType, _ := PayloadMediaType(normalized)
	extension, _ := PayloadExtension(normalized)
	return PayloadDescriptor{
		PayloadType: normalized,
		MediaType:   mediaType,
		Extension:   extension,
	}
}

func normalizeContentTypeOrShortname(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(trimmed); err == nil {
		trimmed = strings.ToLower(strings.TrimSpace(parsed))
	}
	return trimmed
}

func normalizePayloadExtension(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, ".") {
		trimmed = "." + trimmed
	}
	return trimmed
}
