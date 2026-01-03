package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func newPrompter(in io.Reader, out io.Writer) *prompter {
	return &prompter{
		reader: bufio.NewReader(in),
		out:    out,
	}
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
	// Some terminals send backspace/delete bytes; strip them from captured input.
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

func (p *prompter) choice(prompt string, normalize func(string) (string, bool)) (string, error) {
	for {
		value, err := p.readLine(prompt)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("input required")
			}
			return "", err
		}
		if value == "" {
			continue
		}
		if normalized, ok := normalize(value); ok {
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
