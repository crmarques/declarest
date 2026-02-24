package vault

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxVaultResponseBytes = 4 << 20

func (s *VaultSecretService) request(
	ctx context.Context,
	method string,
	endpoint string,
	payload any,
) (vaultResponse, int, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return vaultResponse{}, 0, internalError("failed to encode vault request payload", err)
		}
		body = strings.NewReader(string(encoded))
	}

	requestURL := s.address + endpoint
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return vaultResponse{}, 0, internalError("failed to build vault request", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := strings.TrimSpace(s.token); token != "" {
		req.Header.Set("X-Vault-Token", token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return vaultResponse{}, 0, transportError("vault request failed", err)
	}
	defer resp.Body.Close()

	data, err := readVaultResponseBody(resp.Body)
	if err != nil {
		return vaultResponse{}, 0, transportError("failed to read vault response body", err)
	}

	if len(data) == 0 {
		return vaultResponse{}, resp.StatusCode, nil
	}

	var decoded vaultResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return vaultResponse{}, 0, transportError("failed to decode vault response body", err)
	}

	return decoded, resp.StatusCode, nil
}

func readVaultResponseBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxVaultResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxVaultResponseBytes {
		return nil, errors.New("vault response body too large")
	}
	return data, nil
}

func mapVaultStatus(status int, response vaultResponse, allowNotFound bool, notFoundMessage string) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusNotFound:
		if allowNotFound {
			return nil
		}
		message := notFoundMessage
		if message == "" {
			message = "vault resource not found"
		}
		return notFoundError(message)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return authError(firstVaultError(response, "vault authentication failed"), nil)
	case status >= 500:
		return transportError(firstVaultError(response, "vault service is unavailable"), nil)
	default:
		return validationError(firstVaultError(response, "vault request failed"), nil)
	}
}

func firstVaultError(response vaultResponse, fallback string) string {
	for _, message := range response.Errors {
		trimmed := strings.TrimSpace(message)
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}
