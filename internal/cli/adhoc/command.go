package adhoc

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps
	return common.NewPlaceholderCommand("ad-hoc")
}
