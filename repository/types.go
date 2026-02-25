package repository

import "time"

type ResetPolicy struct {
	Hard bool
}

type PushPolicy struct {
	Force bool
}

type ListPolicy struct {
	Recursive bool
}

type DeletePolicy struct {
	Recursive bool
}

type SyncState string

const (
	SyncStateUpToDate SyncState = "up_to_date"
	SyncStateAhead    SyncState = "ahead"
	SyncStateBehind   SyncState = "behind"
	SyncStateDiverged SyncState = "diverged"
	SyncStateNoRemote SyncState = "no_remote"
)

type SyncReport struct {
	State          SyncState
	Ahead          int
	Behind         int
	HasUncommitted bool
}

type WorktreeStatusEntry struct {
	Path     string `json:"path" yaml:"path"`
	Staging  string `json:"staging" yaml:"staging"`
	Worktree string `json:"worktree" yaml:"worktree"`
}

type HistoryFilter struct {
	MaxCount int
	Author   string
	Grep     string
	Since    *time.Time
	Until    *time.Time
	Paths    []string
	Reverse  bool
}

type HistoryEntry struct {
	Hash    string    `json:"hash" yaml:"hash"`
	Author  string    `json:"author" yaml:"author"`
	Email   string    `json:"email" yaml:"email"`
	Date    time.Time `json:"date" yaml:"date"`
	Subject string    `json:"subject" yaml:"subject"`
	Body    string    `json:"body,omitempty" yaml:"body,omitempty"`
}
