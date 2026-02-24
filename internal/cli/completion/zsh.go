package completion

import (
	"bytes"

	"github.com/spf13/cobra"
)

var (
	zshCompletionAppendPattern = []byte(`completions+=${comp}`)
	zshCompletionAppendQuoted  = []byte(`completions+=("${comp}")`)
	zshEvalRequestPattern      = []byte(`out=$(eval ${requestComp} 2>/dev/null)`)
	zshEvalRequestQuoted       = []byte(`out=$(eval "${requestComp}" 2>/dev/null)`)
)

func newZshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate Zsh completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			buffer := &bytes.Buffer{}
			if err := command.Root().GenZshCompletion(buffer); err != nil {
				return err
			}

			normalized := normalizeZshCompletion(buffer.Bytes())
			_, err := command.OutOrStdout().Write(normalized)
			return err
		},
	}
}

func normalizeZshCompletion(script []byte) []byte {
	normalized := bytes.ReplaceAll(script, zshCompletionAppendPattern, zshCompletionAppendQuoted)
	normalized = bytes.ReplaceAll(normalized, zshEvalRequestPattern, zshEvalRequestQuoted)
	return normalized
}
