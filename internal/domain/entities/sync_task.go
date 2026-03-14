package entities

import (
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

type SyncTask struct {
	ID          string
	Source      string
	Arch        string
	Os          string
	Status      value_objects.TaskStatus
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
		Status: value_objects.TaskStatusPending,
	}
}

func (t *SyncTask) Start() error {
	if t.Status != value_objects.TaskStatusPending {
		return ErrTaskAlreadyStarted
	}
	now := time.Now()
	t.Status = value_objects.TaskStatusSyncing
	t.StartedAt = &now
	return nil
}

func (t *SyncTask) Complete() error {
	if t.Status != value_objects.TaskStatusSyncing {
		return ErrTaskNotSyncing
	}
	now := time.Now()
	t.Status = value_objects.TaskStatusCompleted
	t.CompletedAt = &now
	return nil
}

func (t *SyncTask) Fail(err error) error {
	now := time.Now()
	t.Status = value_objects.TaskStatusFailed
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
