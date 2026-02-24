package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/crmarques/declarest/resource"
)

func encodeRequestBody(contentType string, body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	if strings.Contains(strings.ToLower(contentType), "json") || strings.TrimSpace(contentType) == "" {
		normalized, err := resource.Normalize(body)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return nil, validationError("failed to encode JSON request body", err)
		}
		return encoded, nil
	}

	switch typed := body.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, validationError("failed to encode request body", err)
		}
		return encoded, nil
	}
}

func decodeJSONResponse(body []byte) (resource.Value, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, validationError("response body is not valid JSON", err)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func decodeRequestResponse(body []byte) (resource.Value, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}

	value, err := decodeJSONResponse(body)
	if err == nil {
		return value, nil
	}

	return string(body), nil
}

func classifyStatusError(statusCode int, body []byte) error {
	message := fmt.Sprintf("remote request failed with status %d: %s", statusCode, summarizeBody(body))

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return authError(message, nil)
	case http.StatusNotFound:
		return notFoundError(message, nil)
	case http.StatusConflict:
		return conflictError(message, nil)
	}

	if statusCode >= 400 && statusCode < 500 {
		return validationError(message, nil)
	}
	return transportError(message, nil)
}

func summarizeBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "<empty>"
	}
	if len(trimmed) > 512 {
		return trimmed[:512] + "..."
	}
	return trimmed
}
