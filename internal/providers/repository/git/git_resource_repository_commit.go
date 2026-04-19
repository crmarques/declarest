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

package git

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

func (r *GitResourceRepository) Commit(ctx context.Context, message string) (bool, error) {
	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return false, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, faults.Internal("failed to open git worktree", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, faults.Internal("failed to inspect git worktree status", err)
	}
	if status.IsClean() {
		return false, nil
	}

	if err := worktree.AddGlob("."); err != nil {
		return false, faults.Internal("failed to stage git changes", err)
	}

	commitMessage := strings.TrimSpace(message)
	if commitMessage == "" {
		commitMessage = "declarest: update repository resources"
	}

	if _, err := worktree.Commit(commitMessage, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "declarest",
			Email: "declarest@local",
			When:  time.Now(),
		},
	}); err != nil {
		return false, faults.Internal("failed to commit git changes", err)
	}

	return true, nil
}

func (r *GitResourceRepository) History(ctx context.Context, filter repository.HistoryFilter) ([]repository.HistoryEntry, error) {
	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return nil, err
	}

	logOptions := &gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
		Since: filter.Since,
		Until: filter.Until,
	}
	if pathFilter := buildGitHistoryPathFilter(filter.Paths); pathFilter != nil {
		logOptions.PathFilter = pathFilter
	}

	iter, err := repo.Log(logOptions)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return []repository.HistoryEntry{}, nil
		}
		return nil, faults.Internal("failed to read git history", err)
	}

	entriesCap := 0
	if filter.MaxCount > 0 {
		entriesCap = filter.MaxCount
	}
	entries := make([]repository.HistoryEntry, 0, entriesCap)
	authorFilter := strings.ToLower(strings.TrimSpace(filter.Author))
	grepFilter := strings.ToLower(strings.TrimSpace(filter.Grep))

	for {
		commit, nextErr := iter.Next()
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if errors.Is(nextErr, storer.ErrStop) {
				break
			}
			return nil, faults.Internal("failed to iterate git history", nextErr)
		}

		entry := historyEntryFromCommit(commit)
		if !matchesGitHistoryEntryFilter(entry, authorFilter, grepFilter) {
			continue
		}

		entries = append(entries, entry)
		if filter.MaxCount > 0 && len(entries) >= filter.MaxCount {
			break
		}
	}

	if filter.Reverse {
		for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
	}

	return entries, nil
}

func (r *GitResourceRepository) WorktreeStatus(ctx context.Context) ([]repository.WorktreeStatusEntry, error) {
	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, faults.Internal("failed to open git worktree", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, faults.Internal("failed to inspect git worktree status", err)
	}

	paths := make([]string, 0, len(status))
	for changedPath := range status {
		paths = append(paths, changedPath)
	}
	sort.Strings(paths)

	entries := make([]repository.WorktreeStatusEntry, 0, len(paths))
	for _, changedPath := range paths {
		fileStatus := status[changedPath]
		entries = append(entries, repository.WorktreeStatusEntry{
			Path:     changedPath,
			Staging:  gitStatusCodeString(fileStatus.Staging),
			Worktree: gitStatusCodeString(fileStatus.Worktree),
		})
	}
	return entries, nil
}

func buildGitHistoryPathFilter(paths []string) func(string) bool {
	trimmedPaths := make([]string, 0, len(paths))
	for _, raw := range paths {
		value := strings.Trim(strings.TrimSpace(raw), "/")
		if value == "" {
			continue
		}
		trimmedPaths = append(trimmedPaths, value)
	}
	if len(trimmedPaths) == 0 {
		return nil
	}

	return func(changedPath string) bool {
		candidate := strings.Trim(strings.TrimSpace(changedPath), "/")
		for _, prefix := range trimmedPaths {
			if candidate == prefix || strings.HasPrefix(candidate, prefix+"/") {
				return true
			}
		}
		return false
	}
}

func gitStatusCodeString(code gogit.StatusCode) string {
	if code == 0 {
		return " "
	}
	return string(code)
}

func historyEntryFromCommit(commit *object.Commit) repository.HistoryEntry {
	message := strings.ReplaceAll(commit.Message, "\r\n", "\n")
	lines := strings.Split(message, "\n")
	subject := ""
	if len(lines) > 0 {
		subject = strings.TrimSpace(lines[0])
	}

	body := ""
	if len(lines) > 1 {
		body = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}

	return repository.HistoryEntry{
		Hash:    commit.Hash.String(),
		Author:  strings.TrimSpace(commit.Author.Name),
		Email:   strings.TrimSpace(commit.Author.Email),
		Date:    commit.Author.When,
		Subject: subject,
		Body:    body,
	}
}

func matchesGitHistoryEntryFilter(entry repository.HistoryEntry, authorFilter string, grepFilter string) bool {
	if authorFilter != "" {
		authorHaystack := strings.ToLower(strings.TrimSpace(entry.Author + " " + entry.Email))
		if !strings.Contains(authorHaystack, authorFilter) {
			return false
		}
	}

	if grepFilter != "" {
		messageHaystack := strings.ToLower(strings.TrimSpace(entry.Subject + "\n" + entry.Body))
		if !strings.Contains(messageHaystack, grepFilter) {
			return false
		}
	}

	return true
}
