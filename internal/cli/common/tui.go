package common

import (
	"errors"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func PromptInput(command *cobra.Command, prompt string, required bool) (string, error) {
	if !IsInteractiveTerminal(command) {
		return "", ValidationError("interactive terminal is required", nil)
	}

	value := ""
	field := huh.NewInput().
		Title(normalizePrompt(prompt)).
		Value(&value)
	if required {
		field.Validate(huh.ValidateNotEmpty())
	}

	if err := runInteractiveField(command, field); err != nil {
		return "", err
	}

	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", ValidationError("value is required", nil)
	}
	return value, nil
}

func PromptSelect(command *cobra.Command, prompt string, options []string) (string, error) {
	if len(options) == 0 {
		return "", ValidationError("no options available", nil)
	}
	if !IsInteractiveTerminal(command) {
		return "", ValidationError("interactive terminal is required", nil)
	}

	selected := options[0]
	values := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		values = append(values, huh.NewOption(option, option))
	}

	field := huh.NewSelect[string]().
		Title(normalizePrompt(prompt)).
		Options(values...).
		Value(&selected)

	if err := runInteractiveField(command, field); err != nil {
		return "", err
	}
	return selected, nil
}

func PromptConfirm(command *cobra.Command, prompt string, defaultYes bool) (bool, error) {
	if !IsInteractiveTerminal(command) {
		return false, ValidationError("interactive terminal is required", nil)
	}

	value := defaultYes
	field := huh.NewConfirm().
		Title(normalizePrompt(prompt)).
		Value(&value)

	if err := runInteractiveField(command, field); err != nil {
		return false, err
	}
	return value, nil
}

func runInteractiveField(command *cobra.Command, field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(command.InOrStdin()).
		WithOutput(command.OutOrStdout()).
		WithShowHelp(false)

	err := form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return ValidationError("interactive prompt interrupted", nil)
	}
	return err
}

func normalizePrompt(prompt string) string {
	title := strings.TrimSpace(prompt)
	title = strings.TrimSuffix(title, ":")
	if title == "" {
		return "Input"
	}
	return title
}
