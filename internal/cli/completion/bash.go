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
	"bytes"
	"regexp"

	"github.com/spf13/cobra"
)

var (
	bashEqualsFlagSuggestionLinePattern = regexp.MustCompile(`^\s*[a-zA-Z0-9_]+\s*\+=\s*\("--[^"]+=\"\)\s*$`)
	bashEqualsFlagSuggestionToken       = regexp.MustCompile(`\s*"--[^"=\s]+="`)
	bashEmptyArrayAppendPattern         = regexp.MustCompile(`^\s*[a-zA-Z0-9_]+\s*\+=\s*\(\s*\)\s*$`)
	bashOutCompgenLoopPattern           = regexp.MustCompile(`(?m)^\s*while IFS='' read -r comp; do\s*\n\s*COMPREPLY\+=\("\$comp"\)\s*\n\s*done < <\(compgen\s+-W\s+"?\$\{?out\}?"?\s+--\s+"?\$\{?cur\}?"?\)\s*$`)
	bashReplyCompgenLoopPattern         = regexp.MustCompile(`(?m)^\s*while IFS='' read -r comp; do\s*\n\s*COMPREPLY\+=\("\$comp"\)\s*\n\s*done < <\(compgen -W "\$\{completions\[\*\]\}" -- "\$cur"\)\s*$`)
	bashArgsLine                        = []byte(`    args=("${words[@]:1}")`)
	bashCursorAwareArgsLines            = []byte(`    local completedWords
    completedWords=("${words[@]:0:$((cword+1))}")
    args=("${completedWords[@]:1}")`)
	bashRequestCompLine            = []byte(`    requestComp="DECLAREST_ACTIVE_HELP=0 ${words[0]} __completeNoDesc ${args[*]}"`)
	bashCursorAwareRequestCompLine = []byte(`    requestComp="DECLAREST_ACTIVE_HELP=0 ${completedWords[0]} __completeNoDesc ${args[*]}"`)
	bashLastParamLine              = []byte(`    lastParam=${words[$((${#words[@]}-1))]}`)
	bashCursorAwareLastParamLine   = []byte(`    lastParam=${completedWords[$((${#completedWords[@]}-1))]}`)
)

func newBashCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate Bash completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			buffer := &bytes.Buffer{}
			if err := command.Root().GenBashCompletion(buffer); err != nil {
				return err
			}

			normalized := normalizeBashFlagSuggestions(buffer.Bytes())
			_, err := command.OutOrStdout().Write(normalized)
			return err
		},
	}
}

func normalizeBashFlagSuggestions(script []byte) []byte {
	lines := bytes.Split(script, []byte{'\n'})
	filtered := make([][]byte, 0, len(lines))
	for _, line := range lines {
		if bashEqualsFlagSuggestionLinePattern.Match(line) {
			continue
		}

		normalizedLine := bashEqualsFlagSuggestionToken.ReplaceAll(line, []byte{})
		if bashEmptyArrayAppendPattern.Match(normalizedLine) {
			continue
		}
		filtered = append(filtered, normalizedLine)
	}
	normalized := bytes.Join(filtered, []byte{'\n'})
	normalized = bytes.ReplaceAll(normalized, bashArgsLine, bashCursorAwareArgsLines)
	normalized = bytes.ReplaceAll(normalized, bashRequestCompLine, bashCursorAwareRequestCompLine)
	normalized = bytes.ReplaceAll(normalized, bashLastParamLine, bashCursorAwareLastParamLine)

	// Bash `compgen -W` emits raw candidates. Quote each candidate when adding
	// to COMPREPLY so values containing spaces stay a single shell token.
	normalized = bashOutCompgenLoopPattern.ReplaceAllLiteral(
		normalized,
		[]byte(`        while IFS='' read -r comp; do
            COMPREPLY+=( "$(printf '%q' "$comp")" )
        done < <(compgen -W "${out// /\\ }" -- "$cur")`),
	)
	normalized = bashReplyCompgenLoopPattern.ReplaceAllLiteral(
		normalized,
		[]byte(`    if [[ $(type -t compopt) = "builtin" ]]; then
        compopt +o filenames
    fi
    while IFS='' read -r comp; do
        COMPREPLY+=("$comp")
    done < <(compgen -W "${completions[*]}" -- "$cur")`),
	)
	return normalized
}
