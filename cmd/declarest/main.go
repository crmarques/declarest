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
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		deps = cli.Dependencies{
			Orchestrator: declarestContext.Orchestrator,
			Contexts:     declarestContext.Contexts,
			Repository:   declarestContext.Repository,
			Metadata:     declarestContext.Metadata,
			Secrets:      declarestContext.Secrets,
		}
	}

	if err := cli.Execute(deps); err != nil {
		os.Exit(1)
	}
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
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "completion", "__complete", "__completeNoDesc":
		return true
	default:
		return false
	}
}

func shouldSkipContextBootstrap(args []string) bool {
	if isHelpInvocation(args) || isCompletionInvocation(args) {
		return true
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
	normalized := strings.TrimSpace(commandPath)
	switch {
	case normalized == "declarest config check":
		return true
	case strings.HasPrefix(normalized, "declarest resource "):
		return true
	case strings.HasPrefix(normalized, "declarest metadata "):
		return true
	case strings.HasPrefix(normalized, "declarest repo "):
		return true
	case strings.HasPrefix(normalized, "declarest secret "):
		return true
	case strings.HasPrefix(normalized, "declarest ad-hoc "):
		return normalized != "declarest ad-hoc"
	}

	return false
}
