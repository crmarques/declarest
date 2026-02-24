package secrets

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
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
	scopeByKey := make(map[string]string)
	if err := collectMaskCandidates(normalized, "", candidates, scopeByKey); err != nil {
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

	output, err := applyMask(normalized, "", candidates)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func ResolvePayload(value resource.Value, getFn func(key string) (string, error)) (resource.Value, error) {
	return resolvePayloadWithResourceScope(value, "", getFn)
}

func ResolvePayloadForResource(
	value resource.Value,
	logicalPath string,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	return resolvePayloadWithResourceScope(value, logicalPath, getFn)
}

// ResolvePayloadDirectivesForResource resolves supported exact-placeholder
// directives in resource payloads. It always resolves {{resource_format .}} and
// resolves {{secret ...}} only when getFn is provided.
func ResolvePayloadDirectivesForResource(
	value resource.Value,
	logicalPath string,
	resourceFormat string,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	resolvedFormat, err := metadata.ValidateResourceFormat(resourceFormat)
	if err != nil {
		return nil, validationError("invalid repository resource format", err)
	}

	withFormat, err := resolveResourceFormatDirectivesValue(normalized, resolvedFormat)
	if err != nil {
		return nil, err
	}

	if getFn == nil {
		return withFormat, nil
	}

	cache := make(map[string]string)
	output, err := resolvePayloadValue(withFormat, "", strings.TrimSpace(logicalPath), cache, getFn)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func resolvePayloadWithResourceScope(
	value resource.Value,
	logicalPath string,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	if getFn == nil {
		return nil, validationError("secret get function must not be nil", nil)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]string)
	output, err := resolvePayloadValue(normalized, "", strings.TrimSpace(logicalPath), cache, getFn)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func resolveResourceFormatDirectivesValue(value any, resourceFormat string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			child, err := resolveResourceFormatDirectivesValue(typed[key], resourceFormat)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := resolveResourceFormatDirectivesValue(typed[idx], resourceFormat)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	case string:
		isDirective, err := parseResourceFormatPlaceholder(typed)
		if err != nil {
			return nil, err
		}
		if isDirective {
			return resourceFormat, nil
		}
		return typed, nil
	default:
		return typed, nil
	}
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

func normalizePlaceholdersValue(value any, currentPath string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			child, err := normalizePlaceholdersValue(typed[key], attributePath)
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
		resolvedKey, err := resolvePlaceholderAttribute(key, isCurrent, currentPath)
		if err != nil {
			return nil, err
		}
		if resolvedKey == currentPath {
			return currentScopeSecretPlaceholder(), nil
		}
		return explicitSecretPlaceholder(resolvedKey), nil
	default:
		return typed, nil
	}
}

func collectMaskCandidates(
	value any,
	currentPath string,
	candidates map[string]string,
	scopeByKey map[string]string,
) error {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
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
						if existingPath, found := scopeByKey[key]; found && existingPath != attributePath {
							return validationError("secret masking key scope is ambiguous", nil)
						}
						scopeByKey[key] = attributePath

						if _, found := candidates[attributePath]; found {
							return validationError("secret masking key scope is ambiguous", nil)
						}
						candidates[attributePath] = stringValue
					}
				}
			}

			if err := collectMaskCandidates(field, attributePath, candidates, scopeByKey); err != nil {
				return err
			}
		}
	case []any:
		for idx := range typed {
			if err := collectMaskCandidates(typed[idx], currentPath, candidates, scopeByKey); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyMask(value any, currentPath string, candidates map[string]string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			field := typed[key]
			if _, shouldMask := candidates[attributePath]; shouldMask {
				stringValue, isString := field.(string)
				if isString {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return nil, err
					}
					if !isPlaceholder {
						result[key] = currentScopeSecretPlaceholder()
						continue
					}
				}
			}

			child, err := applyMask(field, attributePath, candidates)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := applyMask(typed[idx], currentPath, candidates)
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
	currentPath string,
	resourcePath string,
	cache map[string]string,
	getFn func(key string) (string, error),
) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			child, err := resolvePayloadValue(typed[key], attributePath, resourcePath, cache, getFn)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := resolvePayloadValue(typed[idx], "", resourcePath, cache, getFn)
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

		resolvedKey, err := resolvePlaceholderStoreKey(key, isCurrent, currentPath, resourcePath)
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
					if !isPlaceholder && isLikelySecretValue(stringValue) {
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
	if len(inner) > len("secret") {
		next := rune(inner[len("secret")])
		if !unicode.IsSpace(next) {
			return "", false, false, nil
		}
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "secret"))
	if argument == "" {
		return "", false, true, validationError("secret placeholder argument is required", nil)
	}

	if argument == "." {
		return "", true, true, nil
	}

	if strings.HasPrefix(argument, "\"") {
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

	if strings.ContainsAny(argument, " \t\r\n") {
		return "", false, true, validationError("secret placeholder key with spaces must be quoted", nil)
	}

	return argument, false, true, nil
}

func parseResourceFormatPlaceholder(value string) (isPlaceholder bool, err error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return false, nil
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}"))
	if !strings.HasPrefix(inner, "resource_format") {
		return false, nil
	}
	if len(inner) > len("resource_format") {
		next := rune(inner[len("resource_format")])
		if !unicode.IsSpace(next) {
			return false, nil
		}
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "resource_format"))
	if argument == "" {
		return true, validationError("resource_format placeholder argument is required", nil)
	}
	if argument != "." {
		return true, validationError("resource_format placeholder supports only {{resource_format .}}", nil)
	}

	return true, nil
}

func resolvePlaceholderStoreKey(
	key string,
	isCurrent bool,
	currentPath string,
	resourcePath string,
) (string, error) {
	resolvedAttribute, err := resolvePlaceholderAttribute(key, isCurrent, currentPath)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(resourcePath) == "" {
		return resolvedAttribute, nil
	}

	if isAbsoluteSecretKey(resolvedAttribute) {
		return resolvedAttribute, nil
	}

	return strings.TrimSpace(resourcePath) + ":" + resolvedAttribute, nil
}

func resolvePlaceholderAttribute(key string, isCurrent bool, currentPath string) (string, error) {
	if !isCurrent {
		resolved := strings.TrimSpace(key)
		if resolved == "" {
			return "", validationError("secret placeholder key must not be empty", nil)
		}
		return resolved, nil
	}

	resolved := strings.TrimSpace(currentPath)
	if resolved == "" {
		return "", validationError("secret placeholder {{secret .}} requires map field scope", nil)
	}

	return resolved, nil
}

func isAbsoluteSecretKey(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, ":")
}

func isLikelySecretValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	switch strings.ToLower(trimmed) {
	case "true", "false", "yes", "no", "on", "off", "enabled", "disabled":
		return false
	}

	allDigits := true
	for _, symbol := range trimmed {
		if symbol < '0' || symbol > '9' {
			allDigits = false
			break
		}
	}
	return !allDigits
}

func currentScopeSecretPlaceholder() string {
	return "{{secret .}}"
}

func explicitSecretPlaceholder(key string) string {
	return "{{secret " + strconv.Quote(key) + "}}"
}

func joinAttributePath(prefix string, key string) string {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return ""
	}
	if strings.TrimSpace(prefix) == "" {
		return trimmedKey
	}
	return prefix + "." + trimmedKey
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
		if idx > 0 && isNonSecretQualifierToken(tokens[idx-1]) {
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

func isSecretPairPrefixToken(token string) bool {
	switch token {
	case "api", "client", "access", "private", "bearer", "refresh":
		return true
	default:
		return false
	}
}

func isStandaloneSecretToken(tokens []string, idx int) bool {
	if idx < 0 || idx >= len(tokens) {
		return false
	}
	if idx > 0 && isNonSecretQualifierToken(tokens[idx-1]) {
		return false
	}
	if idx == len(tokens)-1 {
		if idx >= 2 &&
			isNonSecretQualifierToken(tokens[idx-2]) &&
			(isSecretCoreToken(tokens[idx-1]) || isSecretPairPrefixToken(tokens[idx-1])) {
			return false
		}
		return true
	}
	if isNonSecretQualifierToken(tokens[idx+1]) {
		return false
	}

	for next := idx + 2; next < len(tokens); next++ {
		if isSecretCoreToken(tokens[next]) {
			return true
		}
		if isNonSecretQualifierToken(tokens[next]) {
			return false
		}
	}

	return true
}

func isNonSecretQualifierToken(token string) bool {
	switch token {
	case "mode", "type", "policy", "method", "strategy", "preference", "delivery", "conveyance",
		"endpoint", "url", "uri", "path", "lifetime", "lifespan", "ttl", "timeout", "duration",
		"expiry", "expires", "expiration", "validity", "issuer", "name", "id", "length",
		"size", "count", "min", "max", "enabled", "enable", "required", "supported", "allowed",
		"algorithm", "alg", "version", "scheme", "file", "ref", "reference", "claim", "claims",
		"header", "response", "exchange", "creation", "created", "time", "timestamp", "requested",
		"request", "use", "lower", "upper", "case", "format":
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
