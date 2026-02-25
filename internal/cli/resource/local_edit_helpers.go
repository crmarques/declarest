package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/repository"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func resolveActiveResourceContext(
	ctx context.Context,
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
) (configdomain.Context, error) {
	contexts, err := common.RequireContexts(deps)
	if err != nil {
		return configdomain.Context{}, err
	}

	contextName := ""
	if globalFlags != nil {
		contextName = strings.TrimSpace(globalFlags.Context)
	}
	if contextName == "" {
		contextName = strings.TrimSpace(common.ContextName(ctx))
	}

	return contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: contextName})
}

func resourcePayloadEditFormat(cfg configdomain.Context) string {
	switch strings.TrimSpace(cfg.Repository.ResourceFormat) {
	case configdomain.ResourceFormatYAML:
		return common.OutputYAML
	default:
		return common.OutputJSON
	}
}

func resourcePayloadEditFilename(cfg configdomain.Context) string {
	if resourcePayloadEditFormat(cfg) == common.OutputYAML {
		return "resource.yaml"
	}
	return "resource.json"
}

func encodeResourcePayloadForEdit(cfg configdomain.Context, value resourcedomain.Value) ([]byte, error) {
	normalized, err := resourcedomain.Normalize(value)
	if err != nil {
		return nil, err
	}

	switch resourcePayloadEditFormat(cfg) {
	case common.OutputYAML:
		return yaml.Marshal(normalized)
	default:
		return json.MarshalIndent(normalized, "", "  ")
	}
}

func decodeResourcePayloadFromEdit(cfg configdomain.Context, data []byte) (resourcedomain.Value, error) {
	return common.DecodeInputData[resourcedomain.Value](data, resourcePayloadEditFormat(cfg))
}

func commitAndMaybeAutoSyncRepository(
	ctx context.Context,
	deps common.CommandDependencies,
	cfg configdomain.Context,
	message string,
) error {
	if cfg.Repository.Git == nil {
		return nil
	}

	committer, err := resolveRepositoryCommitter(deps)
	if err != nil {
		return err
	}
	committed, err := committer.Commit(ctx, message)
	if err != nil {
		return err
	}
	if !committed {
		return nil
	}

	if cfg.Repository.Git.Remote == nil || !cfg.Repository.Git.Remote.AutoSync {
		return nil
	}

	syncService, err := common.RequireRepositorySync(deps)
	if err != nil {
		return err
	}
	return syncService.Push(ctx, repository.PushPolicy{})
}

func commitRepositoryIfGit(
	ctx context.Context,
	deps common.CommandDependencies,
	cfg configdomain.Context,
	message string,
) error {
	if cfg.Repository.Git == nil {
		return nil
	}

	committer, err := resolveRepositoryCommitter(deps)
	if err != nil {
		return err
	}
	_, err = committer.Commit(ctx, message)
	return err
}

func resolveRepositoryCommitter(deps common.CommandDependencies) (repository.RepositoryCommitter, error) {
	var committer repository.RepositoryCommitter
	if candidate, ok := deps.RepositorySync.(repository.RepositoryCommitter); ok {
		committer = candidate
	} else if candidate, ok := deps.ResourceStore.(repository.RepositoryCommitter); ok {
		committer = candidate
	}
	if committer == nil {
		return nil, common.ValidationError("git repository commit capability is not available", nil)
	}
	return committer, nil
}

func ensureCleanGitWorktreeForResourceEdit(
	ctx context.Context,
	deps common.CommandDependencies,
	cfg configdomain.Context,
) error {
	return ensureCleanGitWorktreeForAutoCommit(ctx, deps, cfg, "resource edit")
}

func ensureCleanGitWorktreeForAutoCommit(
	ctx context.Context,
	deps common.CommandDependencies,
	cfg configdomain.Context,
	commandName string,
) error {
	if cfg.Repository.Git == nil {
		return nil
	}
	if shouldSkipCleanGitWorktreeCheckForAutoInitBootstrap(cfg) {
		return nil
	}

	syncService, err := common.RequireRepositorySync(deps)
	if err != nil {
		return err
	}

	status, err := syncService.SyncStatus(ctx)
	if err != nil {
		return err
	}
	if status.HasUncommitted && shouldSkipCleanGitWorktreeCheckForFreshBootstrapRepo(ctx, deps, cfg) {
		return nil
	}
	if status.HasUncommitted {
		return common.ValidationError(
			fmt.Sprintf(
				"%s requires a clean git worktree before mutating repository files so auto-commit does not include unrelated changes",
				commandName,
			),
			nil,
		)
	}

	return nil
}

func shouldSkipCleanGitWorktreeCheckForAutoInitBootstrap(cfg configdomain.Context) bool {
	if cfg.Repository.Git == nil {
		return false
	}
	if !cfg.Repository.Git.Local.AutoInitEnabled() {
		return false
	}

	baseDir := strings.TrimSpace(cfg.Repository.Git.Local.BaseDir)
	if baseDir == "" {
		return false
	}

	gitPath := filepath.Join(baseDir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return false
	} else if !os.IsNotExist(err) {
		return false
	}

	// When .git is absent, the next repository operation will auto-init. We
	// must not treat the future bootstrap commit as unrelated dirty worktree
	// state, or first mutations on uninitialized repos will be blocked.
	return true
}

func shouldSkipCleanGitWorktreeCheckForFreshBootstrapRepo(
	ctx context.Context,
	deps common.CommandDependencies,
	cfg configdomain.Context,
) bool {
	if cfg.Repository.Git == nil {
		return false
	}
	if !cfg.Repository.Git.Local.AutoInitEnabled() {
		return false
	}

	var historyReader repository.RepositoryHistoryReader
	if candidate, ok := deps.RepositorySync.(repository.RepositoryHistoryReader); ok {
		historyReader = candidate
	} else if candidate, ok := deps.ResourceStore.(repository.RepositoryHistoryReader); ok {
		historyReader = candidate
	}
	if historyReader == nil {
		return false
	}

	entries, err := historyReader.History(ctx, repository.HistoryFilter{MaxCount: 1})
	if err != nil {
		return false
	}
	return len(entries) == 0
}

func bindRepositoryCommitMessageFlags(command *cobra.Command, message *string, messageOverride *string) {
	command.Flags().StringVarP(
		message,
		"message",
		"m",
		"",
		"append text to the default git commit message (git repositories only)",
	)
	command.Flags().StringVar(
		messageOverride,
		"message-override",
		"",
		"override the default git commit message (git repositories only)",
	)
}

func resolveRepositoryCommitMessage(
	command *cobra.Command,
	defaultMessage string,
	messageAppend string,
	messageOverride string,
) (string, error) {
	appendChanged := command.Flags().Changed("message")
	overrideChanged := command.Flags().Changed("message-override")

	if appendChanged && overrideChanged {
		return "", common.ValidationError("flags --message and --message-override cannot be used together", nil)
	}

	if overrideChanged {
		resolved := strings.TrimSpace(messageOverride)
		if resolved == "" {
			return "", common.ValidationError("flag --message-override cannot be empty", nil)
		}
		return resolved, nil
	}

	base := strings.TrimSpace(defaultMessage)
	if !appendChanged {
		return base, nil
	}

	appendValue := strings.TrimSpace(messageAppend)
	if appendValue == "" {
		return "", common.ValidationError("flag --message cannot be empty", nil)
	}
	if base == "" {
		return appendValue, nil
	}
	return base + " - " + appendValue, nil
}
