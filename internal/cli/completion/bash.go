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
	return bytes.Join(filtered, []byte{'\n'})
}
