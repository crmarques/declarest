package secrets

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

const wholeResourceSecretEncodingPrefix = "declarest:whole-resource:v1:base64:"

// EncodeWholeResourceSecret serializes one complete resource payload into a
// secret-store-safe string while preserving descriptor-aware decode behavior.
func EncodeWholeResourceSecret(content resource.Content) (string, error) {
	encoded, err := resource.EncodeContentPretty(content)
	if err != nil {
		return "", err
	}
	if utf8.Valid(encoded) {
		text := string(encoded)
		if !strings.HasPrefix(text, wholeResourceSecretEncodingPrefix) {
			return text, nil
		}
	}
	return wholeResourceSecretEncodingPrefix + base64.StdEncoding.EncodeToString(encoded), nil
}

// DecodeWholeResourceSecret restores one complete resource payload from a
// secret-store string using the resource payload descriptor.
func DecodeWholeResourceSecret(value string, descriptor resource.PayloadDescriptor) (resource.Value, error) {
	data := []byte(value)
	if strings.HasPrefix(value, wholeResourceSecretEncodingPrefix) {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, wholeResourceSecretEncodingPrefix))
		if err != nil {
			return nil, faults.NewValidationError("stored whole-resource secret payload is invalid", err)
		}
		data = decoded
	}

	content, err := resource.DecodeContent(data, descriptor)
	if err != nil {
		return nil, err
	}
	return content.Value, nil
}

// ResolveWholeResourcePlaceholderForResource resolves an exact whole-payload
// {{secret ...}} placeholder stored as a text or octet-stream resource file.
func ResolveWholeResourcePlaceholderForResource(
	value resource.Value,
	logicalPath string,
	descriptor resource.PayloadDescriptor,
	getFn func(key string) (string, error),
) (resource.Value, bool, error) {
	if getFn == nil {
		return nil, false, nil
	}

	placeholder, ok := wholeResourcePlaceholderString(value)
	if !ok {
		return nil, false, nil
	}

	key, isCurrent, isPlaceholder, err := parseSecretPlaceholder(placeholder)
	if err != nil {
		return nil, true, err
	}
	if !isPlaceholder {
		return nil, false, nil
	}

	resolvedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, true, err
	}

	secretKey, err := resolveWholeResourceSecretKey(resolvedPath, key, isCurrent)
	if err != nil {
		return nil, true, err
	}

	secretValue, err := getFn(secretKey)
	if err != nil {
		return nil, true, err
	}

	decoded, err := DecodeWholeResourceSecret(secretValue, descriptor)
	if err != nil {
		return nil, true, err
	}
	return decoded, true, nil
}

func wholeResourcePlaceholderString(value resource.Value) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case resource.BinaryValue:
		return string(typed.Bytes), true
	case *resource.BinaryValue:
		if typed == nil {
			return "", false
		}
		return string(typed.Bytes), true
	default:
		return "", false
	}
}

func resolveWholeResourceSecretKey(logicalPath string, key string, isCurrent bool) (string, error) {
	if isCurrent {
		return strings.TrimSpace(logicalPath) + ":.", nil
	}

	resolved := strings.TrimSpace(key)
	if resolved == "" {
		return "", faults.NewValidationError("secret placeholder key must not be empty", nil)
	}
	if strings.HasPrefix(resolved, "/") {
		return "", faults.NewValidationError("secret placeholder key must be relative to the resource path", nil)
	}
	return strings.TrimSpace(logicalPath) + ":" + resolved, nil
}
