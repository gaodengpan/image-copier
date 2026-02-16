package cli

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// CLIImage implements the WorkItem interface for CLI image processing
type CLIImage struct {
	ImageID string
}

func (img CLIImage) DisplayName() string {
	return img.ImageID
}

// CLIImagesProcessor handles processing of CLI images
type CLIImagesProcessor struct {
	logger *logrus.Logger
}

// Process handles a single CLI image task
func (p *CLIImagesProcessor) Process(ctx context.Context, task WorkerPoolTask[CLIImage], progressMgr *progress.Progress, workerIdx int) error {
	puller := NewPuller(task.Config, p.logger)

	// Set up the stage callback to update progress
	callback := CreateStageCallback(progressMgr, workerIdx, task.Item.DisplayName(), time.Now())
	puller.StageCallback = callback

	return puller.PullSingle(ctx, task.Item.ImageID)
}

// GetStageCallback returns a stage callback function
func (p *CLIImagesProcessor) GetStageCallback(workerIdx int, label string, startTime time.Time) func(core.PullStage, int) {
	// This method signature is constrained by the WorkerPoolProcessor interface
	// So we return nil, and the callback is set directly in Process method
	return nil
}
