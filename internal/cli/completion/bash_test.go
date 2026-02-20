package completion

import (
	"strings"
	"testing"
)

func TestNormalizeBashFlagSuggestions(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"flags+=(\"--output=\")",
		"two_word_flags += (\"--context\" \"--context=\")",
		"two_word_flags+=(\"--path\")",
		"local_nonpersistent_flags += ( \"--path=\" )",
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))

	if strings.Contains(normalized, "--output=") || strings.Contains(normalized, "--context=") || strings.Contains(normalized, "--path=") {
		t.Fatalf("expected equals-suffixed suggestions to be removed, got %q", normalized)
	}
	if !strings.Contains(normalized, `two_word_flags += ("--context")`) {
		t.Fatalf("expected non-equals context suggestion to be preserved, got %q", normalized)
	}
	if !strings.Contains(normalized, `two_word_flags+=("--path")`) {
		t.Fatalf("expected non-equals path suggestion to be preserved, got %q", normalized)
	}
	if strings.Contains(normalized, "flags+=()") {
		t.Fatalf("expected empty append lines to be dropped, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpaces(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`done < <(compgen -W "${out}" -- "$cur")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `compgen -W "${out// /\\ }" -- "$cur"`) {
		t.Fatalf("expected custom completion compgen to escape spaces, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithBracedCur(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`done < <(compgen   -W   "${out}"   --   "${cur}")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `compgen -W "${out// /\\ }" -- "${cur}"`) {
		t.Fatalf("expected braced-cur custom completion compgen to escape spaces, got %q", normalized)
	}
}
