package completion

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func newBashCommand() *cobra.Command {
	return common.NewPlaceholderCommand("bash")
}
