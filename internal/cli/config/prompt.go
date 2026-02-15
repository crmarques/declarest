package config

import (
	"github.com/crmarques/declarest/internal/cli/common"
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
	return common.IsInteractiveTerminal(command)
}

func (terminalPrompter) Input(command *cobra.Command, prompt string, required bool) (string, error) {
	return common.PromptInput(command, prompt, required)
}

func (terminalPrompter) Select(command *cobra.Command, prompt string, options []string) (string, error) {
	return common.PromptSelect(command, prompt, options)
}

func (terminalPrompter) Confirm(command *cobra.Command, prompt string, defaultYes bool) (bool, error) {
	return common.PromptConfirm(command, prompt, defaultYes)
}
