package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type prompter struct {
	reader          *bufio.Reader
	out             io.Writer
	in              io.Reader
	inputFile       *os.File
	inputIsTerminal bool
	passwordReader  func(fd int) ([]byte, error)
}

func newPrompter(in io.Reader, out io.Writer) *prompter {
	var file *os.File
	isTerm := false
	if f, ok := in.(*os.File); ok {
		file = f
		isTerm = term.IsTerminal(int(f.Fd()))
	}
	return &prompter{
		reader:          bufio.NewReader(in),
		out:             out,
		in:              in,
		inputFile:       file,
		inputIsTerminal: isTerm,
		passwordReader:  term.ReadPassword,
	}
}

type interactivePrompter interface {
	readLine(string) (string, error)
	required(string) (string, error)
	optional(string) (string, error)
	requiredSecret(string) (string, error)
	optionalSecret(string) (string, error)
	choice(string, []string, string, func(string) (string, bool)) (string, error)
	confirm(string, bool) (bool, error)
	sectionHeader(string, string)
	messagef(string, ...interface{})
}

func newInteractivePrompter(cmd *cobra.Command) interactivePrompter {
	fallback := newPrompter(cmd.InOrStdin(), cmd.ErrOrStderr())
	if fallback.inputIsTerminal && isTerminalWriter(cmd.ErrOrStderr()) {
		if inFile, ok := cmd.InOrStdin().(*os.File); ok {
			if outFile, ok := cmd.ErrOrStderr().(*os.File); ok {
				return newHuhPrompter(inFile, outFile, outFile)
			}
		}
	}
	return fallback
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func (p *prompter) readLine(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.out, prompt); err != nil {
		return "", err
	}
	line, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value := strings.TrimSpace(normalizeLineInput(line))
	if errors.Is(err, io.EOF) && value == "" {
		return "", io.EOF
	}
	return value, nil
}

func normalizeLineInput(input string) string {
	if !strings.ContainsAny(input, "\b\x7f") {
		return input
	}

	normalized := make([]rune, 0, len(input))
	for _, r := range input {
		switch r {
		case '\b', '\x7f':
			if len(normalized) > 0 {
				normalized = normalized[:len(normalized)-1]
			}
		default:
			normalized = append(normalized, r)
		}
	}
	return string(normalized)
}

func (p *prompter) required(prompt string) (string, error) {
	for {
		value, err := p.readLine(prompt)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("input required")
			}
			return "", err
		}
		if value != "" {
			return value, nil
		}
	}
}

func (p *prompter) optional(prompt string) (string, error) {
	value, err := p.readLine(prompt)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (p *prompter) readSecret(prompt string) (string, error) {
	if p.inputFile != nil && p.inputIsTerminal && p.passwordReader != nil {
		if _, err := fmt.Fprint(p.out, prompt); err != nil {
			return "", err
		}
		secretBytes, err := p.passwordReader(int(p.inputFile.Fd()))
		fmt.Fprintln(p.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secretBytes)), nil
	}
	return p.readLine(prompt)
}

func (p *prompter) requiredSecret(prompt string) (string, error) {
	for {
		value, err := p.readSecret(prompt)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("input required")
			}
			return "", err
		}
		if value != "" {
			return value, nil
		}
	}
}

func (p *prompter) optionalSecret(prompt string) (string, error) {
	value, err := p.readSecret(prompt)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (p *prompter) choice(label string, options []string, defaultValue string, normalize func(string) (string, bool)) (string, error) {
	defaultValue = strings.TrimSpace(defaultValue)
	defaultNormalized := ""
	if defaultValue != "" {
		normalized, ok := normalize(defaultValue)
		if !ok {
			return "", fmt.Errorf("invalid default choice %q for %s", defaultValue, label)
		}
		defaultNormalized = normalized
	}
	fmt.Fprintf(p.out, "%s options:\n", label)
	for idx, option := range options {
		fmt.Fprintf(p.out, "  %d) %s\n", idx+1, option)
	}
	fmt.Fprintln(p.out)
	defaultSuffix := ""
	if defaultValue != "" {
		defaultSuffix = fmt.Sprintf(" [default %s]", defaultValue)
	}
	promptText := fmt.Sprintf("Select option (name or number)%s: ", defaultSuffix)
	for {
		value, err := p.readLine(promptText)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if defaultValue != "" {
					return defaultNormalized, nil
				}
				return "", errors.New("input required")
			}
			return "", err
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			if defaultValue != "" {
				return defaultNormalized, nil
			}
			continue
		}
		if idx, err := strconv.Atoi(trimmed); err == nil && idx >= 1 && idx <= len(options) {
			trimmed = options[idx-1]
		}
		if normalized, ok := normalize(trimmed); ok {
			return normalized, nil
		}
		fmt.Fprintf(p.out, "invalid choice: %s\n", value)
	}
}

func (p *prompter) confirm(prompt string, defaultValue bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultValue {
		suffix = " [Y/n]: "
	}
	for {
		value, err := p.readLine(prompt + suffix)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return defaultValue, nil
			}
			return false, err
		}
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			return defaultValue, nil
		}
		switch value {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintf(p.out, "invalid choice: %s\n", value)
		}
	}
}

func (p *prompter) sectionHeader(title, subtitle string) {
	fmt.Fprintf(p.out, "\n%s\n%s\n", title, strings.Repeat("-", len(title)))
	if subtitle != "" {
		fmt.Fprintln(p.out, subtitle)
	}
}

func (p *prompter) messagef(format string, args ...interface{}) {
	fmt.Fprintf(p.out, format, args...)
}
