package cli

import (
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
)

func RequiresContextBootstrap(command *cobra.Command) bool {
	return commandmeta.RequiresContextBootstrap(command)
}
