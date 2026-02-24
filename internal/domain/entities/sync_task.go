package entities

import "time"

type SyncTask struct {
	ID          string
	Source      string
	Arch        string
	Os          string
	Status      SyncStatus
	Error       error
	StartedAt   *time.Time
	CompletedAt *time.Time
}

func NewSyncTask(id, source, arch, os string) *SyncTask {
	return &SyncTask{
		ID:     id,
		Source: source,
		Arch:   arch,
		Os:     os,
		Status: SyncStatusPending,
	}
}

func (t *SyncTask) Start() error {
	if t.Status != SyncStatusPending {
		return ErrTaskAlreadyStarted
	}
	now := time.Now()
	t.Status = SyncStatusSyncing
	t.StartedAt = &now
	return nil
}

func (t *SyncTask) Complete() error {
	if t.Status != SyncStatusSyncing {
		return ErrTaskNotSyncing
	}
	now := time.Now()
	t.Status = SyncStatusCompleted
	t.CompletedAt = &now
	return nil
}

func (t *SyncTask) Fail(err error) error {
	now := time.Now()
	t.Status = SyncStatusFailed
	t.Error = err
	t.CompletedAt = &now
	return nil
}

func (t *SyncTask) DisplayName() string {
	if t.Arch == "" && t.Os == "" {
		return t.Source
	}
	return t.Source + " (" + t.Os + "/" + t.Arch + ")"
}

func (t *SyncTask) Duration() time.Duration {
	if t.StartedAt == nil || t.CompletedAt == nil {
		return 0
	}
	return t.CompletedAt.Sub(*t.StartedAt)
}
