package repo

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/repository"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "repo",
		Short: "Manage local repository state",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newInitCommand(deps),
		newRefreshCommand(deps),
		newResetCommand(deps),
		newCheckCommand(deps),
		newPushCommand(deps),
		newStatusCommand(deps, globalFlags),
	)

	return command
}

func newInitCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Init(command.Context())
		},
	}
}

func newRefreshCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Refresh(command.Context())
		},
	}
}

func newResetCommand(deps common.CommandDependencies) *cobra.Command {
	var hard bool

	command := &cobra.Command{
		Use:   "reset",
		Short: "Reset repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Reset(command.Context(), repository.ResetPolicy{Hard: hard})
		},
	}

	command.Flags().BoolVarP(&hard, "hard", "H", false, "hard reset")
	return command
}

func newCheckCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check repository health",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Check(command.Context())
		},
	}
}

func newPushCommand(deps common.CommandDependencies) *cobra.Command {
	var force bool

	command := &cobra.Command{
		Use:   "push",
		Short: "Push repository changes",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Push(command.Context(), repository.PushPolicy{Force: force})
		},
	}

	command.Flags().BoolVarP(&force, "force", "y", false, "force push")
	return command
}

func newStatusCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository sync status",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}

			status, err := repositoryService.SyncStatus(command.Context())
			if err != nil {
				return err
			}

			output := repoStatusOutput{
				State:          status.State,
				Ahead:          status.Ahead,
				Behind:         status.Behind,
				HasUncommitted: status.HasUncommitted,
			}

			format := resolveRepoStatusOutputFormat(globalFlags)
			return common.WriteOutput(command, format, output, func(w io.Writer, value repoStatusOutput) error {
				_, writeErr := fmt.Fprintf(
					w,
					"state=%s ahead=%d behind=%d hasUncommitted=%t\n",
					value.State,
					value.Ahead,
					value.Behind,
					value.HasUncommitted,
				)
				return writeErr
			})
		},
	}
}

type repoStatusOutput struct {
	State          repository.SyncState `json:"state" yaml:"state"`
	Ahead          int                  `json:"ahead" yaml:"ahead"`
	Behind         int                  `json:"behind" yaml:"behind"`
	HasUncommitted bool                 `json:"hasUncommitted" yaml:"hasUncommitted"`
}

func resolveRepoStatusOutputFormat(globalFlags *common.GlobalFlags) string {
	if globalFlags == nil {
		return common.OutputText
	}
	switch globalFlags.Output {
	case "", common.OutputAuto:
		return common.OutputText
	default:
		return globalFlags.Output
	}
}
