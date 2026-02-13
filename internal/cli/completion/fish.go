package completion

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func newFishCommand() *cobra.Command {
	return common.NewPlaceholderCommand("fish")
}
