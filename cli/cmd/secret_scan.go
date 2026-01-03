package cmd

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"declarest/internal/resource"
	"declarest/internal/secrets"
)

var secretKeySuffixes = []string{
	"secret",
	"password",
	"passphrase",
	"credential",
	"token",
}

var secretKeyQualifiers = []string{
	"api",
	"access",
	"secret",
	"private",
	"client",
	"ssh",
	"signing",
	"jwt",
}

var secretValueQualifiers = []string{
	"secret",
	"password",
	"passphrase",
	"credential",
	"token",
	"key",
}

var booleanStringValues = map[string]struct{}{
	"true":  {},
	"false": {},
	"yes":   {},
	"no":    {},
	"on":    {},
	"off":   {},
}

func findUnmappedSecretPaths(res resource.Resource, mapped []string, collection bool) []string {
	mappedSet := buildSecretPathSet(mapped)
	found := map[string]struct{}{}
	scanForUnmappedSecrets(res.V, "", mappedSet, found, collection)
	if len(found) == 0 {
		return nil
	}
	paths := make([]string, 0, len(found))
	for path := range found {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func buildSecretPathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func scanForUnmappedSecrets(value any, basePath string, mapped map[string]struct{}, found map[string]struct{}, collection bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, next := range typed {
			path := joinSecretPath(basePath, key)
			if isLikelySecretKey(key) {
				switch value := next.(type) {
				case []any:
					for idx, item := range value {
						if !isSecretValue(item) {
							continue
						}
						itemPath := joinSecretPathIndex(path, idx)
						if !isMappedSecretPath(itemPath, mapped, collection) {
							found[itemPath] = struct{}{}
						}
					}
				default:
					if isSecretValue(value) && !isMappedSecretPath(path, mapped, collection) {
						found[path] = struct{}{}
					}
				}
			}
			scanForUnmappedSecrets(next, path, mapped, found, collection)
		}
	case []any:
		for idx, next := range typed {
			path := joinSecretPathIndex(basePath, idx)
			scanForUnmappedSecrets(next, path, mapped, found, collection)
		}
	}
}

func joinSecretPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func joinSecretPathIndex(base string, idx int) string {
	if base == "" {
		return "[" + strconv.Itoa(idx) + "]"
	}
	return base + "[" + strconv.Itoa(idx) + "]"
}

func isMappedSecretPath(path string, mapped map[string]struct{}, collection bool) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	_, ok := mapped[trimmed]
	if ok {
		return true
	}
	if collection {
		trimmed = stripLeadingIndex(trimmed)
		if trimmed != "" {
			_, ok = mapped[trimmed]
			return ok
		}
	}
	return false
}

func stripLeadingIndex(path string) string {
	if !strings.HasPrefix(path, "[") {
		return path
	}
	end := strings.Index(path, "]")
	if end == -1 {
		return path
	}
	rest := strings.TrimPrefix(path[end+1:], ".")
	if rest == "" {
		return path
	}
	return rest
}

func isLikelySecretKey(key string) bool {
	normalized := normalizeSecretKey(key)
	if normalized == "" {
		return false
	}
	if hasSuffixAny(normalized, secretKeySuffixes) {
		return true
	}
	if strings.HasSuffix(normalized, "key") && containsAnySubstring(normalized, secretKeyQualifiers) {
		return true
	}
	if strings.HasSuffix(normalized, "value") && containsAnySubstring(normalized, secretValueQualifiers) {
		return true
	}
	return false
}

func normalizeSecretKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func hasSuffixAny(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func containsAnySubstring(value string, parts []string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

func isSecretValue(value any) bool {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return false
		}
		if _, ok := booleanStringValues[strings.ToLower(trimmed)]; ok {
			return false
		}
		if _, ok := secrets.ParseSecretTemplate(trimmed); ok {
			return false
		}
		return true
	case json.Number:
		return true
	default:
		return false
	}
}
