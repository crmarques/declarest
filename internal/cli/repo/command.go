package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
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
		newCleanCommand(deps),
		newResetCommand(deps),
		newCheckCommand(deps),
		newPushCommand(deps, globalFlags),
		newCommitCommand(deps, globalFlags),
		newStatusCommand(deps, globalFlags),
		newTreeCommand(deps, globalFlags),
		newHistoryCommand(deps, globalFlags),
	)

	return command
}

func newHistoryCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var maxCount int
	var author string
	var grep string
	var since string
	var until string
	var paths []string
	var reverse bool
	var oneline bool

	command := &cobra.Command{
		Use:   "history",
		Short: "Show local repository history (git repositories only)",
		Example: strings.Join([]string{
			"  declarest repo history",
			"  declarest --context git repo history --max-count 10 --author alice",
			"  declarest repo history --grep fix --since 2026-01-01",
			"  declarest repo history --path customers --path admin/realms",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return common.WriteText(command, common.OutputText, "repo history is not supported for filesystem repositories")
			}
			if repositoryContext.Kind != repositoryContextGit {
				return common.ValidationError("repo history is only available for git repositories", nil)
			}

			historyReader, err := requireRepositoryHistoryReader(deps)
			if err != nil {
				return err
			}

			sinceTime, err := parseHistoryTimeFlag("since", since)
			if err != nil {
				return err
			}
			untilTime, err := parseHistoryTimeFlag("until", until)
			if err != nil {
				return err
			}

			entries, err := historyReader.History(command.Context(), repository.HistoryFilter{
				MaxCount: maxCount,
				Author:   author,
				Grep:     grep,
				Since:    sinceTime,
				Until:    untilTime,
				Paths:    paths,
				Reverse:  reverse,
			})
			if err != nil {
				return err
			}

			format := resolveRepoStatusOutputFormat(globalFlags)
			return common.WriteOutput(command, format, entries, func(w io.Writer, value []repository.HistoryEntry) error {
				return renderRepoHistoryText(w, value, oneline)
			})
		},
	}

	command.Flags().IntVar(&maxCount, "max-count", 0, "limit the number of commits")
	command.Flags().StringVar(&author, "author", "", "show commits by author (substring match on name/email)")
	command.Flags().StringVar(&grep, "grep", "", "show commits whose message matches substring")
	command.Flags().StringVar(&since, "since", "", "show commits more recent than date (YYYY-MM-DD or RFC3339)")
	command.Flags().StringVar(&until, "until", "", "show commits older than date (YYYY-MM-DD or RFC3339)")
	command.Flags().StringArrayVar(&paths, "path", nil, "limit history to commits touching a path (repeatable)")
	command.Flags().BoolVar(&reverse, "reverse", false, "reverse commit order")
	command.Flags().BoolVar(&oneline, "oneline", false, "compact one-line output")

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

func newCleanCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove uncommitted repository changes",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := common.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Clean(command.Context())
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

func newCommitCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var message string

	command := &cobra.Command{
		Use:   "commit",
		Short: "Commit local repository changes (git repositories only)",
		Example: strings.Join([]string{
			`  declarest --context git repo commit --message "manual repository changes"`,
			`  declarest repo commit -m "update local resources"`,
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return common.ValidationError("repo commit is not available for filesystem repositories", nil)
			}
			if repositoryContext.Kind != repositoryContextGit {
				return common.ValidationError("repo commit is only available for git repositories", nil)
			}

			trimmedMessage := strings.TrimSpace(message)
			if trimmedMessage == "" {
				return common.ValidationError("flag --message is required", nil)
			}

			committer, err := requireRepositoryCommitter(deps)
			if err != nil {
				return err
			}
			committed, err := committer.Commit(command.Context(), trimmedMessage)
			if err != nil {
				return err
			}

			format := resolveRepoStatusOutputFormat(globalFlags)
			if format == common.OutputText {
				return nil
			}
			return common.WriteOutput(command, format, repoCommitOutput{Committed: committed}, renderRepoCommitText)
		},
	}

	command.Flags().StringVarP(&message, "message", "m", "", "git commit message")
	return command
}

func newStatusCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository sync status",
		Example: strings.Join([]string{
			"  declarest repo status",
			"  declarest repo status --verbose",
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
			if common.IsVerbose(globalFlags) && repositoryContext.Kind == repositoryContextGit {
				detailsReader, err := requireRepositoryStatusDetailsReader(deps)
				if err != nil {
					return err
				}
				worktreeEntries, err := detailsReader.WorktreeStatus(command.Context())
				if err != nil {
					return err
				}
				if worktreeEntries == nil {
					worktreeEntries = []repository.WorktreeStatusEntry{}
				}
				output.Worktree = worktreeEntries
			}

			format := resolveRepoStatusOutputFormat(globalFlags)
			return common.WriteOutput(command, format, output, func(w io.Writer, value repoStatusOutput) error {
				return renderRepoStatusText(w, value, repositoryContext)
			})
		},
	}
}

func newTreeCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show local repository directory tree (directories only)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			paths, err := resolveRepoTreePaths(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			return common.WriteText(command, resolveRepoStatusOutputFormat(globalFlags), renderRepoTreeText(paths))
		},
	}
}

type repoStatusOutput struct {
	State          repository.SyncState             `json:"state" yaml:"state"`
	Ahead          int                              `json:"ahead" yaml:"ahead"`
	Behind         int                              `json:"behind" yaml:"behind"`
	HasUncommitted bool                             `json:"hasUncommitted" yaml:"hasUncommitted"`
	Worktree       []repository.WorktreeStatusEntry `json:"worktree,omitempty" yaml:"worktree,omitempty"`
}

type repoCommitOutput struct {
	Committed bool `json:"committed" yaml:"committed"`
}

func requireRepositoryHistoryReader(deps common.CommandDependencies) (repository.RepositoryHistoryReader, error) {
	if candidate, ok := deps.RepositorySync.(repository.RepositoryHistoryReader); ok {
		return candidate, nil
	}
	if candidate, ok := deps.ResourceStore.(repository.RepositoryHistoryReader); ok {
		return candidate, nil
	}
	return nil, common.ValidationError("repository history is not supported by the active repository provider", nil)
}

func requireRepositoryStatusDetailsReader(deps common.CommandDependencies) (repository.RepositoryStatusDetailsReader, error) {
	if candidate, ok := deps.RepositorySync.(repository.RepositoryStatusDetailsReader); ok {
		return candidate, nil
	}
	if candidate, ok := deps.ResourceStore.(repository.RepositoryStatusDetailsReader); ok {
		return candidate, nil
	}
	return nil, common.ValidationError("verbose repo status is not supported by the active repository provider", nil)
}

func requireRepositoryCommitter(deps common.CommandDependencies) (repository.RepositoryCommitter, error) {
	if candidate, ok := deps.RepositorySync.(repository.RepositoryCommitter); ok {
		return candidate, nil
	}
	if candidate, ok := deps.ResourceStore.(repository.RepositoryCommitter); ok {
		return candidate, nil
	}
	return nil, common.ValidationError("git repository commit capability is not available", nil)
}

func requireRepositoryTreeReader(deps common.CommandDependencies) (repository.RepositoryTreeReader, bool) {
	if candidate, ok := deps.RepositorySync.(repository.RepositoryTreeReader); ok {
		return candidate, true
	}
	if candidate, ok := deps.ResourceStore.(repository.RepositoryTreeReader); ok {
		return candidate, true
	}
	return nil, false
}

func resolveRepoTreePaths(
	ctx context.Context,
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
) ([]string, error) {
	if treeReader, ok := requireRepositoryTreeReader(deps); ok {
		return treeReader.Tree(ctx)
	}

	if repositoryService, err := common.RequireRepositorySync(deps); err == nil {
		if checkErr := repositoryService.Check(ctx); checkErr != nil {
			return nil, checkErr
		}
	}

	repositoryContext, err := resolveRepositoryContext(ctx, deps, globalFlags)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repositoryContext.BaseDir) == "" {
		return nil, common.ValidationError("repository base directory is not configured", nil)
	}

	return listRepositoryTreePaths(repositoryContext.BaseDir)
}

func listRepositoryTreePaths(baseDir string) ([]string, error) {
	root := filepath.Clean(strings.TrimSpace(baseDir))
	if root == "" || root == "." {
		return nil, common.ValidationError("repository base directory is not configured", nil)
	}

	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, faults.NewTypedError(faults.NotFoundError, "repository base directory does not exist", nil)
		}
		return nil, faults.NewTypedError(faults.InternalError, "failed to inspect repository base directory", err)
	}
	if !info.IsDir() {
		return nil, common.ValidationError("repository base directory is not a directory", nil)
	}

	paths := make([]string, 0, 32)
	walkErr := filepath.WalkDir(root, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if current == root {
			return nil
		}

		name := entry.Name()
		if name == "_" || strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(relPath))
		return nil
	})
	if walkErr != nil {
		return nil, faults.NewTypedError(faults.InternalError, "failed to walk repository directory tree", walkErr)
	}

	sort.Strings(paths)
	return paths, nil
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
	BaseDir   string
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
			BaseDir:   resolvedContext.Repository.Filesystem.BaseDir,
		}, nil
	case resolvedContext.Repository.Git != nil:
		return repositoryContextInfo{
			Kind:      repositoryContextGit,
			HasRemote: resolvedContext.Repository.Git.Remote != nil,
			BaseDir:   resolvedContext.Repository.Git.Local.BaseDir,
		}, nil
	default:
		return repositoryContextInfo{
			Kind:      repositoryContextUnknown,
			HasRemote: false,
			BaseDir:   "",
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
	var err error
	switch repositoryContext.Kind {
	case repositoryContextFilesystem:
		_, err = fmt.Fprintf(
			w,
			"type=filesystem sync=not_applicable hasUncommitted=%t\n",
			value.HasUncommitted,
		)
	case repositoryContextGit:
		if !repositoryContext.HasRemote {
			_, err = fmt.Fprintf(
				w,
				"type=git state=%s remote=not_configured hasUncommitted=%t\n",
				value.State,
				value.HasUncommitted,
			)
			break
		}
		_, err = fmt.Fprintf(
			w,
			"type=git state=%s ahead=%d behind=%d hasUncommitted=%t\n",
			value.State,
			value.Ahead,
			value.Behind,
			value.HasUncommitted,
		)
	default:
		_, err = fmt.Fprintf(
			w,
			"state=%s ahead=%d behind=%d hasUncommitted=%t\n",
			value.State,
			value.Ahead,
			value.Behind,
			value.HasUncommitted,
		)
	}
	if err != nil {
		return err
	}
	return renderRepoWorktreeStatusText(w, value.Worktree)
}

func renderRepoWorktreeStatusText(w io.Writer, entries []repository.WorktreeStatusEntry) error {
	if entries == nil {
		return nil
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "worktree=clean")
		return err
	}
	if _, err := fmt.Fprintln(w, "worktree:"); err != nil {
		return err
	}
	for _, entry := range entries {
		staging := " "
		worktree := " "
		if entry.Staging != "" {
			staging = entry.Staging
		}
		if entry.Worktree != "" {
			worktree = entry.Worktree
		}
		if _, err := fmt.Fprintf(w, "%s%s %s\n", staging, worktree, entry.Path); err != nil {
			return err
		}
	}
	return nil
}

func renderRepoCommitText(w io.Writer, value repoCommitOutput) error {
	if value.Committed {
		_, err := fmt.Fprintln(w, "committed=true")
		return err
	}
	_, err := fmt.Fprintln(w, "committed=false reason=no_changes")
	return err
}

func renderRepoHistoryText(w io.Writer, entries []repository.HistoryEntry, oneline bool) error {
	for idx, entry := range entries {
		if oneline {
			shortHash := strings.TrimSpace(entry.Hash)
			if len(shortHash) > 12 {
				shortHash = shortHash[:12]
			}
			if _, err := fmt.Fprintf(w, "%s %s\n", shortHash, strings.TrimSpace(entry.Subject)); err != nil {
				return err
			}
			continue
		}

		if idx > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "commit %s\n", strings.TrimSpace(entry.Hash)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Author: %s <%s>\n", strings.TrimSpace(entry.Author), strings.TrimSpace(entry.Email)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Date:   %s\n", entry.Date.Format(time.RFC3339)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\n    %s\n", strings.TrimSpace(entry.Subject)); err != nil {
			return err
		}
		if strings.TrimSpace(entry.Body) != "" {
			bodyLines := strings.Split(strings.ReplaceAll(entry.Body, "\r\n", "\n"), "\n")
			for _, line := range bodyLines {
				if _, err := fmt.Fprintf(w, "    %s\n", strings.TrimRight(line, "\r")); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type repoTreeNode struct {
	children map[string]*repoTreeNode
}

func renderRepoTreeText(paths []string) string {
	root := &repoTreeNode{children: map[string]*repoTreeNode{}}

	for _, rawPath := range paths {
		trimmed := strings.Trim(strings.TrimSpace(rawPath), "/")
		if trimmed == "" {
			continue
		}

		segments := strings.Split(trimmed, "/")
		current := root
		valid := true
		for _, segment := range segments {
			if segment == "" || segment == "." || segment == ".." {
				valid = false
				break
			}
			child, ok := current.children[segment]
			if !ok {
				child = &repoTreeNode{children: map[string]*repoTreeNode{}}
				current.children[segment] = child
			}
			current = child
		}
		if !valid {
			continue
		}
	}

	lines := make([]string, 0, len(paths))
	for _, name := range sortedRepoTreeChildren(root) {
		lines = append(lines, name)
		appendRepoTreeChildLines(&lines, root.children[name], "")
	}
	return strings.Join(lines, "\n")
}

func appendRepoTreeChildLines(lines *[]string, node *repoTreeNode, prefix string) {
	if node == nil {
		return
	}

	children := sortedRepoTreeChildren(node)
	for idx, name := range children {
		last := idx == len(children)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}

		*lines = append(*lines, prefix+connector+name)
		appendRepoTreeChildLines(lines, node.children[name], nextPrefix)
	}
}

func sortedRepoTreeChildren(node *repoTreeNode) []string {
	if node == nil || len(node.children) == 0 {
		return nil
	}

	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseHistoryTimeFlag(flagName string, raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	layouts := []string{
		time.RFC3339,
		time.DateOnly,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, common.ValidationError(
		fmt.Sprintf("invalid --%s value: use YYYY-MM-DD or RFC3339", flagName),
		nil,
	)
}
