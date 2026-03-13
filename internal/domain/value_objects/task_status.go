package value_objects

// TaskStatus represents the status of a task (sync, distribute, etc.)
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusSyncing   TaskStatus = "syncing"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

func (s TaskStatus) String() string {
	return string(s)
}

func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed
}
