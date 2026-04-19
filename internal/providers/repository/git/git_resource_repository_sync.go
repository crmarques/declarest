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
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

func (r *GitResourceRepository) Refresh(ctx context.Context) error {
	if !r.hasRemote() {
		return nil
	}

	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return err
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}

	auth, err := r.authMethod(ctx)
	if err != nil {
		return err
	}

	proxyOpts, err := r.proxyOptions(ctx)
	if err != nil {
		return err
	}

	fetchErr := repo.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemoteName,
		Auth:       auth,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", r.targetBranch(), defaultRemoteName, r.targetBranch())),
		},
		Force:        true,
		ProxyOptions: proxyOpts,
	})
	if fetchErr != nil && !errors.Is(fetchErr, gogit.NoErrAlreadyUpToDate) {
		return classifyRemoteError("failed to refresh repository from remote", fetchErr)
	}
	return nil
}

func (r *GitResourceRepository) Clean(ctx context.Context) error {
	if err := r.Reset(ctx, repository.ResetPolicy{Hard: true}); err != nil {
		return err
	}

	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return faults.Internal("failed to open git worktree", err)
	}

	if err := worktree.Clean(&gogit.CleanOptions{Dir: true}); err != nil {
		return faults.Internal("failed to clean git worktree", err)
	}
	return nil
}

func (r *GitResourceRepository) Reset(ctx context.Context, policy repository.ResetPolicy) error {
	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return faults.Internal("failed to open git worktree", err)
	}

	mode := gogit.MixedReset
	if policy.Hard {
		mode = gogit.HardReset
	}

	if err := worktree.Reset(&gogit.ResetOptions{Mode: mode}); err != nil {
		return faults.Internal("failed to reset git worktree", err)
	}
	return nil
}

func (r *GitResourceRepository) Push(ctx context.Context, policy repository.PushPolicy) error {
	if !r.hasRemote() {
		return faults.Invalid("push requires remote configuration", nil)
	}

	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return err
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}

	sourceBranch, err := r.currentHeadBranch(repo)
	if err != nil {
		return err
	}
	targetBranch := r.targetBranch()

	auth, err := r.authMethod(ctx)
	if err != nil {
		return err
	}

	proxyOpts, err := r.proxyOptions(ctx)
	if err != nil {
		return err
	}

	pushErr := repo.Push(&gogit.PushOptions{
		RemoteName: defaultRemoteName,
		Auth:       auth,
		Force:      policy.Force,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", sourceBranch, targetBranch)),
		},
		ProxyOptions: proxyOpts,
	})
	if pushErr != nil && !errors.Is(pushErr, gogit.NoErrAlreadyUpToDate) {
		return classifyRemoteError("failed to push repository changes", pushErr)
	}
	return nil
}

func (r *GitResourceRepository) SyncStatus(ctx context.Context) (repository.SyncReport, error) {
	repo, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return repository.SyncReport{}, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return repository.SyncReport{}, faults.Internal("failed to open git worktree", err)
	}
	worktreeStatus, err := worktree.Status()
	if err != nil {
		return repository.SyncReport{}, faults.Internal("failed to inspect git worktree status", err)
	}

	report := repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: !worktreeStatus.IsClean(),
	}

	if !r.hasRemote() {
		return report, nil
	}

	auth, err := r.authMethod(ctx)
	if err != nil {
		return repository.SyncReport{}, err
	}

	proxyOpts, err := r.proxyOptions(ctx)
	if err != nil {
		return repository.SyncReport{}, err
	}

	fetchErr := repo.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemoteName,
		Auth:       auth,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", r.targetBranch(), defaultRemoteName, r.targetBranch())),
		},
		Force:        true,
		ProxyOptions: proxyOpts,
	})
	if fetchErr != nil && !errors.Is(fetchErr, gogit.NoErrAlreadyUpToDate) {
		return repository.SyncReport{}, classifyRemoteError("failed to refresh remote refs for status", fetchErr)
	}

	targetBranch := r.targetBranch()

	localHash, err := r.resolveLocalHash(repo, targetBranch)
	if err != nil {
		return repository.SyncReport{}, err
	}
	remoteHash, err := r.resolveRemoteHash(repo, targetBranch)
	if err != nil {
		return repository.SyncReport{}, err
	}

	ahead, behind, err := r.computeAheadBehind(repo, localHash, remoteHash)
	if err != nil {
		return repository.SyncReport{}, err
	}

	report.Ahead = ahead
	report.Behind = behind
	switch {
	case ahead == 0 && behind == 0:
		report.State = repository.SyncStateUpToDate
	case ahead > 0 && behind == 0:
		report.State = repository.SyncStateAhead
	case ahead == 0 && behind > 0:
		report.State = repository.SyncStateBehind
	default:
		report.State = repository.SyncStateDiverged
	}

	return report, nil
}

func (r *GitResourceRepository) currentHeadBranch(repo *gogit.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", faults.Internal("failed to resolve git head", err)
	}
	if !head.Name().IsBranch() {
		return "", faults.Invalid("cannot push from detached head", nil)
	}
	return head.Name().Short(), nil
}

func (r *GitResourceRepository) resolveLocalHash(repo *gogit.Repository, targetBranch string) (plumbing.Hash, error) {
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(targetBranch), true)
	if err == nil {
		return branchRef.Hash(), nil
	}
	if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.Hash{}, faults.Internal("failed to resolve local branch reference", err)
	}

	headRef, headErr := repo.Head()
	if headErr != nil {
		if errors.Is(headErr, plumbing.ErrReferenceNotFound) {
			return plumbing.ZeroHash, nil
		}
		return plumbing.Hash{}, faults.Internal("failed to resolve local head reference", headErr)
	}
	return headRef.Hash(), nil
}

func (r *GitResourceRepository) resolveRemoteHash(repo *gogit.Repository, targetBranch string) (plumbing.Hash, error) {
	remoteRefName := plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/%s", defaultRemoteName, targetBranch))
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return plumbing.ZeroHash, nil
		}
		return plumbing.Hash{}, faults.Internal("failed to resolve remote branch reference", err)
	}
	return remoteRef.Hash(), nil
}

const maxGraphTraversalDepth = 5000

func (r *GitResourceRepository) computeAheadBehind(repo *gogit.Repository, localHash plumbing.Hash, remoteHash plumbing.Hash) (int, int, error) {
	const (
		markLocal = 1 << iota
		markRemote
	)

	marks := make(map[plumbing.Hash]uint8)
	if err := r.markCommitGraph(repo, localHash, markLocal, marks); err != nil {
		return 0, 0, err
	}
	if err := r.markCommitGraph(repo, remoteHash, markRemote, marks); err != nil {
		return 0, 0, err
	}

	var ahead int
	var behind int
	for _, mark := range marks {
		switch mark {
		case markLocal:
			ahead++
		case markRemote:
			behind++
		}
	}
	return ahead, behind, nil
}

type graphEntry struct {
	hash  plumbing.Hash
	depth int
}

func (r *GitResourceRepository) markCommitGraph(
	repo *gogit.Repository,
	start plumbing.Hash,
	mark uint8,
	marks map[plumbing.Hash]uint8,
) error {
	if start == plumbing.ZeroHash {
		return nil
	}

	stack := []graphEntry{{hash: start, depth: 0}}
	for len(stack) > 0 {
		last := len(stack) - 1
		entry := stack[last]
		stack = stack[:last]

		currentMark := marks[entry.hash]
		if currentMark&mark != 0 {
			continue
		}

		commit, err := repo.CommitObject(entry.hash)
		if err != nil {
			return faults.Internal("failed to load git commit for status", err)
		}
		marks[entry.hash] = currentMark | mark

		if entry.depth < maxGraphTraversalDepth {
			for _, parentHash := range commit.ParentHashes {
				stack = append(stack, graphEntry{hash: parentHash, depth: entry.depth + 1})
			}
		}
	}

	return nil
}
