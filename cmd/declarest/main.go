package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/internal/cli"
)

func main() {
	args := os.Args[1:]
	deps := cli.Dependencies{
		Contexts: core.NewContextService(core.BootstrapConfig{}),
	}
	if !shouldSkipContextBootstrap(args) {
		declarestContext, err := core.NewDeclarestContext(
			core.BootstrapConfig{},
			config.ContextSelection{Name: contextNameFromArgs(args)},
		)
		if err != nil {
			if !isShellCompletionInvocation(args) {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(exitCodeForError(err))
			}
		} else {
			deps = cli.Dependencies{
				Orchestrator:   declarestContext.Orchestrator,
				Contexts:       declarestContext.Contexts,
				ResourceStore:  declarestContext.ResourceStore,
				RepositorySync: declarestContext.RepositorySync,
				Metadata:       declarestContext.Metadata,
				Secrets:        declarestContext.Secrets,
				ResourceServer: declarestContext.ResourceServer,
			}
		}
	}

	if err := cli.Execute(deps); err != nil {
		os.Exit(exitCodeForError(err))
	}
}

func exitCodeForError(err error) int {
	return cli.ExitCodeForError(err)
}

func contextNameFromArgs(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		current := args[idx]

		if current == "--context" || current == "-c" {
			if idx+1 < len(args) {
				return args[idx+1]
			}
			return ""
		}
		if strings.HasPrefix(current, "--context=") {
			return strings.TrimPrefix(current, "--context=")
		}
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

	commandPath, ok := resolveCompletionCommandPath(targetArgs)
	if !ok {
		return false
	}

	return requiresContextBootstrap(commandPath)
}

func resolveCompletionCommandPath(args []string) (string, bool) {
	probe := cli.NewRootCommand(cli.Dependencies{})
	command, _, err := probe.Find(args)
	if err != nil {
		return "", false
	}
	if command == nil {
		return "", false
	}
	if !command.Runnable() {
		return "", false
	}
	return strings.TrimSpace(command.CommandPath()), true
}

func shouldSkipContextBootstrap(args []string) bool {
	if isHelpInvocation(args) {
		return true
	}
	if isCompletionScriptInvocation(args) {
		return true
	}
	if isShellCompletionInvocation(args) {
		return !shellCompletionRequiresContextBootstrap(args)
	}

	commandPath, ok := resolveRunnableCommandPath(args)
	if !ok {
		return true
	}

	return !requiresContextBootstrap(commandPath)
}

func isHelpFallbackInvocation(args []string) bool {
	_, ok := resolveRunnableCommandPath(args)
	return !ok
}

func resolveRunnableCommandPath(args []string) (string, bool) {
	probe := cli.NewRootCommand(cli.Dependencies{})
	command, remainingArgs, err := probe.Find(args)
	if err != nil {
		return "", false
	}
	if command == nil {
		return "", false
	}
	if !command.Runnable() {
		return "", false
	}

	if err := command.ParseFlags(remainingArgs); err != nil {
		return "", false
	}
	if err := command.ValidateArgs(command.Flags().Args()); err != nil {
		return "", false
	}

	return strings.TrimSpace(command.CommandPath()), true
}

func requiresContextBootstrap(commandPath string) bool {
	return cli.RequiresContextBootstrapPath(commandPath)
}
