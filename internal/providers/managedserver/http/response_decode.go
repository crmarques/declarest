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

	payloadType, ok := resource.PayloadTypeForMediaType(contentType)
	if !ok {
		switch {
		case resource.IsBinaryValue(body):
			payloadType = resource.PayloadTypeOctetStream
		case strings.TrimSpace(contentType) == "":
			payloadType = resource.PayloadTypeJSON
		default:
			payloadType = resource.PayloadTypeText
		}
	}

	encoded, err := resource.EncodePayload(body, payloadType)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func decodeResponseBody(body []byte, headers http.Header, fallbackPayloadType string) (resource.Value, error) {
	if len(body) == 0 {
		return nil, nil
	}

	fallbackType := strings.TrimSpace(fallbackPayloadType)
	headerType := ""
	if headers != nil {
		if contentType := strings.TrimSpace(headers.Get("Content-Type")); contentType != "" {
			if inferred, ok := resource.PayloadTypeForMediaType(contentType); ok {
				headerType = inferred
			}
		}
	}

	candidates := responseDecodeCandidates(headerType, fallbackType)
	if len(candidates) == 0 {
		candidates = []string{resource.PayloadTypeJSON}
	}
	if resource.IsStructuredPayloadType(candidates[0]) && len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}

	var lastErr error
	for _, candidate := range candidates {
		decoded, err := resource.DecodePayload(body, candidate)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
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

func classifyStatusError(statusCode int, body []byte) error {
	message := fmt.Sprintf("remote request failed with status %d: %s", statusCode, summarizeBody(body))

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return authError(message, nil)
	case http.StatusNotFound:
		return notFoundError(message, nil)
	case http.StatusConflict:
		return faults.NewConflictError(message, nil)
	}

	if statusCode >= 400 && statusCode < 500 {
		return faults.NewValidationError(message, nil)
	}
	return transportError(message, nil)
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
