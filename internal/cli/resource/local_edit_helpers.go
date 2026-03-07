package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func resolveActiveResourceContext(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
) (configdomain.Context, error) {
	contexts, err := cliutil.RequireContexts(deps)
	if err != nil {
		return configdomain.Context{}, err
	}

	contextName := ""
	if globalFlags != nil {
		contextName = strings.TrimSpace(globalFlags.Context)
	}
	if contextName == "" {
		contextName = strings.TrimSpace(cliutil.ContextName(ctx))
	}

	return contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: contextName})
}

func resourcePayloadEditType(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	cfg configdomain.Context,
	logicalPath string,
	value resourcedomain.Value,
) (string, error) {
	if deps.Metadata != nil {
		md, err := deps.Metadata.ResolveForPath(ctx, logicalPath)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(md.PayloadType) != "" {
			return metadata.EffectivePayloadType(md, cfg.Repository.ResourceFormat)
		}
	}

	if _, ok := resourcedomain.BinaryBytes(value); ok {
		return resourcedomain.PayloadTypeOctetStream, nil
	}
	if _, ok := value.(string); ok {
		candidate := metadata.NormalizeResourceFormat(cfg.Repository.ResourceFormat)
		if resourcedomain.IsTextPayloadType(candidate) {
			return candidate, nil
		}
		return resourcedomain.PayloadTypeText, nil
	}

	candidate := metadata.NormalizeResourceFormat(cfg.Repository.ResourceFormat)
	if resourcedomain.IsStructuredPayloadType(candidate) {
		return candidate, nil
	}
	switch candidate {
	case resourcedomain.PayloadTypeXML,
		resourcedomain.PayloadTypeHCL,
		resourcedomain.PayloadTypeINI,
		resourcedomain.PayloadTypeProperties,
		resourcedomain.PayloadTypeText,
		resourcedomain.PayloadTypeOctetStream:
		return resourcedomain.PayloadTypeJSON, nil
	default:
		return candidate, nil
	}
}

func resourcePayloadEditFilename(payloadType string) string {
	extension, err := resourcedomain.PayloadExtension(payloadType)
	if err != nil {
		return "resource.json"
	}
	return "resource" + extension
}

func validateEditPayloadType(payloadType string) error {
	if resourcedomain.IsBinaryPayloadType(payloadType) {
		return cliutil.ValidationError("resource edit does not support octet-stream payloads; use file or stdin based mutation commands", nil)
	}
	return nil
}

func encodeResourcePayloadForEdit(payloadType string, value resourcedomain.Value) ([]byte, error) {
	if err := validateEditPayloadType(payloadType); err != nil {
		return nil, err
	}
	return resourcedomain.EncodePayloadPretty(value, payloadType)
}

func decodeResourcePayloadFromEdit(payloadType string, data []byte) (resourcedomain.Value, error) {
	if err := validateEditPayloadType(payloadType); err != nil {
		return nil, err
	}
	return resourcedomain.DecodePayload(data, payloadType)
}

func commitAndMaybeAutoSyncRepository(
	ctx context.Context,
	deps cliutil.CommandDependencies,
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

	if cfg.Repository.Git.Remote == nil || !cfg.Repository.Git.Remote.AutoSyncEnabled() {
		return nil
	}

	syncService, err := cliutil.RequireRepositorySync(deps)
	if err != nil {
		return err
	}
	return syncService.Push(ctx, repository.PushPolicy{})
}

func validateRepositoryPushFlag(cfg configdomain.Context, push bool) error {
	if !push {
		return nil
	}
	if cfg.Repository.Git == nil {
		return cliutil.ValidationError("flag --push is only available for git repositories", nil)
	}
	if cfg.Repository.Git.Remote == nil {
		return cliutil.ValidationError("flag --push requires repository.git.remote configuration", nil)
	}
	return nil
}

func commitAndMaybePushRepository(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	cfg configdomain.Context,
	message string,
	push bool,
) error {
	if err := validateRepositoryPushFlag(cfg, push); err != nil {
		return err
	}
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
	if !push {
		if cfg.Repository.Git.Remote == nil || !cfg.Repository.Git.Remote.AutoSyncEnabled() {
			return nil
		}
	}

	syncService, err := cliutil.RequireRepositorySync(deps)
	if err != nil {
		return err
	}
	return syncService.Push(ctx, repository.PushPolicy{})
}

func commitRepositoryIfGit(
	ctx context.Context,
	deps cliutil.CommandDependencies,
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

func resolveRepositoryCommitter(deps cliutil.CommandDependencies) (repository.RepositoryCommitter, error) {
	var committer repository.RepositoryCommitter
	if candidate, ok := deps.RepositorySync.(repository.RepositoryCommitter); ok {
		committer = candidate
	} else if candidate, ok := deps.ResourceStore.(repository.RepositoryCommitter); ok {
		committer = candidate
	}
	if committer == nil {
		return nil, cliutil.ValidationError("git repository commit capability is not available", nil)
	}
	return committer, nil
}

func ensureCleanGitWorktreeForResourceEdit(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	cfg configdomain.Context,
) error {
	return ensureCleanGitWorktreeForAutoCommit(ctx, deps, cfg, "resource edit")
}

func ensureCleanGitWorktreeForAutoCommit(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	cfg configdomain.Context,
	commandName string,
) error {
	if cfg.Repository.Git == nil {
		return nil
	}
	if shouldSkipCleanGitWorktreeCheckForAutoInitBootstrap(cfg) {
		return nil
	}

	syncService, err := cliutil.RequireRepositorySync(deps)
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
		return cliutil.ValidationError(
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
	deps cliutil.CommandDependencies,
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
		return "", cliutil.ValidationError("flags --message and --message-override cannot be used together", nil)
	}

	if overrideChanged {
		resolved := strings.TrimSpace(messageOverride)
		if resolved == "" {
			return "", cliutil.ValidationError("flag --message-override cannot be empty", nil)
		}
		return resolved, nil
	}

	base := strings.TrimSpace(defaultMessage)
	if !appendChanged {
		return base, nil
	}

	appendValue := strings.TrimSpace(messageAppend)
	if appendValue == "" {
		return "", cliutil.ValidationError("flag --message cannot be empty", nil)
	}
	if base == "" {
		return appendValue, nil
	}
	return base + " - " + appendValue, nil
}
