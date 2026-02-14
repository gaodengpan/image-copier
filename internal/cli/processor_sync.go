package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// SyncTask implements the WorkItem interface for sync task processing
type SyncTask struct {
	Source string
	Arch   string
	Os     string
}

func (t SyncTask) DisplayName() string {
	return fmt.Sprintf("%s (%s/%s)", t.Source, t.Os, t.Arch)
}

// SyncTasksProcessor handles processing of sync tasks from YAML manifests
type SyncTasksProcessor struct {
	logger *logrus.Logger
	force  bool
}

// Process handles a single sync task
func (p *SyncTasksProcessor) Process(ctx context.Context, task WorkerPoolTask[SyncTask], progressMgr *progress.Progress, workerIdx int) error {
	// Create task-specific config with specific arch/os
	taskCfg := *task.Config
	taskCfg.RegistryArch = task.Item.Arch
	taskCfg.RegistryOs = task.Item.Os
	taskCfg.Force = p.force // Respect the force flag from processor

	puller := core.NewPuller(&taskCfg, p.logger)

	// Set up the stage callback to update progress
	callback := CreateStageCallback(progressMgr, workerIdx, task.Item.DisplayName(), time.Now())
	puller.StageCallback = callback

	return puller.PullSingle(ctx, task.Item.Source)
}

// GetStageCallback returns a stage callback function
func (p *SyncTasksProcessor) GetStageCallback(workerIdx int, label string, startTime time.Time) func(core.PullStage, int) {
	// This method signature is constrained by the WorkerPoolProcessor interface
	// So we return nil, and the callback is set directly in Process method
	return nil
}