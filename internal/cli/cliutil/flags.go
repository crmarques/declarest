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
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	GlobalFlagContext                = "context"
	GlobalFlagContextShort           = "c"
	GlobalFlagVerbose                = "verbose"
	GlobalFlagVerboseShort           = "v"
	GlobalFlagVerboseInsecure        = "verbose-insecure"
	GlobalFlagSkipResultMessage      = "skip-result-message"
	GlobalFlagSkipResultMessageShort = "n"
	GlobalFlagIgnoreWarnings         = "ignore-warnings"
	GlobalFlagNoColor                = "no-color"
	GlobalFlagOutput                 = "output"
	GlobalFlagOutputShort            = "o"

	GlobalEnvContext                 = "DECLAREST_CONTEXT"
	GlobalEnvOutput                  = "DECLAREST_OUTPUT"
	GlobalEnvVerbose                 = "DECLAREST_VERBOSE"
	GlobalEnvVerboseInsecure         = "DECLAREST_VERBOSE_INSECURE"
	GlobalEnvSkipResultMessage       = "DECLAREST_SKIP_RESULT_MESSAGE"
	GlobalEnvSkipResultMessageLegacy = "DECLAREST_NO_STATUS"
	GlobalEnvIgnoreWarnings          = "DECLAREST_IGNORE_WARNINGS"
	GlobalEnvNoColor                 = "DECLAREST_NO_COLOR"
	GlobalEnvNoColorLegacy           = "NO_COLOR"
)

type GlobalFlags struct {
	Context           string
	Verbose           int
	VerboseInsecure   bool
	SkipResultMessage bool
	IgnoreWarnings    bool
	NoColor           bool
	Output            string
}

type InputFlags struct {
	Payload     string
	ContentType string
}

func BindGlobalFlags(command *cobra.Command, flags *GlobalFlags) {
	flags.Context = EnvOrDefault(GlobalEnvContext, "")
	flags.Verbose = EnvIntOrDefault(GlobalEnvVerbose, 0)
	flags.VerboseInsecure = EnvBoolOrDefault(GlobalEnvVerboseInsecure, false)
	flags.SkipResultMessage = EnvBoolOrDefault(
		GlobalEnvSkipResultMessage,
		EnvBoolOrDefault(GlobalEnvSkipResultMessageLegacy, false),
	)
	flags.IgnoreWarnings = EnvBoolOrDefault(GlobalEnvIgnoreWarnings, false)
	flags.NoColor = EnvBoolOrDefault(GlobalEnvNoColor, EnvPresentOrDefault(GlobalEnvNoColorLegacy, false))
	flags.Output = EnvOrDefault(GlobalEnvOutput, OutputAuto)

	command.PersistentFlags().StringVarP(
		&flags.Context,
		GlobalFlagContext,
		GlobalFlagContextShort,
		flags.Context,
		"context name (env: "+GlobalEnvContext+")",
	)

	vf := command.PersistentFlags().VarPF(
		newVerboseFlag(&flags.Verbose),
		GlobalFlagVerbose,
		GlobalFlagVerboseShort,
		"verbosity level: -v info, -vv detail, -vvv trace (or --verbose=N where N is 1-3) (env: "+GlobalEnvVerbose+")",
	)
	vf.NoOptDefVal = "+1"

	command.PersistentFlags().BoolVar(
		&flags.VerboseInsecure,
		GlobalFlagVerboseInsecure,
		flags.VerboseInsecure,
		"show secrets, tokens, and credentials in verbose output (requires -v; env: "+GlobalEnvVerboseInsecure+")",
	)

	command.PersistentFlags().BoolVarP(
		&flags.SkipResultMessage,
		GlobalFlagSkipResultMessage,
		GlobalFlagSkipResultMessageShort,
		flags.SkipResultMessage,
		"skip result footer output (env: "+GlobalEnvSkipResultMessage+"; legacy: "+GlobalEnvSkipResultMessageLegacy+")",
	)
	command.PersistentFlags().BoolVar(
		&flags.IgnoreWarnings,
		GlobalFlagIgnoreWarnings,
		flags.IgnoreWarnings,
		"suppress standalone warning output (env: "+GlobalEnvIgnoreWarnings+")",
	)
	command.PersistentFlags().BoolVar(
		&flags.NoColor,
		GlobalFlagNoColor,
		flags.NoColor,
		"disable color output (env: "+GlobalEnvNoColor+" or "+GlobalEnvNoColorLegacy+")",
	)
	command.PersistentFlags().StringVarP(
		&flags.Output,
		GlobalFlagOutput,
		GlobalFlagOutputShort,
		flags.Output,
		"output format: auto|text|json|yaml (env: "+GlobalEnvOutput+")",
	)
	RegisterOutputFlagCompletion(command)
}

// IsVerbose returns true when verbosity level is >= 1.
func IsVerbose(flags *GlobalFlags) bool {
	return flags != nil && flags.Verbose >= 1
}

// VerboseLevel returns the resolved verbosity level (0-3).
func VerboseLevel(flags *GlobalFlags) int {
	if flags == nil {
		return 0
	}
	level := flags.Verbose
	if level > 3 {
		level = 3
	}
	return level
}

func BindInputFlags(command *cobra.Command, flags *InputFlags) {
	command.Flags().StringVarP(&flags.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(&flags.ContentType, "content-type", "", "input content type: json|yaml")
	RegisterInputContentTypeFlagCompletion(command)
}

func BindResourceInputFlags(command *cobra.Command, flags *InputFlags) {
	command.Flags().StringVarP(&flags.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(
		&flags.ContentType,
		"content-type",
		"",
		"input content type: json|yaml|xml|hcl|ini|properties|text|binary",
	)
	RegisterResourceInputContentTypeFlagCompletion(command)
}

func BindPathFlag(command *cobra.Command, path *string) {
	command.Flags().StringVarP(path, "path", "p", "", "resource path")
}

// verboseFlag is a custom pflag.Value that supports both -v/-vv/-vvv stacking
// and explicit --verbose=N syntax.
type verboseFlag int

func newVerboseFlag(p *int) *verboseFlag {
	return (*verboseFlag)(p)
}

func (f *verboseFlag) String() string {
	return strconv.Itoa(int(*f))
}

func (f *verboseFlag) Set(s string) error {
	if s == "+1" {
		*f++
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid verbosity level %q: must be 1, 2, or 3", s)
	}
	if v < 0 || v > 3 {
		return fmt.Errorf("verbosity level must be between 0 and 3, got %d", v)
	}
	*f = verboseFlag(v)
	return nil
}

// Type returns "count" so pflag allows -v/-vv/-vvv short flag stacking.
func (f *verboseFlag) Type() string {
	return "count"
}

func EnvOrDefault(envKey string, defaultValue string) string {
	if value, ok := envValue(envKey); ok {
		return value
	}
	return defaultValue
}

func EnvBoolOrDefault(envKey string, defaultValue bool) bool {
	value, ok := envValue(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func EnvIntOrDefault(envKey string, defaultValue int) int {
	value, ok := envValue(envKey)
	if !ok {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	if parsed < 0 || parsed > 3 {
		return defaultValue
	}
	return parsed
}

func EnvPresentOrDefault(envKey string, defaultValue bool) bool {
	_, ok := envValue(envKey)
	if ok {
		return true
	}
	return defaultValue
}

func envValue(envKey string) (string, bool) {
	value := strings.TrimSpace(os.Getenv(envKey))
	if value == "" {
		return "", false
	}
	return value, true
}
