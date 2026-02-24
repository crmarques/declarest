package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/providers/repository/fsstore"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	sshauth "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

var _ repository.ResourceStore = (*GitResourceRepository)(nil)
var _ repository.RepositorySync = (*GitResourceRepository)(nil)

const (
	defaultRemoteName = "origin"
	defaultBranchName = "main"
)

type GitResourceRepository struct {
	local   *fsstore.LocalResourceRepository
	baseDir string
	remote  *config.GitRemote
}

func NewGitResourceRepository(repoConfig config.GitRepository, resourceFormat string) *GitResourceRepository {
	return &GitResourceRepository{
		local:   fsstore.NewLocalResourceRepository(repoConfig.Local.BaseDir, resourceFormat),
		baseDir: repoConfig.Local.BaseDir,
		remote:  repoConfig.Remote,
	}
}

func (r *GitResourceRepository) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	return r.local.Save(ctx, logicalPath, value)
}

func (r *GitResourceRepository) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	return r.local.Get(ctx, logicalPath)
}

func (r *GitResourceRepository) Delete(ctx context.Context, logicalPath string, policy repository.DeletePolicy) error {
	return r.local.Delete(ctx, logicalPath, policy)
}

func (r *GitResourceRepository) List(ctx context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	return r.local.List(ctx, logicalPath, policy)
}

func (r *GitResourceRepository) Exists(ctx context.Context, logicalPath string) (bool, error) {
	return r.local.Exists(ctx, logicalPath)
}

// Deprecated: Move is a concrete helper and is not part of the repository
// interfaces. Prefer interface-based flows for new call sites.
func (r *GitResourceRepository) Move(ctx context.Context, fromPath string, toPath string) error {
	return r.local.Move(ctx, fromPath, toPath)
}

func (r *GitResourceRepository) Init(ctx context.Context) error {
	if err := r.local.Init(ctx); err != nil {
		return err
	}

	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if !errors.Is(err, gogit.ErrRepositoryNotExists) {
			return internalError("failed to open repository", err)
		}
		repo, err = gogit.PlainInit(r.baseDir, false)
		if err != nil {
			return internalError("failed to initialize git repository", err)
		}
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}
	return nil
}

func (r *GitResourceRepository) Refresh(context.Context) error {
	if !r.hasRemote() {
		return nil
	}

	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return notFoundError("git repository not initialized")
		}
		return internalError("failed to open git repository", err)
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}

	auth, err := r.authMethod()
	if err != nil {
		return err
	}

	fetchErr := repo.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemoteName,
		Auth:       auth,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", r.targetBranch(), defaultRemoteName, r.targetBranch())),
		},
		Force: true,
	})
	if fetchErr != nil && !errors.Is(fetchErr, gogit.NoErrAlreadyUpToDate) {
		return classifyRemoteError("failed to refresh repository from remote", fetchErr)
	}
	return nil
}

func (r *GitResourceRepository) Reset(_ context.Context, policy repository.ResetPolicy) error {
	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return notFoundError("git repository not initialized")
		}
		return internalError("failed to open git repository", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return internalError("failed to open git worktree", err)
	}

	mode := gogit.MixedReset
	if policy.Hard {
		mode = gogit.HardReset
	}

	if err := worktree.Reset(&gogit.ResetOptions{Mode: mode}); err != nil {
		return internalError("failed to reset git worktree", err)
	}
	return nil
}

func (r *GitResourceRepository) Check(ctx context.Context) error {
	if err := r.local.Check(ctx); err != nil {
		return err
	}

	_, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return notFoundError("git repository not initialized")
		}
		return internalError("failed to open git repository", err)
	}
	return nil
}

func (r *GitResourceRepository) Push(_ context.Context, policy repository.PushPolicy) error {
	if !r.hasRemote() {
		return validationError("push requires remote configuration", nil)
	}

	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return notFoundError("git repository not initialized")
		}
		return internalError("failed to open git repository", err)
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}

	sourceBranch, err := r.currentHeadBranch(repo)
	if err != nil {
		return err
	}
	targetBranch := r.targetBranch()

	auth, err := r.authMethod()
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
	})
	if pushErr != nil && !errors.Is(pushErr, gogit.NoErrAlreadyUpToDate) {
		return classifyRemoteError("failed to push repository changes", pushErr)
	}
	return nil
}

func (r *GitResourceRepository) SyncStatus(_ context.Context) (repository.SyncReport, error) {
	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return repository.SyncReport{}, notFoundError("git repository not initialized")
		}
		return repository.SyncReport{}, internalError("failed to open git repository", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return repository.SyncReport{}, internalError("failed to open git worktree", err)
	}
	worktreeStatus, err := worktree.Status()
	if err != nil {
		return repository.SyncReport{}, internalError("failed to inspect git worktree status", err)
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

	auth, err := r.authMethod()
	if err != nil {
		return repository.SyncReport{}, err
	}

	fetchErr := repo.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemoteName,
		Auth:       auth,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", r.targetBranch(), defaultRemoteName, r.targetBranch())),
		},
		Force: true,
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

func (r *GitResourceRepository) ensureRemote(repo *gogit.Repository) error {
	if !r.hasRemote() {
		return nil
	}

	_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: defaultRemoteName,
		URLs: []string{r.remote.URL},
	})
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return internalError("failed to configure git remote", err)
	}

	cfg, cfgErr := repo.Config()
	if cfgErr != nil {
		return internalError("failed to load git config", cfgErr)
	}
	cfg.Remotes[defaultRemoteName] = &gitcfg.RemoteConfig{
		Name: defaultRemoteName,
		URLs: []string{r.remote.URL},
	}
	if setErr := repo.Storer.SetConfig(cfg); setErr != nil {
		return internalError("failed to update git remote config", setErr)
	}
	return nil
}

func (r *GitResourceRepository) authMethod() (transport.AuthMethod, error) {
	if r.remote == nil || r.remote.Auth == nil {
		return nil, nil
	}

	auth := r.remote.Auth
	switch {
	case auth.BasicAuth != nil:
		return &httpauth.BasicAuth{
			Username: auth.BasicAuth.Username,
			Password: auth.BasicAuth.Password,
		}, nil
	case auth.AccessKey != nil:
		return &httpauth.BasicAuth{
			Username: "token",
			Password: auth.AccessKey.Token,
		}, nil
	case auth.SSH != nil:
		username := auth.SSH.User
		if username == "" {
			username = "git"
		}

		sshKeys, err := sshauth.NewPublicKeysFromFile(username, auth.SSH.PrivateKeyFile, auth.SSH.Passphrase)
		if err != nil {
			return nil, faults.NewTypedError(faults.AuthError, "failed to load git ssh auth configuration", nil)
		}
		return sshKeys, nil
	default:
		return nil, validationError("git remote auth configuration is invalid", nil)
	}
}

func (r *GitResourceRepository) currentHeadBranch(repo *gogit.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", internalError("failed to resolve git head", err)
	}
	if !head.Name().IsBranch() {
		return "", validationError("cannot push from detached head", nil)
	}
	return head.Name().Short(), nil
}

func (r *GitResourceRepository) resolveLocalHash(repo *gogit.Repository, targetBranch string) (plumbing.Hash, error) {
	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(targetBranch), true)
	if err == nil {
		return branchRef.Hash(), nil
	}
	if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.Hash{}, internalError("failed to resolve local branch reference", err)
	}

	headRef, headErr := repo.Head()
	if headErr != nil {
		if errors.Is(headErr, plumbing.ErrReferenceNotFound) {
			return plumbing.ZeroHash, nil
		}
		return plumbing.Hash{}, internalError("failed to resolve local head reference", headErr)
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
		return plumbing.Hash{}, internalError("failed to resolve remote branch reference", err)
	}
	return remoteRef.Hash(), nil
}

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

func (r *GitResourceRepository) collectCommitSet(repo *gogit.Repository, start plumbing.Hash) (map[plumbing.Hash]struct{}, error) {
	set := make(map[plumbing.Hash]struct{})
	if start == plumbing.ZeroHash {
		return set, nil
	}

	stack := []plumbing.Hash{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		hash := stack[last]
		stack = stack[:last]

		if _, seen := set[hash]; seen {
			continue
		}

		commit, err := repo.CommitObject(hash)
		if err != nil {
			return nil, internalError("failed to load git commit for status", err)
		}
		set[hash] = struct{}{}
		stack = append(stack, commit.ParentHashes...)
	}

	return set, nil
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

	stack := []plumbing.Hash{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		hash := stack[last]
		stack = stack[:last]

		currentMark := marks[hash]
		if currentMark&mark != 0 {
			continue
		}

		commit, err := repo.CommitObject(hash)
		if err != nil {
			return internalError("failed to load git commit for status", err)
		}
		marks[hash] = currentMark | mark
		stack = append(stack, commit.ParentHashes...)
	}

	return nil
}

func (r *GitResourceRepository) hasRemote() bool {
	return r.remote != nil && strings.TrimSpace(r.remote.URL) != ""
}

func (r *GitResourceRepository) targetBranch() string {
	if r.remote != nil && strings.TrimSpace(r.remote.Branch) != "" {
		return strings.TrimSpace(r.remote.Branch)
	}
	return defaultBranchName
}

func countSetDifference(source map[plumbing.Hash]struct{}, target map[plumbing.Hash]struct{}) int {
	count := 0
	for hash := range source {
		if _, exists := target[hash]; !exists {
			count++
		}
	}
	return count
}

func classifyRemoteError(message string, err error) error {
	lower := strings.ToLower(err.Error())

	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired) ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "permission denied"):
		return faults.NewTypedError(faults.AuthError, message, nil)
	case strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "fetch first") ||
		strings.Contains(lower, "rejected"):
		return faults.NewTypedError(faults.ConflictError, message, nil)
	case strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "tls") ||
		strings.Contains(lower, "connection") ||
		strings.Contains(lower, "network"):
		return faults.NewTypedError(faults.TransportError, message, nil)
	default:
		return faults.NewTypedError(faults.InternalError, message, nil)
	}
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
