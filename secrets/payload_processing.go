package secrets

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func NormalizePlaceholders(value resource.Value) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	output, err := normalizePlaceholdersValue(normalized, "")
	if err != nil {
		return nil, err
	}
	return output, nil
}

func MaskPayload(value resource.Value, storeFn func(key string, value string) error) (resource.Value, error) {
	if storeFn == nil {
		return nil, validationError("secret store function must not be nil", nil)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]string)
	if err := collectMaskCandidates(normalized, candidates); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := storeFn(key, candidates[key]); err != nil {
			return nil, err
		}
	}

	output, err := applyMask(normalized, candidates)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func ResolvePayload(value resource.Value, getFn func(key string) (string, error)) (resource.Value, error) {
	if getFn == nil {
		return nil, validationError("secret get function must not be nil", nil)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]string)
	output, err := resolvePayloadValue(normalized, "", cache, getFn)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func DetectSecretCandidates(value resource.Value) ([]string, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]struct{})
	if err := collectDetectedCandidates(normalized, candidates); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys, nil
}

func normalizePlaceholdersValue(value any, currentKey string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			child, err := normalizePlaceholdersValue(typed[key], key)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := normalizePlaceholdersValue(typed[idx], "")
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	case string:
		key, isCurrent, isPlaceholder, err := parseSecretPlaceholder(typed)
		if err != nil {
			return nil, err
		}
		if !isPlaceholder {
			return typed, nil
		}
		resolvedKey, err := resolvePlaceholderKey(key, isCurrent, currentKey)
		if err != nil {
			return nil, err
		}
		return secretPlaceholder(resolvedKey), nil
	default:
		return typed, nil
	}
}

func collectMaskCandidates(value any, candidates map[string]string) error {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			field := typed[key]
			if isLikelySecretKey(key) {
				stringValue, isString := field.(string)
				if !isString {
					if field != nil {
						return validationError("secret masking supports only string values for detected keys", nil)
					}
				} else {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return err
					}
					if !isPlaceholder {
						if _, found := candidates[key]; found {
							return validationError("secret masking key scope is ambiguous", nil)
						}
						candidates[key] = stringValue
					}
				}
			}

			if err := collectMaskCandidates(field, candidates); err != nil {
				return err
			}
		}
	case []any:
		for idx := range typed {
			if err := collectMaskCandidates(typed[idx], candidates); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyMask(value any, candidates map[string]string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			field := typed[key]
			if _, shouldMask := candidates[key]; shouldMask {
				stringValue, isString := field.(string)
				if isString {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return nil, err
					}
					if !isPlaceholder {
						result[key] = secretPlaceholder(key)
						continue
					}
				}
			}

			child, err := applyMask(field, candidates)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := applyMask(typed[idx], candidates)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	default:
		return typed, nil
	}
}

func resolvePayloadValue(
	value any,
	currentKey string,
	cache map[string]string,
	getFn func(key string) (string, error),
) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			child, err := resolvePayloadValue(typed[key], key, cache, getFn)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := resolvePayloadValue(typed[idx], "", cache, getFn)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	case string:
		key, isCurrent, isPlaceholder, err := parseSecretPlaceholder(typed)
		if err != nil {
			return nil, err
		}
		if !isPlaceholder {
			return typed, nil
		}

		resolvedKey, err := resolvePlaceholderKey(key, isCurrent, currentKey)
		if err != nil {
			return nil, err
		}

		if cached, found := cache[resolvedKey]; found {
			return cached, nil
		}

		resolved, err := getFn(resolvedKey)
		if err != nil {
			return nil, err
		}
		cache[resolvedKey] = resolved

		return resolved, nil
	default:
		return typed, nil
	}
}

func collectDetectedCandidates(value any, candidates map[string]struct{}) error {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			field := typed[key]
			if isLikelySecretKey(key) {
				stringValue, isString := field.(string)
				if isString {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return err
					}
					if !isPlaceholder {
						candidates[key] = struct{}{}
					}
				}
			}

			if err := collectDetectedCandidates(field, candidates); err != nil {
				return err
			}
		}
	case []any:
		for idx := range typed {
			if err := collectDetectedCandidates(typed[idx], candidates); err != nil {
				return err
			}
		}
	}

	return nil
}

func parseSecretPlaceholder(value string) (key string, isCurrent bool, isPlaceholder bool, err error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false, false, nil
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}")
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "secret") {
		return "", false, false, nil
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "secret"))
	if argument == "" {
		return "", false, true, validationError("secret placeholder argument is required", nil)
	}

	if argument == "." {
		return "", true, true, nil
	}

	if !strings.HasPrefix(argument, "\"") {
		return "", false, true, validationError("secret placeholder must use current key or quoted key", nil)
	}

	parsed, parseErr := strconv.Unquote(argument)
	if parseErr != nil {
		return "", false, true, validationError("secret placeholder key is invalid", parseErr)
	}

	parsed = strings.TrimSpace(parsed)
	if parsed == "" {
		return "", false, true, validationError("secret placeholder key must not be empty", nil)
	}

	return parsed, false, true, nil
}

func resolvePlaceholderKey(key string, isCurrent bool, currentKey string) (string, error) {
	if !isCurrent {
		return key, nil
	}

	resolved := strings.TrimSpace(currentKey)
	if resolved == "" {
		return "", validationError("secret placeholder {{secret .}} requires map field scope", nil)
	}

	return resolved, nil
}

func secretPlaceholder(key string) string {
	return "{{secret " + strconv.Quote(key) + "}}"
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isLikelySecretKey(key string) bool {
	tokens := splitIdentifierTokens(key)
	if len(tokens) == 0 {
		return false
	}

	if hasStrongSecretPair(tokens) {
		return true
	}

	switch strings.Join(tokens, "") {
	case "apikey", "clientsecret", "accesskey", "accesstoken", "privatekey", "bearertoken", "refreshtoken":
		return true
	}

	for idx, token := range tokens {
		if !isSecretCoreToken(token) {
			continue
		}
		if isStandaloneSecretToken(tokens, idx) {
			return true
		}
	}

	return false
}

func hasStrongSecretPair(tokens []string) bool {
	for idx := 0; idx < len(tokens)-1; idx++ {
		pair := tokens[idx] + "_" + tokens[idx+1]
		switch pair {
		case "api_key", "client_secret", "access_key", "access_token", "private_key", "bearer_token", "refresh_token":
		default:
			continue
		}

		if idx+1 == len(tokens)-1 {
			return true
		}

		if isNonSecretQualifierToken(tokens[idx+2]) {
			continue
		}
		return true
	}
	return false
}

func isSecretCoreToken(token string) bool {
	switch token {
	case "password", "passwd", "pwd", "passphrase", "secret", "token":
		return true
	default:
		return false
	}
}

func isStandaloneSecretToken(tokens []string, idx int) bool {
	if idx < 0 || idx >= len(tokens) {
		return false
	}
	if idx == len(tokens)-1 {
		return true
	}
	return !isNonSecretQualifierToken(tokens[idx+1])
}

func isNonSecretQualifierToken(token string) bool {
	switch token {
	case "mode", "type", "policy", "method", "strategy", "preference", "delivery", "conveyance",
		"endpoint", "url", "uri", "path", "lifetime", "ttl", "timeout", "duration",
		"expiry", "expires", "expiration", "validity", "issuer", "name", "id", "length",
		"size", "count", "min", "max", "enabled", "required", "supported", "allowed",
		"algorithm", "alg", "version", "scheme", "file", "ref", "reference":
		return true
	default:
		return false
	}
}

func splitIdentifierTokens(value string) []string {
	tokens := make([]string, 0)
	current := make([]rune, 0)

	flush := func() {
		if len(current) == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(string(current)))
		current = current[:0]
	}

	for _, symbol := range value {
		if !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) {
			flush()
			continue
		}

		if unicode.IsUpper(symbol) && len(current) > 0 {
			previous := current[len(current)-1]
			if unicode.IsLower(previous) || unicode.IsDigit(previous) {
				flush()
			}
		}

		current = append(current, symbol)
	}

	flush()
	return tokens
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}
