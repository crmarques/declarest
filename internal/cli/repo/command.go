// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repo

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/crmarques/declarest/repository"
	"github.com/spf13/cobra"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "repository",
		Short: "Manage local repository state",
		Args:  cobra.NoArgs,
	}
	commandmeta.MarkRequiresContextBootstrap(command)

	historyCommand := newHistoryCommand(deps, globalFlags)
	initCommand := newInitCommand(deps)
	refreshCommand := newRefreshCommand(deps)
	cleanCommand := newCleanCommand(deps)
	resetCommand := newResetCommand(deps)
	checkCommand := newCheckCommand(deps)
	pushCommand := newPushCommand(deps, globalFlags)
	commitCommand := newCommitCommand(deps, globalFlags)
	statusCommand := newStatusCommand(deps, globalFlags)
	treeCommand := newTreeCommand(deps, globalFlags)

	commandmeta.MarkTextDefaultStructuredOutput(historyCommand)
	commandmeta.MarkEmitsExecutionStatus(commitCommand)
	commandmeta.MarkTextDefaultStructuredOutput(commitCommand)
	commandmeta.MarkTextDefaultStructuredOutput(statusCommand)
	commandmeta.MarkTextOnlyOutput(treeCommand)

	command.AddCommand(
		initCommand,
		refreshCommand,
		cleanCommand,
		resetCommand,
		checkCommand,
		pushCommand,
		commitCommand,
		statusCommand,
		treeCommand,
		historyCommand,
	)

	return command
}

func newHistoryCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
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
			"  declarest repository history",
			"  declarest --context git repository history --max-count 10 --author alice",
			"  declarest repository history --grep fix --since 2026-01-01",
			"  declarest repository history --path customers --path admin/realms",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return cliutil.WriteText(command, cliutil.OutputText, "repository history is not supported for filesystem repositories")
			}
			if repositoryContext.Kind != repositoryContextGit {
				return cliutil.ValidationError("repository history is only available for git repositories", nil)
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

			format := cliutil.ResolveCommandOutputFormat(command, globalFlags)
			return cliutil.WriteOutput(command, format, entries, func(w io.Writer, value []repository.HistoryEntry) error {
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

func newInitCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Init(command.Context())
		},
	}
}

func newRefreshCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Refresh(command.Context())
		},
	}
}

func newCleanCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove uncommitted repository changes",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Clean(command.Context())
		},
	}
}

func newResetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var hard bool

	command := &cobra.Command{
		Use:   "reset",
		Short: "Reset repository",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Reset(command.Context(), repository.ResetPolicy{Hard: hard})
		},
	}

	command.Flags().BoolVar(&hard, "hard", false, "hard reset")
	return command
}

func newCheckCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check repository health",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			return repositoryService.Check(command.Context())
		},
	}
}

func newPushCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var forcePush bool

	command := &cobra.Command{
		Use:   "push",
		Short: "Push repository changes",
		Example: strings.Join([]string{
			"  declarest repository push",
			"  declarest repository push --force-push",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
			if err != nil {
				return err
			}
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return cliutil.ValidationError("repository push is not available for filesystem repositories", nil)
			}
			if repositoryContext.Kind == repositoryContextGit && !repositoryContext.HasRemote {
				return cliutil.ValidationError("repository push requires repository.git.remote configuration", nil)
			}
			return repositoryService.Push(command.Context(), repository.PushPolicy{Force: forcePush})
		},
	}

	command.Flags().BoolVar(&forcePush, "force-push", false, "force push")
	return command
}

func newCommitCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var message string

	command := &cobra.Command{
		Use:   "commit",
		Short: "Commit local repository changes (git repositories only)",
		Example: strings.Join([]string{
			`  declarest --context git repository commit --message "manual repository changes"`,
			`  declarest repository commit -m "update local resources"`,
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryContext, err := resolveRepositoryContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if repositoryContext.Kind == repositoryContextFilesystem {
				return cliutil.ValidationError("repository commit is not available for filesystem repositories", nil)
			}
			if repositoryContext.Kind != repositoryContextGit {
				return cliutil.ValidationError("repository commit is only available for git repositories", nil)
			}

			trimmedMessage := strings.TrimSpace(message)
			if trimmedMessage == "" {
				return cliutil.ValidationError("flag --message is required", nil)
			}

			committer, err := requireRepositoryCommitter(deps)
			if err != nil {
				return err
			}
			committed, err := committer.Commit(command.Context(), trimmedMessage)
			if err != nil {
				return err
			}

			format := cliutil.ResolveCommandOutputFormat(command, globalFlags)
			if format == cliutil.OutputText {
				return nil
			}
			return cliutil.WriteOutput(command, format, repoCommitOutput{Committed: committed}, renderRepoCommitText)
		},
	}

	command.Flags().StringVarP(&message, "message", "m", "", "git commit message")
	return command
}

func newStatusCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository sync status",
		Example: strings.Join([]string{
			"  declarest repository status",
			"  declarest repository status --verbose",
			"  declarest repository status --output json",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryService, err := cliutil.RequireRepositorySync(deps)
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
			if cliutil.IsVerbose(globalFlags) && repositoryContext.Kind == repositoryContextGit {
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

			format := cliutil.ResolveCommandOutputFormat(command, globalFlags)
			return cliutil.WriteOutput(command, format, output, func(w io.Writer, value repoStatusOutput) error {
				return renderRepoStatusText(w, value, repositoryContext)
			})
		},
	}
}

func newTreeCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show local repository directory tree (directories only)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			paths, err := resolveRepoTreePaths(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			return cliutil.WriteText(command, cliutil.ResolveCommandOutputFormat(command, globalFlags), renderRepoTreeText(paths))
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

func requireRepositoryHistoryReader(deps cliutil.CommandDependencies) (repository.RepositoryHistoryReader, error) {
	if deps.Services != nil {
		if candidate, ok := deps.Services.RepositorySync().(repository.RepositoryHistoryReader); ok {
			return candidate, nil
		}
		if candidate, ok := deps.Services.RepositoryStore().(repository.RepositoryHistoryReader); ok {
			return candidate, nil
		}
	}
	return nil, cliutil.ValidationError("repository history is not supported by the active repository provider", nil)
}

func requireRepositoryStatusDetailsReader(deps cliutil.CommandDependencies) (repository.RepositoryStatusDetailsReader, error) {
	if deps.Services != nil {
		if candidate, ok := deps.Services.RepositorySync().(repository.RepositoryStatusDetailsReader); ok {
			return candidate, nil
		}
		if candidate, ok := deps.Services.RepositoryStore().(repository.RepositoryStatusDetailsReader); ok {
			return candidate, nil
		}
	}
	return nil, cliutil.ValidationError("verbose repository status is not supported by the active repository provider", nil)
}

func requireRepositoryCommitter(deps cliutil.CommandDependencies) (repository.RepositoryCommitter, error) {
	if deps.Services != nil {
		if candidate, ok := deps.Services.RepositorySync().(repository.RepositoryCommitter); ok {
			return candidate, nil
		}
		if candidate, ok := deps.Services.RepositoryStore().(repository.RepositoryCommitter); ok {
			return candidate, nil
		}
	}
	return nil, cliutil.ValidationError("git repository commit capability is not available", nil)
}

func requireRepositoryTreeReader(deps cliutil.CommandDependencies) (repository.RepositoryTreeReader, bool) {
	if deps.Services != nil {
		if candidate, ok := deps.Services.RepositorySync().(repository.RepositoryTreeReader); ok {
			return candidate, true
		}
		if candidate, ok := deps.Services.RepositoryStore().(repository.RepositoryTreeReader); ok {
			return candidate, true
		}
	}
	return nil, false
}

func resolveRepoTreePaths(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	_ *cliutil.GlobalFlags,
) ([]string, error) {
	if treeReader, ok := requireRepositoryTreeReader(deps); ok {
		return treeReader.Tree(ctx)
	}

	return nil, cliutil.ValidationError("repository tree is not supported by the active repository provider", nil)
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
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
) (repositoryContextInfo, error) {
	contexts, err := cliutil.RequireContexts(deps)
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

func selectedContextName(globalFlags *cliutil.GlobalFlags, ctx context.Context) string {
	if globalFlags != nil && strings.TrimSpace(globalFlags.Context) != "" {
		return strings.TrimSpace(globalFlags.Context)
	}
	return strings.TrimSpace(cliutil.ContextName(ctx))
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

	return nil, cliutil.ValidationError(
		fmt.Sprintf("invalid --%s value: use YYYY-MM-DD or RFC3339", flagName),
		nil,
	)
}
