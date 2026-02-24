package entities

type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusSyncing   SyncStatus = "syncing"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

func (s SyncStatus) String() string {
	return string(s)
}

func (s SyncStatus) IsTerminal() bool {
	return s == SyncStatusCompleted || s == SyncStatusFailed
}
