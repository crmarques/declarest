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
	bashOutCompgenPattern               = regexp.MustCompile(`compgen\s+-W\s+"\$\{out\}"\s+--\s+"\$cur"`)
	bashOutCompgenBracedCurPattern      = regexp.MustCompile(`compgen\s+-W\s+"\$\{out\}"\s+--\s+"\$\{cur\}"`)
	bashOutCompgenPlainOutPattern       = regexp.MustCompile(`compgen\s+-W\s+"\$out"\s+--\s+"\$cur"`)
	bashOutCompgenPlainOutBracedCur     = regexp.MustCompile(`compgen\s+-W\s+"\$out"\s+--\s+"\$\{cur\}"`)
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

	// Bash `compgen -W` splits completion words by spaces. Escape spaces in
	// dynamic custom-completion values so aliases like "AD PRD" stay intact.
	normalized = bashOutCompgenPattern.ReplaceAllLiteral(
		normalized,
		[]byte(`compgen -W "${out// /\\ }" -- "$cur"`),
	)
	normalized = bashOutCompgenBracedCurPattern.ReplaceAllLiteral(
		normalized,
		[]byte(`compgen -W "${out// /\\ }" -- "${cur}"`),
	)
	normalized = bashOutCompgenPlainOutPattern.ReplaceAllLiteral(
		normalized,
		[]byte(`compgen -W "${out// /\\ }" -- "$cur"`),
	)
	normalized = bashOutCompgenPlainOutBracedCur.ReplaceAllLiteral(
		normalized,
		[]byte(`compgen -W "${out// /\\ }" -- "${cur}"`),
	)

	return normalized
}
