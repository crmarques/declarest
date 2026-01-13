package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"declarest/internal/resource"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"golang.org/x/crypto/ssh"
)

type GitResourceRepositoryManager struct {
	fs             *FileSystemResourceRepositoryManager
	config         *GitResourceRepositoryConfig
	resourceFormat ResourceFormat
}

func NewGitResourceRepositoryManager(baseDir string) *GitResourceRepositoryManager {
	return &GitResourceRepositoryManager{
		fs:             NewFileSystemResourceRepositoryManager(baseDir),
		resourceFormat: ResourceFormatJSON,
	}
}

func (m *GitResourceRepositoryManager) SetConfig(cfg *GitResourceRepositoryConfig) {
	if m == nil {
		return
	}
	m.config = cfg
	m.applyConfig()
}

func (m *GitResourceRepositoryManager) SetResourceFormat(format ResourceFormat) {
	if m == nil {
		return
	}
	m.resourceFormat = normalizeResourceFormat(format)
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager("")
	}
	m.fs.SetResourceFormat(m.resourceFormat)
}

func (m *GitResourceRepositoryManager) SetMetadataBaseDir(dir string) {
	if m == nil {
		return
	}
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager("")
	}
	m.fs.SetMetadataBaseDir(dir)
}

func (m *GitResourceRepositoryManager) Init() error {
	if m == nil {
		return errors.New("git resource repository manager is nil")
	}
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager("")
	}
	m.applyConfig()
	return m.fs.Init()
}

func (m *GitResourceRepositoryManager) InitLocalRepository() error {
	if m == nil {
		return errors.New("git resource repository manager is nil")
	}
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager("")
	}
	m.applyConfig()
	if err := m.fs.Init(); err != nil {
		return err
	}
	_, _, err := m.openRepo()
	return err
}

func (m *GitResourceRepositoryManager) IsLocalRepositoryInitialized() (bool, error) {
	if m == nil {
		return false, errors.New("git resource repository manager is nil")
	}
	repo, _, err := m.openRepoIfExists()
	if err != nil {
		return false, err
	}
	return repo != nil, nil
}

func (m *GitResourceRepositoryManager) GetResource(path string) (resource.Resource, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return resource.Resource{}, err
	}
	return fs.GetResource(path)
}

func (m *GitResourceRepositoryManager) CreateResource(path string, res resource.Resource) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}
	if err := fs.CreateResource(path, res); err != nil {
		return err
	}
	return m.commitResourceChange(path, "Create", false)
}

func (m *GitResourceRepositoryManager) UpdateResource(path string, res resource.Resource) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}
	if err := fs.UpdateResource(path, res); err != nil {
		return err
	}
	return m.commitResourceChange(path, "Update", false)
}

func (m *GitResourceRepositoryManager) ApplyResource(path string, res resource.Resource) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}
	if err := fs.ApplyResource(path, res); err != nil {
		return err
	}
	return m.commitResourceChange(path, "Apply", false)
}

func (m *GitResourceRepositoryManager) DeleteResource(path string) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}
	if err := m.commitResourceChange(path, "Delete", true); err != nil {
		return err
	}
	return fs.DeleteResource(path)
}

func (m *GitResourceRepositoryManager) ReadMetadata(path string) (map[string]any, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return nil, err
	}
	return fs.ReadMetadata(path)
}

func (m *GitResourceRepositoryManager) WriteMetadata(path string, metadata map[string]any) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}

	filePath, err := fs.metadataFile(path)
	if err != nil {
		return err
	}

	action := "Update"
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			action = "Add"
		} else {
			return err
		}
	}

	if err := fs.WriteMetadata(path, metadata); err != nil {
		return err
	}
	return m.commitMetadataChange(path, filePath, action, false)
}

func (m *GitResourceRepositoryManager) DeleteMetadata(path string) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}
	filePath, err := fs.metadataFile(path)
	if err != nil {
		return err
	}
	if err := m.commitMetadataChange(path, filePath, "Delete", true); err != nil {
		return err
	}
	return fs.DeleteMetadata(path)
}

func (m *GitResourceRepositoryManager) MoveResourceTree(fromPath, toPath string) error {
	fromPath = resource.NormalizePath(fromPath)
	toPath = resource.NormalizePath(toPath)
	if fromPath == toPath {
		return nil
	}

	fsManager, err := m.ensureFS()
	if err != nil {
		return err
	}

	if err := fsManager.MoveResourceTree(fromPath, toPath); err != nil {
		return err
	}

	repo, repoRoot, err := m.openRepo()
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	fromDir, err := fsManager.resourceDir(fromPath)
	if err != nil {
		return err
	}
	toDir, err := fsManager.resourceDir(toPath)
	if err != nil {
		return err
	}

	relFrom, err := filepath.Rel(repoRoot, fromDir)
	if err != nil {
		return err
	}
	relFrom = filepath.ToSlash(relFrom)

	if err := stageDirectory(wt, repoRoot, toDir); err != nil {
		return err
	}
	if err := removeTrackedPrefix(repo, wt, relFrom); err != nil {
		return err
	}

	changed, err := hasAnyStagedChanges(wt)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	message := fmt.Sprintf("Move resource %s to %s", strings.TrimSpace(fromPath), strings.TrimSpace(toPath))
	signature := m.commitSignature(repo)
	if _, err := wt.Commit(message, &git.CommitOptions{
		Author:    &signature,
		Committer: &signature,
	}); err != nil {
		return err
	}
	return m.autoSyncIfEnabled()
}

func (m *GitResourceRepositoryManager) GetResourceCollection(path string) ([]resource.Resource, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return nil, err
	}
	return fs.GetResourceCollection(path)
}

func (m *GitResourceRepositoryManager) ListResourcePaths() []string {
	fs, err := m.ensureFS()
	if err != nil {
		return nil
	}
	return fs.ListResourcePaths()
}

func (m *GitResourceRepositoryManager) RebaseLocalFromRemote() error {
	repo, _, err := m.openRepoForRemote()
	if err != nil {
		return err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		return err
	}

	if _, err := ensureRemote(repo, settings.remoteName, settings.remoteURL); err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	opts := &git.PullOptions{
		RemoteName:      settings.remoteName,
		RemoteURL:       settings.remoteURL,
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	}

	if branch := resolveBranchName(repo, settings.remoteName, settings.branch); branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}

	if err := wt.Pull(opts); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return err
	}
	return nil
}

func (m *GitResourceRepositoryManager) SyncLocalFromRemoteIfConfigured() error {
	if m == nil {
		return errors.New("git resource repository manager is nil")
	}
	if m.config == nil || m.config.Remote == nil {
		return nil
	}
	if err := m.RebaseLocalFromRemote(); err != nil {
		return m.wrapRepoSyncError(err)
	}
	return nil
}

func (m *GitResourceRepositoryManager) PushLocalDiffsToRemote() error {
	return m.pushLocalDiffsToRemote(false)
}

func (m *GitResourceRepositoryManager) ForcePushLocalDiffsToRemote() error {
	return m.pushLocalDiffsToRemote(true)
}

func (m *GitResourceRepositoryManager) pushLocalDiffsToRemote(force bool) error {
	repo, _, err := m.openRepo()
	if err != nil {
		return err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		return err
	}

	remote, err := ensureRemote(repo, settings.remoteName, settings.remoteURL)
	if err != nil {
		return err
	}

	branch := resolveBranchName(repo, settings.remoteName, settings.branch)
	if branch == "" {
		return errors.New("unable to determine current branch")
	}

	if err := m.prepareRemoteForPush(repo, remote, settings, branch, force); err != nil {
		return err
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	opts := &git.PushOptions{
		RemoteName:      settings.remoteName,
		RemoteURL:       settings.remoteURL,
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
		RefSpecs:        []config.RefSpec{refSpec},
		Force:           force,
	}

	if err := repo.Push(opts); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return err
	}
	return nil
}

func (m *GitResourceRepositoryManager) prepareRemoteForPush(repo *git.Repository, remote *git.Remote, settings gitRemoteSettings, branch string, force bool) error {
	if repo == nil {
		return errors.New("git repository is nil")
	}
	if remote == nil {
		return errors.New("git remote is nil")
	}

	branch = strings.TrimSpace(branch)
	if branch == "" {
		return errors.New("branch is required")
	}

	localRef, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("local branch %s not found", branch)
		}
		return err
	}

	refs, err := remote.List(&git.ListOptions{
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	})
	if err != nil {
		return sanitizeGitError(err)
	}

	remoteHash, ok := findRemoteBranchHash(refs, branch)
	if !ok {
		return nil
	}
	if force {
		return nil
	}

	if err := fetchRemoteBranch(repo, settings, branch); err != nil {
		return err
	}

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName(settings.remoteName, branch), true)
	if err != nil {
		return err
	}
	remoteHash = remoteRef.Hash()
	localHash := localRef.Hash()

	if remoteHash == localHash {
		return nil
	}

	remoteAncestor, err := isAncestor(repo, remoteHash, localHash)
	if err != nil {
		return err
	}
	if remoteAncestor {
		return nil
	}

	localAncestor, err := isAncestor(repo, localHash, remoteHash)
	if err != nil {
		return err
	}
	if localAncestor {
		return repoPushError{
			message: fmt.Sprintf("remote branch %s is ahead of local; push would overwrite remote history", branch),
			hints: []string{
				"declarest repo refresh",
				"declarest repo reset",
			},
		}
	}

	return repoPushError{
		message: fmt.Sprintf("local branch %s has diverged from remote; push would be non-fast-forward", branch),
		hints: []string{
			"declarest repo reset",
			"declarest repo push --force",
			"git merge/rebase",
		},
	}
}

func (m *GitResourceRepositoryManager) ResetLocal() error {
	repo, _, err := m.openRepoForRemote()
	if err != nil {
		return err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		return err
	}

	if _, err := ensureRemote(repo, settings.remoteName, settings.remoteURL); err != nil {
		return err
	}

	branch := resolveBranchName(repo, settings.remoteName, settings.branch)
	if branch == "" {
		return errors.New("unable to determine current branch")
	}

	fetchOpts := &git.FetchOptions{
		RemoteName:      settings.remoteName,
		RemoteURL:       settings.remoteURL,
		Auth:            settings.auth,
		Force:           true,
		InsecureSkipTLS: settings.insecureSkipTLS,
	}

	if err := repo.Fetch(fetchOpts); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	remoteRefName := plumbing.NewRemoteReferenceName(settings.remoteName, branch)
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	localRefName := plumbing.NewBranchReferenceName(branch)
	_, err = repo.Reference(localRefName, true)
	checkout := &git.CheckoutOptions{
		Branch: localRefName,
		Force:  true,
	}
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			checkout.Create = true
			checkout.Hash = remoteRef.Hash()
		} else {
			return err
		}
	}

	if err := wt.Checkout(checkout); err != nil {
		return err
	}

	return wt.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: remoteRef.Hash(),
	})
}

func fetchRemoteBranch(repo *git.Repository, settings gitRemoteSettings, branch string) error {
	if repo == nil {
		return errors.New("git repository is nil")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return errors.New("branch is required")
	}
	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/remotes/%s/%s", branch, settings.remoteName, branch))
	if err := repo.Fetch(&git.FetchOptions{
		RemoteName:      settings.remoteName,
		RemoteURL:       settings.remoteURL,
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
		RefSpecs:        []config.RefSpec{refSpec},
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return sanitizeGitError(err)
	}
	return nil
}

func (GitResourceRepositoryManager) Close() error { return nil }

func (m *GitResourceRepositoryManager) CheckRemoteAccess() (bool, error) {
	if m == nil {
		return false, errors.New("git resource repository manager is nil")
	}

	repo, _, err := m.openRepoIfExists()
	if err != nil {
		return false, err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		if errors.Is(err, errRemoteNotConfigured) {
			return false, nil
		}
		return true, err
	}

	remote, err := m.remoteForCheck(repo, settings)
	if err != nil {
		return true, err
	}

	_, err = remote.List(&git.ListOptions{
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	})
	if err != nil {
		return true, sanitizeGitError(err)
	}
	return true, nil
}

func (m *GitResourceRepositoryManager) InitRemoteIfEmpty() (bool, error) {
	if m == nil {
		return false, errors.New("git resource repository manager is nil")
	}

	repo, _, err := m.openRepo()
	if err != nil {
		return false, err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		if errors.Is(err, errRemoteNotConfigured) {
			return false, nil
		}
		return true, err
	}

	remote, err := ensureRemote(repo, settings.remoteName, settings.remoteURL)
	if err != nil {
		return true, err
	}

	refs, err := remote.List(&git.ListOptions{
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	})
	if err != nil {
		if !errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return true, sanitizeGitError(err)
		}
		refs = nil
	}
	if len(refs) > 0 {
		return true, nil
	}

	branch := resolveBranchName(repo, settings.remoteName, settings.branch)
	if branch == "" {
		branch = "main"
	}
	refName := plumbing.NewBranchReferenceName(branch)

	head, err := repo.Head()
	if err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return true, err
		}
		if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refName)); err != nil {
			return true, err
		}
		wt, err := repo.Worktree()
		if err != nil {
			return true, err
		}
		if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
			return true, err
		}
		signature := m.commitSignature(repo)
		if _, err := wt.Commit("Initialize declarest repository", &git.CommitOptions{
			Author:            &signature,
			Committer:         &signature,
			AllowEmptyCommits: true,
		}); err != nil {
			return true, err
		}
	} else if !head.Name().IsBranch() || head.Name().Short() != branch {
		if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, head.Hash())); err != nil {
			return true, err
		}
		if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refName)); err != nil {
			return true, err
		}
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	if err := repo.Push(&git.PushOptions{
		RemoteName:      settings.remoteName,
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
		RefSpecs:        []config.RefSpec{refSpec},
	}); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return true, nil
		}
		return true, sanitizeGitError(err)
	}
	return true, nil
}

func (m *GitResourceRepositoryManager) CheckRemoteSync() (bool, bool, error) {
	if m == nil {
		return false, false, errors.New("git resource repository manager is nil")
	}

	repo, _, err := m.openRepoIfExists()
	if err != nil {
		return false, false, err
	}

	settings, err := m.remoteSettings(repo)
	if err != nil {
		if errors.Is(err, errRemoteNotConfigured) {
			return false, false, nil
		}
		return true, false, err
	}

	if repo == nil {
		return true, false, errors.New("local git repository is not initialized")
	}

	remote, err := ensureRemote(repo, settings.remoteName, settings.remoteURL)
	if err != nil {
		return true, false, err
	}

	branch := resolveBranchName(repo, settings.remoteName, settings.branch)
	if branch == "" {
		return true, false, errors.New("unable to determine current branch")
	}

	refs, err := remote.List(&git.ListOptions{
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	})
	if err != nil {
		return true, false, sanitizeGitError(err)
	}

	remoteHash, ok := findRemoteBranchHash(refs, branch)
	if !ok {
		return true, false, fmt.Errorf("remote branch %s not found", branch)
	}

	localRef, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return true, false, fmt.Errorf("local branch %s not found", branch)
		}
		return true, false, err
	}

	if localRef.Hash() == remoteHash {
		return true, true, nil
	}
	return true, false, fmt.Errorf("local branch %s is out of sync with remote", branch)
}

func (m *GitResourceRepositoryManager) ensureFS() (*FileSystemResourceRepositoryManager, error) {
	if m == nil {
		return nil, errors.New("git resource repository manager is nil")
	}
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager("")
		m.applyConfig()
	}
	return m.fs, nil
}

func (m *GitResourceRepositoryManager) commitResourceChange(path, action string, deleted bool) error {
	fs, err := m.ensureFS()
	if err != nil {
		return err
	}

	repo, repoRoot, err := m.openRepo()
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	filePaths, err := fs.resourceFilesForDelete(path)
	if err != nil {
		return err
	}

	var primaryFile string
	if !deleted {
		filePath, err := fs.resourceFile(path)
		if err != nil {
			return err
		}
		primaryFile = filePath
	}

	changed := false
	for _, filePath := range filePaths {
		rel, err := filepath.Rel(repoRoot, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if deleted || (primaryFile != "" && filePath != primaryFile) {
			if _, err := wt.Remove(rel); err != nil {
				tracked, trackErr := m.isIndexTracked(repo, rel)
				if trackErr != nil {
					return trackErr
				}
				if !tracked {
					continue
				}
				return err
			}
		} else {
			if _, err := wt.Add(rel); err != nil {
				return err
			}
		}

		relChanged, err := m.hasStagedChanges(wt, rel)
		if err != nil {
			return err
		}
		if relChanged {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	message := fmt.Sprintf("%s resource %s", strings.TrimSpace(action), path)
	signature := m.commitSignature(repo)
	_, err = wt.Commit(message, &git.CommitOptions{
		Author:    &signature,
		Committer: &signature,
	})
	if err != nil {
		return err
	}
	return m.autoSyncIfEnabled()
}

func (m *GitResourceRepositoryManager) commitMetadataChange(path, filePath, action string, deleted bool) error {
	repo, repoRoot, err := m.openRepo()
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || strings.HasPrefix(rel, "../") {
		return nil
	}
	rel = filepath.ToSlash(rel)

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if deleted {
		if _, err := wt.Remove(rel); err != nil {
			tracked, trackErr := m.isIndexTracked(repo, rel)
			if trackErr != nil {
				return trackErr
			}
			if !tracked {
				return nil
			}
			return err
		}
	} else {
		if _, err := wt.Add(rel); err != nil {
			return err
		}
	}

	changed, err := m.hasStagedChanges(wt, rel)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	message := fmt.Sprintf("%s metadata %s", strings.TrimSpace(action), path)
	signature := m.commitSignature(repo)
	_, err = wt.Commit(message, &git.CommitOptions{
		Author:    &signature,
		Committer: &signature,
	})
	if err != nil {
		return err
	}
	return m.autoSyncIfEnabled()
}

func (m *GitResourceRepositoryManager) autoSyncIfEnabled() error {
	if !m.autoSyncEnabled() {
		return nil
	}
	if err := m.PushLocalDiffsToRemote(); err != nil {
		if errors.Is(err, errRemoteNotConfigured) {
			return nil
		}
		return err
	}
	return nil
}

func (m *GitResourceRepositoryManager) autoSyncEnabled() bool {
	if m == nil || m.config == nil || m.config.Remote == nil {
		return false
	}
	if m.config.Remote.AutoSync == nil {
		return true
	}
	return *m.config.Remote.AutoSync
}

func (m *GitResourceRepositoryManager) openRepoForRemote() (*git.Repository, string, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return nil, "", err
	}
	baseDir, err := AbsBaseDir(fs.BaseDir)
	if err != nil {
		return nil, "", err
	}

	repo, err := git.PlainOpenWithOptions(baseDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err == nil {
		return repo, repoRoot(repo, baseDir), nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, "", err
	}

	settings, err := m.remoteSettings(nil)
	if err != nil {
		return nil, "", err
	}

	empty, err := isDirEmpty(baseDir)
	if err != nil {
		return nil, "", err
	}
	if !empty {
		return nil, "", fmt.Errorf("cannot clone remote repository into non-empty directory %s", baseDir)
	}

	cloneOpts := &git.CloneOptions{
		URL:             settings.remoteURL,
		Auth:            settings.auth,
		InsecureSkipTLS: settings.insecureSkipTLS,
	}
	if branch := strings.TrimSpace(settings.branch); branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		cloneOpts.SingleBranch = true
	}

	repo, err = git.PlainClone(baseDir, false, cloneOpts)
	if err != nil {
		return nil, "", fmt.Errorf("clone remote repository: %w", err)
	}
	return repo, repoRoot(repo, baseDir), nil
}

func (m *GitResourceRepositoryManager) openRepo() (*git.Repository, string, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return nil, "", err
	}
	baseDir, err := AbsBaseDir(fs.BaseDir)
	if err != nil {
		return nil, "", err
	}

	repo, err := git.PlainOpenWithOptions(baseDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		if !errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, "", err
		}
		repo, err = git.PlainInit(baseDir, false)
		if err != nil {
			return nil, "", err
		}
	}

	return repo, repoRoot(repo, baseDir), nil
}

func (m *GitResourceRepositoryManager) openRepoIfExists() (*git.Repository, string, error) {
	fs, err := m.ensureFS()
	if err != nil {
		return nil, "", err
	}
	baseDir, err := AbsBaseDir(fs.BaseDir)
	if err != nil {
		return nil, "", err
	}

	repo, err := git.PlainOpenWithOptions(baseDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, baseDir, nil
		}
		return nil, "", err
	}

	return repo, repoRoot(repo, baseDir), nil
}

func (m *GitResourceRepositoryManager) commitSignature(repo *git.Repository) object.Signature {
	name := "Declarest"
	email := "declarest@localhost"
	if repo != nil {
		if cfg, err := repo.Config(); err == nil {
			if strings.TrimSpace(cfg.User.Name) != "" {
				name = cfg.User.Name
			}
			if strings.TrimSpace(cfg.User.Email) != "" {
				email = cfg.User.Email
			}
		}
	}
	return object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

func (m *GitResourceRepositoryManager) hasStagedChanges(wt *git.Worktree, rel string) (bool, error) {
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	entry, ok := status[rel]
	if !ok {
		return false, nil
	}
	return entry.Staging != git.Unmodified, nil
}

func hasAnyStagedChanges(wt *git.Worktree) (bool, error) {
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	for _, entry := range status {
		if entry.Staging != git.Unmodified {
			return true, nil
		}
	}
	return false, nil
}

func stageDirectory(wt *git.Worktree, repoRoot, dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, err := wt.Add(rel); err != nil {
			return err
		}
		return nil
	})
}

func removeTrackedPrefix(repo *git.Repository, wt *git.Worktree, prefix string) error {
	idx, err := repo.Storer.Index()
	if err != nil {
		return err
	}
	prefix = filepath.ToSlash(strings.TrimSpace(prefix))
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, entry := range idx.Entries {
		name := filepath.ToSlash(entry.Name)
		if prefix == "" || strings.HasPrefix(name, prefix) {
			if _, err := wt.Remove(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *GitResourceRepositoryManager) isIndexTracked(repo *git.Repository, rel string) (bool, error) {
	if repo == nil {
		return false, errors.New("git repository is nil")
	}
	index, err := repo.Storer.Index()
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range index.Entries {
		if entry.Name == rel {
			return true, nil
		}
	}
	return false, nil
}

func repoRoot(repo *git.Repository, fallback string) string {
	root := fallback
	if repo == nil {
		return root
	}
	if wt, err := repo.Worktree(); err == nil {
		type rooter interface {
			Root() string
		}
		if fsRoot, ok := wt.Filesystem.(rooter); ok {
			if candidate := strings.TrimSpace(fsRoot.Root()); candidate != "" {
				root = candidate
			}
		}
	}
	return root
}

func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

type RepoSyncError struct {
	repoDir string
	cause   error
}

func (e RepoSyncError) Error() string {
	target := "git repository"
	if strings.TrimSpace(e.repoDir) != "" {
		target = fmt.Sprintf("git repository at %s", e.repoDir)
	}
	msg := fmt.Sprintf("failed to sync %s from remote", target)
	if e.cause != nil {
		detail := redactGitCredentials(strings.TrimSpace(e.cause.Error()))
		if detail != "" {
			msg = fmt.Sprintf("%s: %s", msg, detail)
		}
	}
	return fmt.Sprintf("%s\nTry:\n  declarest repo reset\n  declarest repo push --force", msg)
}

func (e RepoSyncError) Unwrap() error {
	return e.cause
}

type repoPushError struct {
	message string
	hints   []string
}

func (e repoPushError) Error() string {
	msg := strings.TrimSpace(e.message)
	if msg == "" {
		msg = "failed to push repository changes"
	}

	var hints []string
	for _, hint := range e.hints {
		trimmed := strings.TrimSpace(hint)
		if trimmed != "" {
			hints = append(hints, trimmed)
		}
	}
	if len(hints) == 0 {
		return msg
	}
	return fmt.Sprintf("%s\nTry:\n  %s", msg, strings.Join(hints, "\n  "))
}

var gitURLCredentialPattern = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)([^/@\s]+@)`)
var errRemoteNotConfigured = errors.New("git remote url is not configured")
var errAncestorFound = errors.New("ancestor found")

func redactGitCredentials(message string) string {
	if message == "" {
		return ""
	}
	return gitURLCredentialPattern.ReplaceAllString(message, "$1***@")
}

func sanitizeGitError(err error) error {
	if err == nil {
		return nil
	}
	detail := redactGitCredentials(strings.TrimSpace(err.Error()))
	if detail == "" || detail == err.Error() {
		return err
	}
	return fmt.Errorf("%s", detail)
}

func (m *GitResourceRepositoryManager) applyConfig() {
	if m == nil || m.config == nil || m.config.Local == nil {
		return
	}
	baseDir := strings.TrimSpace(m.config.Local.BaseDir)
	if baseDir == "" {
		return
	}
	if m.fs == nil {
		m.fs = NewFileSystemResourceRepositoryManager(baseDir)
		m.fs.SetResourceFormat(m.resourceFormat)
		return
	}
	m.fs.BaseDir = baseDir
	m.fs.SetResourceFormat(m.resourceFormat)
}

func (m *GitResourceRepositoryManager) wrapRepoSyncError(err error) error {
	repoDir := ""
	if m != nil && m.fs != nil {
		if dir, dirErr := AbsBaseDir(m.fs.BaseDir); dirErr == nil {
			repoDir = dir
		}
	}
	return RepoSyncError{
		repoDir: repoDir,
		cause:   err,
	}
}

type gitRemoteSettings struct {
	remoteName      string
	remoteURL       string
	branch          string
	auth            transport.AuthMethod
	insecureSkipTLS bool
}

func (m *GitResourceRepositoryManager) remoteSettings(repo *git.Repository) (gitRemoteSettings, error) {
	settings := gitRemoteSettings{remoteName: git.DefaultRemoteName}

	if m != nil && m.config != nil && m.config.Remote != nil {
		remoteCfg := m.config.Remote
		settings.remoteURL = strings.TrimSpace(remoteCfg.URL)
		settings.branch = strings.TrimSpace(remoteCfg.Branch)
		if remoteCfg.TLS != nil {
			settings.insecureSkipTLS = remoteCfg.TLS.InsecureSkipVerify
		}
	}

	if settings.remoteURL == "" && repo != nil {
		if remote, err := repo.Remote(settings.remoteName); err == nil {
			if cfg := remote.Config(); cfg != nil && len(cfg.URLs) > 0 {
				settings.remoteURL = cfg.URLs[len(cfg.URLs)-1]
			}
		}
	}

	if settings.branch == "" {
		settings.branch = currentBranchName(repo)
	}

	if m != nil && m.config != nil && m.config.Remote != nil {
		auth, err := authFromConfig(m.config.Remote, settings.remoteURL)
		if err != nil {
			return settings, err
		}
		settings.auth = auth
	}

	if settings.remoteURL == "" {
		return settings, errRemoteNotConfigured
	}
	return settings, nil
}

func ensureRemote(repo *git.Repository, name, url string) (*git.Remote, error) {
	if repo == nil {
		return nil, errors.New("git repository is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = git.DefaultRemoteName
	}
	url = strings.TrimSpace(url)

	remote, err := repo.Remote(name)
	if err == nil {
		if url != "" {
			cfg := remote.Config()
			if cfg == nil || len(cfg.URLs) == 0 || cfg.URLs[0] != url {
				if err := repo.DeleteRemote(name); err != nil && !errors.Is(err, git.ErrRemoteNotFound) {
					return nil, err
				}
				return repo.CreateRemote(&config.RemoteConfig{
					Name: name,
					URLs: []string{url},
				})
			}
		}
		return remote, nil
	}

	if !errors.Is(err, git.ErrRemoteNotFound) {
		return nil, err
	}
	if url == "" {
		return nil, errRemoteNotConfigured
	}
	return repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
}

func (m *GitResourceRepositoryManager) remoteForCheck(repo *git.Repository, settings gitRemoteSettings) (*git.Remote, error) {
	if repo != nil {
		return ensureRemote(repo, settings.remoteName, settings.remoteURL)
	}
	if strings.TrimSpace(settings.remoteURL) == "" {
		return nil, errRemoteNotConfigured
	}
	store := memory.NewStorage()
	return git.NewRemote(store, &config.RemoteConfig{
		Name: settings.remoteName,
		URLs: []string{settings.remoteURL},
	}), nil
}

func findRemoteBranchHash(refs []*plumbing.Reference, branch string) (plumbing.Hash, bool) {
	if branch == "" {
		return plumbing.ZeroHash, false
	}
	target := plumbing.NewBranchReferenceName(branch).String()
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if ref.Name().String() == target {
			return ref.Hash(), true
		}
	}
	return plumbing.ZeroHash, false
}

func isAncestor(repo *git.Repository, ancestor, descendant plumbing.Hash) (bool, error) {
	if repo == nil {
		return false, errors.New("git repository is nil")
	}
	if ancestor == descendant {
		return true, nil
	}
	iter, err := repo.Log(&git.LogOptions{From: descendant})
	if err != nil {
		return false, err
	}
	defer iter.Close()

	err = iter.ForEach(func(commit *object.Commit) error {
		if commit.Hash == ancestor {
			return errAncestorFound
		}
		return nil
	})
	if errors.Is(err, errAncestorFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func resolveBranchName(repo *git.Repository, remoteName, configured string) string {
	name := normalizeBranchName(configured, remoteName)
	if name != "" {
		return name
	}
	name = currentBranchName(repo)
	if name != "" {
		return name
	}
	if repo == nil {
		return ""
	}
	refName := plumbing.NewRemoteReferenceName(remoteName, "HEAD")
	ref, err := repo.Reference(refName, true)
	if err != nil || ref == nil {
		return ""
	}
	if ref.Type() != plumbing.SymbolicReference {
		return ""
	}
	target := ref.Target()
	if target == "" {
		return ""
	}
	return normalizeBranchName(target.String(), remoteName)
}

func normalizeBranchName(branch, remoteName string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	if strings.HasPrefix(branch, "refs/heads/") {
		return strings.TrimPrefix(branch, "refs/heads/")
	}
	if strings.HasPrefix(branch, "refs/remotes/") {
		branch = strings.TrimPrefix(branch, "refs/remotes/")
		parts := strings.SplitN(branch, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return parts[0]
	}
	if remoteName != "" && strings.HasPrefix(branch, remoteName+"/") {
		return strings.TrimPrefix(branch, remoteName+"/")
	}
	return branch
}

func currentBranchName(repo *git.Repository) string {
	if repo == nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	if head.Name().IsBranch() {
		return head.Name().Short()
	}
	return ""
}

func authFromConfig(cfg *GitResourceRepositoryRemoteConfig, remoteURL string) (transport.AuthMethod, error) {
	if cfg == nil || cfg.Auth == nil {
		if cfg != nil {
			if _, err := normalizeGitProvider(cfg.Provider); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	provider, err := normalizeGitProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}

	count := 0
	if cfg.Auth.BasicAuth != nil {
		count++
	}
	if cfg.Auth.SSH != nil {
		count++
	}
	if cfg.Auth.AccessKey != nil {
		count++
	}
	if count > 1 {
		return nil, errors.New("multiple git auth methods configured")
	}

	if cfg.Auth.SSH != nil {
		return sshAuthFromConfig(cfg.Auth.SSH, remoteURL)
	}
	if cfg.Auth.BasicAuth != nil {
		return basicAuthFromConfig(cfg.Auth.BasicAuth)
	}
	if cfg.Auth.AccessKey != nil {
		return accessKeyAuthFromConfig(cfg.Auth.AccessKey, provider)
	}
	return nil, nil
}

func basicAuthFromConfig(cfg *GitResourceRepositoryBasicAuthConfig) (transport.AuthMethod, error) {
	if cfg == nil {
		return nil, nil
	}
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	if username == "" && password == "" {
		return nil, nil
	}
	if username == "" {
		return nil, errors.New("git basic auth requires username")
	}
	return &githttp.BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

func accessKeyAuthFromConfig(cfg *GitResourceRepositoryAccessKeyConfig, provider string) (transport.AuthMethod, error) {
	if cfg == nil {
		return nil, nil
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("git access key token is required")
	}
	if provider != "" {
		if username := accessKeyUsernameForProvider(provider); username != "" {
			return &githttp.BasicAuth{
				Username: username,
				Password: token,
			}, nil
		}
	}
	return &githttp.TokenAuth{Token: token}, nil
}

func sshAuthFromConfig(cfg *GitResourceRepositorySSHAuthConfig, remoteURL string) (transport.AuthMethod, error) {
	if cfg == nil {
		return nil, nil
	}
	keyFile := strings.TrimSpace(cfg.PrivateKeyFile)
	if keyFile == "" {
		return nil, errors.New("git ssh auth requires private_key_file")
	}

	userName := strings.TrimSpace(cfg.User)
	if userName == "" {
		userName = sshUserFromURL(remoteURL)
	}
	if userName == "" {
		userName = defaultSSHUser()
	}
	if userName == "" {
		return nil, errors.New("git ssh auth requires user")
	}

	auth, err := gitssh.NewPublicKeysFromFile(userName, keyFile, cfg.Passphrase)
	if err != nil {
		return nil, err
	}

	if cfg.InsecureIgnoreHostKey {
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else if knownHosts := strings.TrimSpace(cfg.KnownHostsFile); knownHosts != "" {
		callback, err := gitssh.NewKnownHostsCallback(knownHosts)
		if err != nil {
			return nil, err
		}
		auth.HostKeyCallback = callback
	}

	return auth, nil
}

func normalizeGitProvider(provider string) (string, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return "", nil
	}
	switch provider {
	case "github", "gitlab", "gitea":
		return provider, nil
	default:
		return "", fmt.Errorf("unsupported git provider %q", provider)
	}
}

func accessKeyUsernameForProvider(provider string) string {
	switch provider {
	case "github":
		return "x-access-token"
	case "gitlab":
		return "oauth2"
	case "gitea":
		return "token"
	default:
		return ""
	}
}

func sshUserFromURL(remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return ""
	}
	if strings.Contains(remoteURL, "://") {
		parsed, err := url.Parse(remoteURL)
		if err != nil || parsed == nil || parsed.User == nil {
			return ""
		}
		return parsed.User.Username()
	}
	if at := strings.Index(remoteURL, "@"); at > 0 {
		return remoteURL[:at]
	}
	return ""
}

func defaultSSHUser() string {
	if u, err := user.Current(); err == nil && u != nil && strings.TrimSpace(u.Username) != "" {
		return u.Username
	}
	if name := strings.TrimSpace(os.Getenv("USER")); name != "" {
		return name
	}
	return ""
}
