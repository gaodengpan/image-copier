package cli

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// WorkItem represents a unit of work to be processed
type WorkItem interface {
	DisplayName() string // Human-readable display name for progress tracking
}

// WorkerPoolTask represents a task to be executed by the worker pool
type WorkerPoolTask[T any] struct {
	Index  int            // Index in the original work slice
	Item   T              // The actual work item to process
	Config *config.Config // Configuration for this task
}

// WorkerPoolProcessor defines the interface for processing tasks
type WorkerPoolProcessor[T any] interface {
	Process(ctx context.Context, task WorkerPoolTask[T], progressMgr *progress.Progress, workerIdx int) error
}
