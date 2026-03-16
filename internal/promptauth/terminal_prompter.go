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

package promptauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
)

type terminalPrompter struct{}

func (terminalPrompter) PromptCredentials(
	_ context.Context,
	target Target,
	keepForSession bool,
	persistentSession bool,
) (Credentials, error) {
	if !isInteractiveTerminal(os.Stdin, os.Stdout) {
		return Credentials{}, faults.NewValidationError(
			fmt.Sprintf(
				"prompt auth for %s requires an interactive terminal when no cached session credentials are available",
				target.Label,
			),
			nil,
		)
	}

	if keepForSession {
		writePromptWarning(
			os.Stderr,
			fmt.Sprintf(
				"credentials for %s will be stored in declarest session environment variables and reused by %s.",
				target.Label,
				sessionReuseScope(persistentSession),
			),
		)
	}

	username := ""
	usernameField := huh.NewInput().
		Title(normalizePrompt(fmt.Sprintf("%s username", target.Label))).
		Value(&username).
		Validate(huh.ValidateNotEmpty())
	if err := runField(usernameField); err != nil {
		return Credentials{}, err
	}

	password := ""
	passwordField := huh.NewInput().
		Title(normalizePrompt(fmt.Sprintf("%s password", target.Label))).
		Value(&password).
		Password(true).
		Validate(huh.ValidateNotEmpty())
	if err := runField(passwordField); err != nil {
		return Credentials{}, err
	}

	return Credentials{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
	}, nil
}

func (terminalPrompter) ConfirmReuse(_ context.Context, source Target, targets []Target) (bool, error) {
	if !isInteractiveTerminal(os.Stdin, os.Stdout) {
		return false, faults.NewValidationError(
			fmt.Sprintf("prompt auth for %s requires an interactive terminal", source.Label),
			nil,
		)
	}

	labels := make([]string, 0, len(targets))
	for _, target := range targets {
		labels = append(labels, target.Label)
	}

	value := false
	field := huh.NewConfirm().
		Title(normalizePrompt("Reuse these credentials for other prompt-auth components in this command")).
		Description(strings.Join(labels, ", ")).
		Value(&value)
	if err := runField(field); err != nil {
		return false, err
	}
	return value, nil
}

func runField(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(os.Stdin).
		WithOutput(os.Stdout).
		WithShowHelp(false)

	err := form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return faults.NewValidationError("interactive prompt interrupted", nil)
	}
	return err
}

func normalizePrompt(value string) string {
	title := strings.TrimSpace(value)
	title = strings.TrimSuffix(title, ":")
	if title == "" {
		return "Input"
	}
	return title
}

func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
	inFile, inInfo, ok := fileFromReader(in)
	if !ok || inFile == nil || inInfo == nil {
		return false
	}
	outFile, outInfo, ok := fileFromWriter(out)
	if !ok || outFile == nil || outInfo == nil {
		return false
	}
	return (inInfo.Mode()&os.ModeCharDevice) != 0 && (outInfo.Mode()&os.ModeCharDevice) != 0
}

func writePromptWarning(w io.Writer, message string) {
	writePromptWarningWithArgs(w, os.Args[1:], message)
}

func writePromptWarningWithArgs(w io.Writer, args []string, message string) {
	if cliutil.ShouldIgnoreWarnings(args) {
		return
	}

	cliutil.WriteWarningLine(w, message)
}

func sessionReuseScope(persistentSession bool) string {
	if persistentSession {
		return "later declarest commands in this terminal session"
	}
	return "this declarest command"
}

func fileFromReader(reader io.Reader) (*os.File, os.FileInfo, bool) {
	file, ok := reader.(*os.File)
	if !ok {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, false
	}
	return file, info, true
}

func fileFromWriter(writer io.Writer) (*os.File, os.FileInfo, bool) {
	file, ok := writer.(*os.File)
	if !ok {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, false
	}
	return file, info, true
}
