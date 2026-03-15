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
