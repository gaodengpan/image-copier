package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// WorkItem represents a unit of work to be processed
type WorkItem interface {
	DisplayName() string // Human-readable display name for progress tracking
}

// WorkerPoolTask represents a task to be executed by the worker pool
type WorkerPoolTask[T any] struct {
	Index  int          // Index in the original work slice
	Item   T            // The actual work item to process
	Config *core.Config // Configuration for this task
}

// WorkerPoolProcessor defines the interface for processing tasks
type WorkerPoolProcessor[T any] interface {
	Process(ctx context.Context, task WorkerPoolTask[T], progressMgr *progress.Progress, workerIdx int) error
	GetStageCallback(workerIdx int, label string, startTime time.Time) func(core.PullStage, int)
}

// GenericWorkerPool executes tasks concurrently with progress tracking and error handling
func GenericWorkerPool[T WorkItem](logger *logrus.Logger, tasks []WorkerPoolTask[T],
	processor WorkerPoolProcessor[T], progressMgr *progress.Progress, workerCount int, verbose bool, ctx context.Context) (int, error) {

	// Validate and optimize worker count
	if workerCount < 1 {
		workerCount = 1
	}

	// Adjust worker count based on number of tasks and CPU cores
	maxFromTasks := len(tasks)
	if maxFromTasks == 0 {
		maxFromTasks = 1
	}

	if workerCount > maxFromTasks {
		workerCount = maxFromTasks
	}

	// Limit worker count by CPU cores to prevent excessive resource usage
	maxWorkersByCPU := runtime.NumCPU()
	if workerCount > maxWorkersByCPU {
		logger.Debugf("Reducing worker count from %d to %d based on CPU cores", workerCount, maxWorkersByCPU)
		workerCount = maxWorkersByCPU
	}

	// Route logger output based on verbose flag
	if verbose {
		logger.SetOutput(progressMgr.LogWriter())
	} else {
		logger.SetOutput(io.Discard)
	}
	defer logger.SetOutput(os.Stderr)

	// Create worker pool
	jobs := make(chan int, len(tasks))
	for i := range tasks {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var failCount atomic.Int32

	// Start workers — each worker owns a fixed worker bar
	for i := 0; i < workerCount; i++ {
		workerIdx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				select {
				case <-ctx.Done():
					// Context cancelled, exit early
					return
				default:
					// Continue with task processing
				}

				task := tasks[idx]

				// Update progress status
				progressMgr.UpdateStatus(idx, progress.StatusRunning, nil)
				startTime := time.Now()

				// Execute the task
				err := processor.Process(ctx, task, progressMgr, workerIdx)
				elapsed := time.Since(startTime)

				// Handle the result
				if err != nil {
					if errors.Is(err, core.ErrSkipped) {
						progressMgr.UpdateStatus(idx, progress.StatusSkipped, nil)
					} else if errors.Is(err, core.ErrDryRun) {
						progressMgr.UpdateStatus(idx, progress.StatusDryRun, nil)
					} else {
						progressMgr.UpdateStatus(idx, progress.StatusFailed, err)
						failCount.Add(1)
					}
				} else {
					progressMgr.UpdateStatus(idx, progress.StatusCompleted, nil)
				}

				progressMgr.SetDuration(idx, elapsed)
				progressMgr.UpdateWorker(workerIdx, "")
				progressMgr.Increment()
			}
		}()
	}

	wg.Wait()
	progressMgr.Wait()

	numFailed := int(failCount.Load())
	if numFailed > 0 {
		return numFailed, errors.New("some tasks failed")
	}
	return numFailed, nil
}
