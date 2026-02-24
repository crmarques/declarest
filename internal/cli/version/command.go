package version

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type info struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	BuildDate string `json:"build_date" yaml:"build_date"`
}

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	_ = deps

	command := &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			value := info{Version: Version, Commit: Commit, BuildDate: BuildDate}
			return common.WriteOutput(cmd, globalFlags.Output, value, func(w io.Writer, item info) error {
				_, err := fmt.Fprintf(w, "%s (%s) %s\n", item.Version, item.Commit, item.BuildDate)
				return err
			})
		},
	}

	return command
}
