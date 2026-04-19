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

package http

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func encodeRequestBody(contentType string, body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	content := resource.Content{Value: body}
	if typed, ok := body.(resource.Content); ok {
		content = typed
	}
	if strings.TrimSpace(contentType) != "" {
		content.Descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			MediaType:   contentType,
			PayloadType: content.Descriptor.PayloadType,
			Extension:   content.Descriptor.Extension,
		})
	} else if !resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		switch {
		case resource.IsBinaryValue(content.Value):
			content.Descriptor = resource.DefaultOctetStreamDescriptor()
		default:
			content.Descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
				PayloadType: resource.PayloadTypeJSON,
			})
		}
	}

	encoded, err := resource.EncodeContent(content)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decodeResponseBody(
	body []byte,
	headers http.Header,
	fallback resource.PayloadDescriptor,
) (resource.Content, error) {
	if len(body) == 0 {
		return resource.Content{
			Value:      nil,
			Descriptor: normalizeResponseFallbackDescriptor(fallback, body),
		}, nil
	}

	headerDescriptor, headerExplicit := responseHeaderDescriptor(headers)
	fallbackDescriptor := normalizeResponseFallbackDescriptor(fallback, body)
	fallbackExplicit := resource.IsPayloadDescriptorExplicit(fallback)

	candidates := responseDecodeCandidates(headerDescriptor.PayloadType, fallbackDescriptor.PayloadType)
	if len(candidates) == 0 {
		candidates = []string{resource.PayloadTypeJSON}
	}
	if resource.IsStructuredPayloadType(candidates[0]) && len(bytes.TrimSpace(body)) == 0 {
		return resource.Content{Descriptor: fallbackDescriptor}, nil
	}

	var lastErr error
	for _, candidate := range candidates {
		decoded, err := resource.DecodePayload(body, candidate)
		if err == nil {
			return resource.Content{
				Value: decoded,
				Descriptor: descriptorForDecodedCandidate(
					headerDescriptor,
					headerExplicit,
					fallbackDescriptor,
					fallbackExplicit,
					candidate,
				),
			}, nil
		}
		lastErr = err
	}
	return resource.Content{}, lastErr
}

func responseDecodeCandidates(headerType string, fallbackType string) []string {
	candidates := make([]string, 0, 3)
	appendCandidate := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		for _, existing := range candidates {
			if existing == trimmed {
				return
			}
		}
		candidates = append(candidates, trimmed)
	}

	switch {
	case headerType == resource.PayloadTypeText && resource.IsStructuredPayloadType(fallbackType):
		appendCandidate(fallbackType)
		appendCandidate(headerType)
	default:
		appendCandidate(headerType)
		appendCandidate(fallbackType)
	}
	return candidates
}

func responseHeaderDescriptor(headers http.Header) (resource.PayloadDescriptor, bool) {
	if headers == nil {
		return resource.PayloadDescriptor{}, false
	}
	contentType := strings.TrimSpace(headers.Get("Content-Type"))
	if contentType == "" {
		return resource.PayloadDescriptor{}, false
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		MediaType: contentType,
	}), true
}

func normalizeResponseFallbackDescriptor(
	fallback resource.PayloadDescriptor,
	body []byte,
) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(fallback) {
		return resource.NormalizePayloadDescriptor(fallback)
	}
	if resource.StructuredLookingPayload(body) {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			PayloadType: resource.PayloadTypeJSON,
		})
	}
	return resource.DefaultOctetStreamDescriptor()
}

func descriptorForDecodedCandidate(
	headerDescriptor resource.PayloadDescriptor,
	headerExplicit bool,
	fallbackDescriptor resource.PayloadDescriptor,
	fallbackExplicit bool,
	candidate string,
) resource.PayloadDescriptor {
	switch {
	case headerExplicit && headerDescriptor.PayloadType == candidate:
		return headerDescriptor
	case fallbackExplicit && fallbackDescriptor.PayloadType == candidate:
		return fallbackDescriptor
	default:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			PayloadType: candidate,
		})
	}
}

func classifyStatusError(statusCode int, body []byte) error {
	message := fmt.Sprintf("remote request failed with status %d: %s", statusCode, summarizeBody(body))

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return faults.Auth(message, nil)
	case http.StatusNotFound:
		return faults.NotFound(message, nil)
	case http.StatusConflict:
		return faults.Conflict(message, nil)
	}

	if statusCode >= 400 && statusCode < 500 {
		return faults.Invalid(message, nil)
	}
	return faults.Transport(message, nil)
}

func summarizeBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "<empty>"
	}
	if len(trimmed) > 256 {
		return trimmed[:256] + "..."
	}
	return trimmed
}
