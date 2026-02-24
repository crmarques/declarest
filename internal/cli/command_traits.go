package cli

import "github.com/crmarques/declarest/internal/cli/commandmeta"

func RequiresContextBootstrapPath(commandPath string) bool {
	return commandmeta.RequiresContextBootstrapPath(commandPath)
}
