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

func (terminalPrompter) PromptValue(
	_ context.Context,
	credentialName string,
	field string,
	persistInSession bool,
	persistentSession bool,
) (string, error) {
	if !isInteractiveTerminal(os.Stdin, os.Stdout) {
		return "", faults.Invalid(
			fmt.Sprintf(
				"credential %q %s requires an interactive terminal when no cached session value is available",
				strings.TrimSpace(credentialName),
				strings.TrimSpace(field),
			),
			nil,
		)
	}

	if persistInSession {
		writePromptWarning(
			os.Stderr,
			fmt.Sprintf(
				"credential %q %s %s.",
				strings.TrimSpace(credentialName),
				strings.TrimSpace(field),
				sessionReuseScope(persistentSession),
			),
		)
	}

	value := ""
	input := huh.NewInput().
		Title(normalizePrompt(fmt.Sprintf("%s %s", strings.TrimSpace(credentialName), strings.TrimSpace(field)))).
		Value(&value).
		Validate(huh.ValidateNotEmpty())
	if strings.EqualFold(strings.TrimSpace(field), "password") {
		input.EchoMode(huh.EchoModePassword)
	}
	if err := runField(input); err != nil {
		return "", err
	}

	return strings.TrimSpace(value), nil
}

func runField(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(os.Stdin).
		WithOutput(os.Stdout).
		WithShowHelp(false)

	err := form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return faults.Invalid("interactive prompt interrupted", nil)
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
		return "will be reused by later declarest commands in this shell session until the shell exits or you run declarest context clean --credentials-in-session"
	}
	return "cannot be kept across commands because runtime session storage is unavailable and will only be reused by this declarest command"
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
