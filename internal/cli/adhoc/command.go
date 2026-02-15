package adhoc

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	return &cobra.Command{
		Use:   "ad-hoc",
		Short: "Execute ad-hoc operations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return common.NotImplementedError("AdHoc", "Run")
		},
	}
}
