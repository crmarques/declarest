package repository

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
