package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/internal/cli"
	"github.com/crmarques/declarest/internal/cli/common"
)

func main() {
	appState := core.NewAppState(
		core.BootstrapConfig{},
		config.ContextSelection{Name: contextNameFromArgs(os.Args[1:])},
	)
	deps := common.CommandWiring{
		Reconciler: appState.Reconciler,
		Contexts:   appState.Contexts,
	}

	if err := cli.Execute(deps); err != nil {
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
