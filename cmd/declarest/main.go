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
	declarestContext, err := core.NewDeclarestContext(
		core.BootstrapConfig{},
		config.ContextSelection{Name: contextNameFromArgs(os.Args[1:])},
	)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	deps := cli.Dependencies{
		Orchestrator: declarestContext.Orchestrator,
		Contexts:     declarestContext.Contexts,
		Repository:   declarestContext.Repository,
		Metadata:     declarestContext.Metadata,
		Secrets:      declarestContext.Secrets,
	}

	if err = cli.Execute(deps); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
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
