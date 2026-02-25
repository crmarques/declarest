package repository

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

// ResourceStore manages deterministic local resource persistence operations.
type ResourceStore interface {
	Save(ctx context.Context, logicalPath string, value resource.Value) error
	Get(ctx context.Context, logicalPath string) (resource.Value, error)
	Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error
	List(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
	Exists(ctx context.Context, logicalPath string) (bool, error)
}

// RepositoryCommitter is an optional repository capability used by commands
// that want to create a local VCS commit after mutating repository files.
type RepositoryCommitter interface {
	Commit(ctx context.Context, message string) (bool, error)
}

// RepositoryHistoryReader is an optional repository capability for reading
// local VCS history when supported by the active repository backend.
type RepositoryHistoryReader interface {
	History(ctx context.Context, filter HistoryFilter) ([]HistoryEntry, error)
}

// RepositoryTreeReader is an optional repository capability for reading a
// deterministic directory-only tree of the local repository layout.
type RepositoryTreeReader interface {
	Tree(ctx context.Context) ([]string, error)
}

// RepositoryStatusDetailsReader is an optional repository capability for
// reading worktree file-level change details when supported by the active
// repository backend.
type RepositoryStatusDetailsReader interface {
	WorktreeStatus(ctx context.Context) ([]WorktreeStatusEntry, error)
}

// RepositorySync manages repository lifecycle and synchronization operations.
type RepositorySync interface {
	Init(ctx context.Context) error
	Refresh(ctx context.Context) error
	Clean(ctx context.Context) error
	Reset(ctx context.Context, policy ResetPolicy) error
	Check(ctx context.Context) error
	Push(ctx context.Context, policy PushPolicy) error
	SyncStatus(ctx context.Context) (SyncReport, error)
}
