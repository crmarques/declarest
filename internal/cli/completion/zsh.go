package completion

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func newZshCommand() *cobra.Command {
	return common.NewPlaceholderCommand("zsh")
}
