package completion

import (
	"strings"
	"testing"
)

func TestNormalizeZshCompletionPreservesCompletionItemsWithSpaces(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`completions+=${comp}`,
		"",
	}, "\n")

	normalized := string(normalizeZshCompletion([]byte(raw)))
	if !strings.Contains(normalized, `completions+=("${comp}")`) {
		t.Fatalf("expected zsh completion append to preserve spaced tokens, got %q", normalized)
	}
}

func TestNormalizeZshCompletionQuotesEvalRequest(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`out=$(eval ${requestComp} 2>/dev/null)`,
		"",
	}, "\n")

	normalized := string(normalizeZshCompletion([]byte(raw)))
	if !strings.Contains(normalized, `out=$(eval "${requestComp}" 2>/dev/null)`) {
		t.Fatalf("expected zsh completion request to be eval-quoted, got %q", normalized)
	}
}
