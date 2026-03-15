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

package cliutil

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func FormatStatusLabel(w io.Writer, status string) string {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	label := fmt.Sprintf("[%s]", normalized)
	if !supportsANSIStatus(w) {
		return label
	}

	switch normalized {
	case "OK":
		return "\x1b[1;32m" + label + "\x1b[0m"
	case "ERROR", "FAIL":
		return "\x1b[1;31m" + label + "\x1b[0m"
	case "WARN", "WARNING":
		return "\x1b[1;33m" + label + "\x1b[0m"
	default:
		return label
	}
}

func WriteStatusLine(w io.Writer, status string, message string) {
	label := FormatStatusLabel(w, status)
	rendered := strings.TrimSpace(message)
	if rendered == "" {
		_, _ = fmt.Fprintln(w, label)
		return
	}

	_, _ = fmt.Fprintf(w, "%s %s\n", label, rendered)
}

func WriteWarningLine(w io.Writer, message string) {
	WriteStatusLine(w, "WARNING", message)
}

func ShouldIgnoreWarnings(args []string) bool {
	return parseGlobalBoolFlag(
		args,
		GlobalFlagIgnoreWarnings,
		EnvBoolOrDefault(GlobalEnvIgnoreWarnings, false),
	)
}

func ShouldSuppressColor(args []string) bool {
	return parseGlobalBoolFlag(
		args,
		GlobalFlagNoColor,
		EnvBoolOrDefault(
			GlobalEnvNoColor,
			EnvPresentOrDefault(GlobalEnvNoColorLegacy, false),
		),
	)
}

func supportsANSIStatus(w io.Writer) bool {
	if ShouldSuppressColor(os.Args[1:]) {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil || info == nil {
		return false
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	term := strings.TrimSpace(strings.ToLower(os.Getenv("TERM")))
	return term != "" && term != "dumb"
}

func parseGlobalBoolFlag(args []string, name string, defaultValue bool) bool {
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.ParseErrorsAllowlist = pflag.ParseErrorsAllowlist{
		UnknownFlags: true,
	}
	flags.SetOutput(io.Discard)

	var value bool
	flags.BoolVar(&value, name, defaultValue, "")
	if err := flags.Parse(args); err != nil {
		return defaultValue
	}
	return value
}
