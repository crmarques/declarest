package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
)

type huhPrompter struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func newHuhPrompter(stdin io.Reader, stdout, stderr io.Writer) interactivePrompter {
	return &huhPrompter{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}

func (h *huhPrompter) runField(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithShowHelp(false).
		WithInput(h.stdin).
		WithOutput(h.stdout)
	return form.Run()
}

func (h *huhPrompter) readLine(prompt string) (string, error) {
	var value string
	field := huh.NewInput().
		Title(prompt).
		Prompt("> ").
		Value(&value)
	if err := h.runField(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *huhPrompter) required(prompt string) (string, error) {
	var value string
	field := huh.NewInput().
		Title(prompt).
		Prompt("> ").
		Value(&value).
		Validate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("input required")
			}
			return nil
		})
	if err := h.runField(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *huhPrompter) optional(prompt string) (string, error) {
	return h.readLine(prompt)
}

func (h *huhPrompter) requiredSecret(prompt string) (string, error) {
	var value string
	field := huh.NewInput().
		Title(prompt).
		Prompt("> ").
		Value(&value).
		EchoMode(huh.EchoModePassword).
		Validate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("input required")
			}
			return nil
		})
	if err := h.runField(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *huhPrompter) optionalSecret(prompt string) (string, error) {
	var value string
	field := huh.NewInput().
		Title(prompt).
		Prompt("> ").
		Value(&value).
		EchoMode(huh.EchoModePassword)
	if err := h.runField(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (h *huhPrompter) choice(label string, options []string, defaultValue string, normalize func(string) (string, bool)) (string, error) {
	defaultValue = strings.TrimSpace(defaultValue)
	defaultNormalized := ""
	if defaultValue != "" {
		normalized, ok := normalize(defaultValue)
		if !ok {
			return "", fmt.Errorf("invalid default choice %q for %s", defaultValue, label)
		}
		defaultNormalized = normalized
	}

	selection := ""
	if defaultValue != "" {
		selection = defaultValue
	} else if defaultNormalized != "" {
		for _, option := range options {
			if normalized, ok := normalize(option); ok && normalized == defaultNormalized {
				selection = option
				break
			}
		}
	}

	opts := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		opts = append(opts, huh.NewOption(option, option))
	}

	field := huh.NewSelect[string]().
		Title(label).
		Options(opts...).
		Value(&selection)
	if err := h.runField(field); err != nil {
		return "", err
	}

	normalized, ok := normalize(selection)
	if !ok {
		return "", fmt.Errorf("invalid choice: %s", selection)
	}
	return normalized, nil
}

func (h *huhPrompter) confirm(prompt string, defaultValue bool) (bool, error) {
	value := defaultValue
	field := huh.NewConfirm().
		Title(prompt).
		Value(&value)
	if err := h.runField(field); err != nil {
		return false, err
	}
	return value, nil
}

func (h *huhPrompter) sectionHeader(title, subtitle string) {
	fmt.Fprintf(h.stdout, "\n%s\n%s\n", title, strings.Repeat("-", len(title)))
	if subtitle != "" {
		fmt.Fprintln(h.stdout, subtitle)
	}
}

func (h *huhPrompter) messagef(format string, args ...interface{}) {
	fmt.Fprintf(h.stderr, format, args...)
}
