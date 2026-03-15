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

package config

import (
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

type configPrompter interface {
	IsInteractive(command *cobra.Command) bool
	Input(command *cobra.Command, prompt string, required bool) (string, error)
	Select(command *cobra.Command, prompt string, options []string) (string, error)
	Confirm(command *cobra.Command, prompt string, defaultYes bool) (bool, error)
}

type terminalPrompter struct{}

func (terminalPrompter) IsInteractive(command *cobra.Command) bool {
	return cliutil.IsInteractiveTerminal(command)
}

func (terminalPrompter) Input(command *cobra.Command, prompt string, required bool) (string, error) {
	return cliutil.PromptInput(command, prompt, required)
}

func (terminalPrompter) Select(command *cobra.Command, prompt string, options []string) (string, error) {
	return cliutil.PromptSelect(command, prompt, options)
}

func (terminalPrompter) Confirm(command *cobra.Command, prompt string, defaultYes bool) (bool, error) {
	return cliutil.PromptConfirm(command, prompt, defaultYes)
}
