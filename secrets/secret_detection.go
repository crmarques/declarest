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

package secrets

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func DetectSecretCandidates(value resource.Value) ([]string, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]struct{})
	if err := collectDetectedCandidates(normalized, "", candidates, 0); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys, nil
}

func collectDetectedCandidates(value any, currentPath string, candidates map[string]struct{}, depth int) error {
	if depth > maxPayloadDepth {
		return faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			field := typed[key]
			attributePath := joinAttributePath(currentPath, key)
			if isLikelySecretKey(key) {
				stringValue, isString := field.(string)
				if isString {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return err
					}
					if !isPlaceholder && isLikelySecretValue(stringValue) {
						candidates[attributePath] = struct{}{}
					}
				}
			}

			if err := collectDetectedCandidates(field, attributePath, candidates, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for idx := range typed {
			if err := collectDetectedCandidates(typed[idx], joinAttributePath(currentPath, strconv.Itoa(idx)), candidates, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
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
