package entities

import (
	"time"

	sharederrors "github.com/gaodengpan/image-copier/internal/shared/errors"
)

// DistributeStatus represents the status of a distribution task
type DistributeStatus string

const (
	DistributeStatusPending   DistributeStatus = "pending"
	DistributeStatusSyncing   DistributeStatus = "syncing"
	DistributeStatusCompleted DistributeStatus = "completed"
	DistributeStatusFailed    DistributeStatus = "failed"
)

func (s DistributeStatus) String() string {
	return string(s)
}

func (s DistributeStatus) IsTerminal() bool {
	return s == DistributeStatusCompleted || s == DistributeStatusFailed
}

// TargetResult represents the result of distributing to a single target
type TargetResult struct {
	TargetName string
	Success    bool
	Skipped    bool
	Error      error
}

// DistributeTask represents a task to distribute an image to multiple targets
type DistributeTask struct {
	SourceImageID  string
	OriginalSource string
	Arch           string
	Os             string
	Targets        []string // Target names
	Status         DistributeStatus
	Results        []TargetResult
	Error          error
	StartedAt      *time.Time
	CompletedAt    *time.Time
}

// ErrDistributeTaskAlreadyStarted is returned when trying to start an already started task
var ErrDistributeTaskAlreadyStarted = sharederrors.NewDomainError("DistributeTask", "Start", "task already started")

// ErrDistributeTaskNotDistributing is returned when trying to complete a task that is not distributing
var ErrDistributeTaskNotDistributing = sharederrors.NewDomainError("DistributeTask", "Complete", "task is not in distributing status")

// NewDistributeTask creates a new DistributeTask
func NewDistributeTask(sourceImageID, originalSource, arch, os string, targets []string) *DistributeTask {
	return &DistributeTask{
		SourceImageID:  sourceImageID,
		OriginalSource: originalSource,
		Arch:           arch,
		Os:             os,
		Targets:        targets,
		Status:         DistributeStatusPending,
		Results:        make([]TargetResult, 0),
	}
}

// Start marks the task as syncing (phase 1)
func (t *DistributeTask) Start() error {
	if t.Status != DistributeStatusPending {
		return ErrDistributeTaskAlreadyStarted
	}
	now := time.Now()
	t.Status = DistributeStatusSyncing
	t.StartedAt = &now
	return nil
}

// Complete marks the task as completed
func (t *DistributeTask) Complete() error {
	if t.Status != DistributeStatusSyncing {
		return ErrDistributeTaskNotDistributing
	}
	now := time.Now()
	t.Status = DistributeStatusCompleted
	t.CompletedAt = &now
	return nil
}

// Fail marks the task as failed with an error
func (t *DistributeTask) Fail(err error) {
	now := time.Now()
	t.Status = DistributeStatusFailed
	t.Error = err
	t.CompletedAt = &now
}

// AddResult adds a target result to the task
func (t *DistributeTask) AddResult(result TargetResult) {
	t.Results = append(t.Results, result)
}

// DisplayName returns a human-readable name for the task
func (t *DistributeTask) DisplayName() string {
	if t.Arch == "" && t.Os == "" {
		return t.OriginalSource
	}
	return t.OriginalSource + " (" + t.Os + "/" + t.Arch + ")"
}

// Duration returns the duration of the task
func (t *DistributeTask) Duration() time.Duration {
	if t.StartedAt == nil || t.CompletedAt == nil {
		return 0
	}
	return t.CompletedAt.Sub(*t.StartedAt)
}

// HasErrors returns true if any target failed
func (t *DistributeTask) HasErrors() bool {
	for _, r := range t.Results {
		if r.Error != nil {
			return true
		}
	}
	return t.Error != nil
}

// Counts returns success, skipped, and failed counts in a single pass
func (t *DistributeTask) Counts() (success, skipped, failed int) {
	for _, r := range t.Results {
		if r.Error != nil {
			failed++
		} else if r.Skipped {
			skipped++
		} else if r.Success {
			success++
		}
	}
	return
}

// SuccessCount returns the number of successful targets
func (t *DistributeTask) SuccessCount() int {
	success, _, _ := t.Counts()
	return success
}

// SkippedCount returns the number of skipped targets
func (t *DistributeTask) SkippedCount() int {
	_, skipped, _ := t.Counts()
	return skipped
}

// FailedCount returns the number of failed targets
func (t *DistributeTask) FailedCount() int {
	_, _, failed := t.Counts()
	return failed
}
