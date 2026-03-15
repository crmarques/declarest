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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/bootstrap"
	"github.com/crmarques/declarest/internal/cli"
)

const (
	contextFlagName      = "context"
	contextFlagShort     = "c"
	contextEnvDefaultKey = "DECLAREST_CONTEXT"
)

func main() {
	args := os.Args[1:]
	resolvedInvocation, hasRunnableCommand := cli.ResolveRunnableInvocation(args)
	deps := cli.Dependencies{
		Contexts: bootstrap.NewContextService(bootstrap.BootstrapConfig{}),
	}
	if !shouldSkipContextBootstrap(args, resolvedInvocation, hasRunnableCommand) {
		session, err := bootstrap.NewSession(
			bootstrap.BootstrapConfig{},
			config.ContextSelection{Name: contextNameFromArgs(args, resolvedInvocation, hasRunnableCommand)},
		)
		if err != nil {
			if !isShellCompletionInvocation(args) {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(exitCodeForError(err))
			}
		} else {
			deps = dependenciesFromSession(session)
		}
	}

	if err := cli.Execute(deps); err != nil {
		os.Exit(exitCodeForError(err))
	}
}

func exitCodeForError(err error) int {
	return cli.ExitCodeForError(err)
}

func dependenciesFromSession(s bootstrap.Session) cli.Dependencies {
	return cli.NewDependencies(s.Orchestrator, s.Contexts, s.Services)
}

func contextNameFromArgs(args []string, resolvedInvocation cli.Invocation, ok bool) string {
	if contextName, provided := contextNameFromExplicitContextFlag(args); provided {
		return contextName
	}
	if contextName := strings.TrimSpace(os.Getenv(contextEnvDefaultKey)); contextName != "" {
		return contextName
	}

	return contextNameFromPositionalContextArg(resolvedInvocation, ok)
}

func contextNameFromExplicitContextFlag(args []string) (string, bool) {
	for idx := 0; idx < len(args); idx++ {
		current := args[idx]

		if current == "--"+contextFlagName || current == "-"+contextFlagShort {
			if idx+1 < len(args) {
				return args[idx+1], true
			}
			return "", true
		}
		if strings.HasPrefix(current, "--"+contextFlagName+"=") {
			return strings.TrimPrefix(current, "--"+contextFlagName+"="), true
		}
	}

	return "", false
}

func contextNameFromPositionalContextArg(resolvedInvocation cli.Invocation, ok bool) string {
	if !ok || !commandUsesPositionalContextSelection(resolvedInvocation) {
		return ""
	}

	if len(resolvedInvocation.PositionalArgs) > 0 {
		return strings.TrimSpace(resolvedInvocation.PositionalArgs[0])
	}

	return ""
}

func isHelpInvocation(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if args[0] == "help" {
		return true
	}

	for _, current := range args {
		if current == "--" {
			break
		}
		if current == "--help" || current == "-h" {
			return true
		}
	}

	return false
}

func isCompletionInvocation(args []string) bool {
	return isCompletionScriptInvocation(args) || isShellCompletionInvocation(args)
}

func isCompletionScriptInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "completion"
}

func isShellCompletionInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "__complete" || args[0] == "__completeNoDesc"
}

func shellCompletionRequiresContextBootstrap(args []string) bool {
	if !isShellCompletionInvocation(args) || len(args) <= 1 {
		return false
	}

	completionArgs := args[1:]
	targetArgs := completionArgs
	if len(completionArgs) > 0 {
		targetArgs = completionArgs[:len(completionArgs)-1]
	}
	if len(targetArgs) == 0 {
		return false
	}

	invocation, ok := cli.ResolveRunnableInvocation(targetArgs)
	if !ok {
		return false
	}

	return invocation.RequiresContextBootstrap
}

func shouldSkipContextBootstrap(
	args []string,
	resolvedInvocation cli.Invocation,
	hasRunnableCommand bool,
) bool {
	if isHelpInvocation(args) {
		return true
	}
	if isCompletionScriptInvocation(args) {
		return true
	}
	if isShellCompletionInvocation(args) {
		return !shellCompletionRequiresContextBootstrap(args)
	}

	if !hasRunnableCommand {
		return true
	}

	return !resolvedInvocation.RequiresContextBootstrap
}

func isHelpFallbackInvocation(args []string) bool {
	_, ok := cli.ResolveRunnableInvocation(args)
	return !ok
}

func commandUsesPositionalContextSelection(invocation cli.Invocation) bool {
	return strings.TrimSpace(invocation.ParentCommandName) == "context" &&
		(strings.TrimSpace(invocation.CommandName) == "check" || strings.TrimSpace(invocation.CommandName) == "init")
}
