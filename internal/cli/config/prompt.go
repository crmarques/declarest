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
