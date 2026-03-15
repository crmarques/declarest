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
		`while IFS='' read -r comp; do`,
		`    COMPREPLY+=("$comp")`,
		`done < <(compgen -W "${out}" -- "$cur")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected custom completion COMPREPLY to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected custom completion compgen to preserve spaced candidates, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithBracedCur(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		`  COMPREPLY+=("$comp")`,
		`done < <(compgen   -W   "${out}"   --   "${cur}")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected braced-cur custom completion compgen to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected braced-cur custom completion compgen to normalize quoting, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithPlainOut(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		`COMPREPLY+=("$comp")`,
		`done < <(compgen -W "$out" -- "$cur")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected plain-out custom completion compgen to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected plain-out custom completion compgen to normalize quoting, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithPlainOutAndBracedCur(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		`COMPREPLY+=("$comp")`,
		`done < <(compgen   -W "$out" -- "${cur}")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected plain-out+braced-cur custom completion compgen to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected plain-out+braced-cur custom completion compgen to normalize quoting, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithUnquotedOutCur(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		`    COMPREPLY+=("$comp")`,
		`done < <(compgen -W $out -- $cur)`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected unquoted custom completion compgen to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected unquoted custom completion compgen to normalize quoting, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsEscapesCustomCompletionSpacesWithBracedOutUnquotedCur(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		` COMPREPLY+=("$comp")`,
		`done < <(compgen -W ${out} -- $cur)`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `COMPREPLY+=( "$(printf '%q' "$comp")" )`) {
		t.Fatalf("expected braced-out custom completion compgen to quote candidates, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${out// /\\ }" -- "$cur")`) {
		t.Fatalf("expected braced-out custom completion compgen to normalize quoting, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsDoesNotInjectFilenamesCompletionMode(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`complete -o default -o nospace -F __start_declarest declarest`,
		`complete -o default -F __start_declarest declarest`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if strings.Contains(normalized, `complete -o default -o filenames -o nospace -F __start_declarest declarest`) {
		t.Fatalf("expected bash completion to avoid injecting filenames mode, got %q", normalized)
	}
	if strings.Contains(normalized, `complete -o default -o filenames -F __start_declarest declarest`) {
		t.Fatalf("expected bash completion to avoid injecting filenames mode, got %q", normalized)
	}
	if !strings.Contains(normalized, `complete -o default -o nospace -F __start_declarest declarest`) {
		t.Fatalf("expected existing completion options to remain unchanged, got %q", normalized)
	}
	if !strings.Contains(normalized, `complete -o default -F __start_declarest declarest`) {
		t.Fatalf("expected existing completion options to remain unchanged, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsDoesNotInjectFilenamesIntoCompoptNospace(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`compopt -o nospace`,
		`if [[ $(type -t compopt) = "builtin" ]]; then`,
		`    compopt -o nospace`,
		`fi`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if strings.Contains(normalized, `compopt -o nospace -o filenames`) {
		t.Fatalf("expected compopt nospace to avoid filenames mode injection, got %q", normalized)
	}
	if !strings.Contains(normalized, `compopt -o nospace`) {
		t.Fatalf("expected compopt nospace lines to remain unchanged, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsUsesCursorScopedWordsForCustomCompletion(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`    args=("${words[@]:1}")`,
		`    # Disable ActiveHelp which is not supported for bash completion v1`,
		`    requestComp="DECLAREST_ACTIVE_HELP=0 ${words[0]} __completeNoDesc ${args[*]}"`,
		``,
		`    lastParam=${words[$((${#words[@]}-1))]}`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `completedWords=("${words[@]:0:$((cword+1))}")`) {
		t.Fatalf("expected custom completion to truncate words to the active cursor, got %q", normalized)
	}
	if !strings.Contains(normalized, `args=("${completedWords[@]:1}")`) {
		t.Fatalf("expected custom completion args to be rebuilt from cursor-scoped words, got %q", normalized)
	}
	if !strings.Contains(normalized, `requestComp="DECLAREST_ACTIVE_HELP=0 ${completedWords[0]} __completeNoDesc ${args[*]}"`) {
		t.Fatalf("expected requestComp to use cursor-scoped words, got %q", normalized)
	}
	if !strings.Contains(normalized, `lastParam=${completedWords[$((${#completedWords[@]}-1))]}`) {
		t.Fatalf("expected lastParam to use cursor-scoped words, got %q", normalized)
	}
}

func TestNormalizeBashFlagSuggestionsDisablesFilenameModeForCommandLoop(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		`while IFS='' read -r comp; do`,
		`    COMPREPLY+=("$comp")`,
		`done < <(compgen -W "${completions[*]}" -- "$cur")`,
		"",
	}, "\n")

	normalized := string(normalizeBashFlagSuggestions([]byte(raw)))
	if !strings.Contains(normalized, `compopt +o filenames`) {
		t.Fatalf("expected command loop normalization to disable filenames mode, got %q", normalized)
	}
	if strings.Contains(normalized, `COMPREPLY+=("${comp} ")`) {
		t.Fatalf("expected command loop normalization to avoid manual trailing-space insertion, got %q", normalized)
	}
	if !strings.Contains(normalized, `COMPREPLY+=("$comp")`) {
		t.Fatalf("expected command loop normalization to preserve plain command-token insertion, got %q", normalized)
	}
	if !strings.Contains(normalized, `done < <(compgen -W "${completions[*]}" -- "$cur")`) {
		t.Fatalf("expected command loop completion candidates to remain command-based, got %q", normalized)
	}
}
