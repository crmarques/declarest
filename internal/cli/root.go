package cli

import (
	"github.com/crmarques/declarest/internal/cli/adhoc"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/internal/cli/completion"
	"github.com/crmarques/declarest/internal/cli/config"
	metadatacmd "github.com/crmarques/declarest/internal/cli/metadata"
	"github.com/crmarques/declarest/internal/cli/repo"
	resourcecmd "github.com/crmarques/declarest/internal/cli/resource"
	"github.com/crmarques/declarest/internal/cli/secret"
	"github.com/crmarques/declarest/internal/cli/version"
	"github.com/spf13/cobra"
)

func NewRootCommand(deps common.CommandWiring) *cobra.Command {
	var globalFlags common.GlobalFlags

	root := &cobra.Command{
		Use:           "declarest",
		Short:         common.PlaceholderMessage,
		RunE:          common.RunPlaceholder,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	common.BindGlobalFlags(root, &globalFlags)

	root.AddCommand(
		config.NewCommand(deps),
		resourcecmd.NewCommand(deps),
		metadatacmd.NewCommand(deps),
		repo.NewCommand(deps),
		secret.NewCommand(deps),
		completion.NewCommand(deps),
		adhoc.NewCommand(deps),
		version.NewCommand(deps),
	)

	return root
}
