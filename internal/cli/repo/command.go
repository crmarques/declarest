package repo

import (
	"context"
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
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
		newPushCommand(deps, globalFlags),
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
			repositoryService, err := common.RequireRepositorySync(deps)
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
			repositoryService, err := common.RequireRepositorySync(deps)
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
			repositoryService, err := common.RequireRepositorySync(deps)
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
			repositoryService, err := common.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Check(command.Context())
		},
	}
}

func newPushCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var forcePush bool

	command := &cobra.Command{
		Use:   "push",
		Short: "Push repository changes",
		Example: strings.Join([]string{
			"  declarest repo push",
			"  declarest repo push --force-push",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return common.ValidationError("repo push is not available for filesystem repositories", nil)
			}
			if repositoryContext.Kind == repositoryContextGit && !repositoryContext.HasRemote {
				return common.ValidationError("repo push requires repository.git.remote configuration", nil)
			}
			return repositoryService.Push(command.Context(), repository.PushPolicy{Force: forcePush})
		},
	}

	command.Flags().BoolVarP(&forcePush, "force-push", "y", false, "force push")
	command.Flags().BoolVar(&forcePush, "force", false, "legacy alias for --force-push")
	_ = command.Flags().MarkHidden("force")
	return command
}

func newStatusCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository sync status",
		Example: strings.Join([]string{
			"  declarest repo status",
			"  declarest repo status --output json",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepositorySync(deps)
			if err != nil {
				return err
			}

			status, err := repositoryService.SyncStatus(command.Context())
			if err != nil {
				return err
			}
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
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
				return renderRepoStatusText(w, value, repositoryContext)
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

type repositoryContextKind string

const (
	repositoryContextUnknown    repositoryContextKind = "unknown"
	repositoryContextFilesystem repositoryContextKind = "filesystem"
	repositoryContextGit        repositoryContextKind = "git"
)

type repositoryContextInfo struct {
	Kind      repositoryContextKind
	HasRemote bool
}

func resolveRepositoryContext(
	ctx context.Context,
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
) (repositoryContextInfo, error) {
	contexts, err := common.RequireContexts(deps)
	if err != nil {
		return repositoryContextInfo{}, err
	}

	resolvedContext, err := contexts.ResolveContext(ctx, configdomain.ContextSelection{
		Name: selectedContextName(globalFlags, ctx),
	})
	if err != nil {
		return repositoryContextInfo{}, err
	}

	switch {
	case resolvedContext.Repository.Filesystem != nil:
		return repositoryContextInfo{
			Kind:      repositoryContextFilesystem,
			HasRemote: false,
		}, nil
	case resolvedContext.Repository.Git != nil:
		return repositoryContextInfo{
			Kind:      repositoryContextGit,
			HasRemote: resolvedContext.Repository.Git.Remote != nil,
		}, nil
	default:
		return repositoryContextInfo{
			Kind:      repositoryContextUnknown,
			HasRemote: false,
		}, nil
	}
}

func selectedContextName(globalFlags *common.GlobalFlags, ctx context.Context) string {
	if globalFlags != nil && strings.TrimSpace(globalFlags.Context) != "" {
		return strings.TrimSpace(globalFlags.Context)
	}
	return strings.TrimSpace(common.ContextName(ctx))
}

func renderRepoStatusText(w io.Writer, value repoStatusOutput, repositoryContext repositoryContextInfo) error {
	switch repositoryContext.Kind {
	case repositoryContextFilesystem:
		_, err := fmt.Fprintf(
			w,
			"type=filesystem sync=not_applicable hasUncommitted=%t\n",
			value.HasUncommitted,
		)
		return err
	case repositoryContextGit:
		if !repositoryContext.HasRemote {
			_, err := fmt.Fprintf(
				w,
				"type=git state=%s remote=not_configured hasUncommitted=%t\n",
				value.State,
				value.HasUncommitted,
			)
			return err
		}
		_, err := fmt.Fprintf(
			w,
			"type=git state=%s ahead=%d behind=%d hasUncommitted=%t\n",
			value.State,
			value.Ahead,
			value.Behind,
			value.HasUncommitted,
		)
		return err
	default:
		_, err := fmt.Fprintf(
			w,
			"state=%s ahead=%d behind=%d hasUncommitted=%t\n",
			value.State,
			value.Ahead,
			value.Behind,
			value.HasUncommitted,
		)
		return err
	}
}
