package use_cases

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/utils/concurrency"
)

// SyncCommandUseCaseImpl implements the two-phase sync command
type SyncCommandUseCaseImpl struct {
	registryClient output.RegistryClient
	githubClient   output.GitHubClientWithRetry
	logger         output.Logger
	imageIDService output.ImageIDService
	syncConfig     output.SyncConfig
	targetBuilder  output.DistributionTargetBuilder
	distributor    output.MultiTargetDistributor
}

// NewSyncCommandUseCase creates a new SyncCommandUseCase
func NewSyncCommandUseCase(
	registryClient output.RegistryClient,
	githubClient output.GitHubClientWithRetry,
	logger output.Logger,
	imageIDService output.ImageIDService,
	syncConfig output.SyncConfig,
	targetBuilder output.DistributionTargetBuilder,
	distributor output.MultiTargetDistributor,
) *SyncCommandUseCaseImpl {
	return &SyncCommandUseCaseImpl{
		registryClient: registryClient,
		githubClient:   githubClient,
		logger:         logger,
		imageIDService: imageIDService,
		syncConfig:     syncConfig,
		targetBuilder:  targetBuilder,
		distributor:    distributor,
	}
}

// Execute executes the full two-phase sync command
func (uc *SyncCommandUseCaseImpl) Execute(ctx context.Context, in input.SyncCommandInput) (*input.SyncCommandResult, error) {
	startTime := time.Now()

	// Build sync tasks from input
	tasks := uc.buildSyncTasks(in)

	result := &input.SyncCommandResult{
		SyncPhase:       &input.SyncPhaseResult{},
		DistributePhase: &input.DistributePhaseResult{},
	}

	// Phase 1: Sync to staging registry
	if !in.SkipSync {
		syncResult, err := uc.executeSyncPhase(ctx, tasks, in)
		if err != nil {
			return nil, err
		}
		result.SyncPhase = syncResult
	} else {
		// When skipping sync, treat all tasks as already existed
		result.SyncPhase.AlreadyExisted = tasks
	}

	// Phase 2: Distribute to targets
	if !in.SkipDistribute {
		// Build distribute tasks from synced images
		distributeTasks, skippedInSync := uc.buildDistributeTasks(result.SyncPhase, in)
		distributeResult, err := uc.executeDistributePhase(ctx, distributeTasks, skippedInSync, in)
		if err != nil {
			return nil, err
		}
		result.DistributePhase = distributeResult

		// Notify task complete for failed sync tasks (they are not included in distribute tasks)
		if in.TaskComplete != nil {
			for _, task := range result.SyncPhase.Failed {
				in.TaskComplete(task.Source, false, task.Error)
			}
		}
	} else {
		// When skipping distribute, notify task complete for all sync results
		if in.TaskComplete != nil {
			for _, task := range result.SyncPhase.AlreadyExisted {
				in.TaskComplete(task.Source, true, nil) // skipped = true
			}
			for _, task := range result.SyncPhase.NewlySynced {
				in.TaskComplete(task.Source, false, nil) // skipped = false
			}
			for _, task := range result.SyncPhase.Failed {
				in.TaskComplete(task.Source, false, task.Error)
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// SyncPhase executes only the sync phase
func (uc *SyncCommandUseCaseImpl) SyncPhase(ctx context.Context, in input.SyncCommandInput) (*input.SyncPhaseResult, error) {
	tasks := uc.buildSyncTasks(in)
	return uc.executeSyncPhase(ctx, tasks, in)
}

// DistributePhase executes only the distribute phase
func (uc *SyncCommandUseCaseImpl) DistributePhase(ctx context.Context, syncedImages []string, in input.SyncCommandInput) (*input.DistributePhaseResult, error) {
	// Build distribute tasks from synced images
	tasks := make([]*entities.DistributeTask, len(syncedImages))
	for i, imageID := range syncedImages {
		tasks[i] = entities.NewDistributeTask(imageID, imageID, in.Arch, in.Os, in.Targets)
	}
	// When running distribute phase alone, all images are considered newly synced (not skipped)
	return uc.executeDistributePhase(ctx, tasks, make(map[string]bool), in)
}

// buildSyncTasks builds sync tasks from input
func (uc *SyncCommandUseCaseImpl) buildSyncTasks(in input.SyncCommandInput) []*entities.SyncTask {
	tasks := make([]*entities.SyncTask, len(in.Images))
	for i, image := range in.Images {
		tasks[i] = entities.NewSyncTask(image, image, in.Arch, in.Os)
	}
	return tasks
}

// executeSyncPhase executes Phase 1: sync to staging registry
func (uc *SyncCommandUseCaseImpl) executeSyncPhase(ctx context.Context, tasks []*entities.SyncTask, in input.SyncCommandInput) (*input.SyncPhaseResult, error) {
	uc.logger.Infof("Phase 1: Syncing %d images to staging registry", len(tasks))

	result := &input.SyncPhaseResult{
		AlreadyExisted: make([]*entities.SyncTask, 0),
		NewlySynced:    make([]*entities.SyncTask, 0),
		Failed:         make([]*entities.SyncTask, 0),
		Errors:         make([]error, 0),
	}

	// Diff phase: check which images need sync
	diffResults := uc.diffStagingRegistry(ctx, tasks, in)

	// Separate tasks into already existed and needs sync
	var needsSync []*entities.SyncTask
	for _, diff := range diffResults {
		if diff.RemoteExists && !in.Force {
			result.AlreadyExisted = append(result.AlreadyExisted, diff.Task)
		} else {
			needsSync = append(needsSync, diff.Task)
		}
		if diff.Error != nil {
			result.Errors = append(result.Errors, diff.Error)
		}
	}

	uc.logger.Infof("Diff complete: %d already existed, %d need sync", len(result.AlreadyExisted), len(needsSync))

	if in.DryRun {
		uc.logger.Infof("[dry-run] Would sync %d images to staging registry", len(needsSync))
		// In dry-run mode, treat needsSync as newly synced for reporting
		result.NewlySynced = needsSync
		return result, nil
	}

	// Sync phase: trigger GitHub Actions for images that need sync
	if len(needsSync) > 0 {
		uc.syncToStaging(ctx, needsSync, in, result)
	}

	return result, nil
}

// diffResult represents the result of checking an image in staging registry
type diffResult struct {
	Task         *entities.SyncTask
	RemoteExists bool
	Error        error
}

// diffStagingRegistry checks which images exist in the staging registry
func (uc *SyncCommandUseCaseImpl) diffStagingRegistry(ctx context.Context, tasks []*entities.SyncTask, in input.SyncCommandInput) []diffResult {
	rawResults := concurrency.CollectResults(concurrency.ParallelForEach(ctx, tasks, in.WorkerCount,
		func(ctx context.Context, task *entities.SyncTask) (diffResult, error) {
			// Notify progress: checking stage
			if in.ProgressCallback != nil {
				in.ProgressCallback(task.Source, value_objects.SyncStageChecking, "", 0)
			}

			sourceID := uc.imageIDService.NormalizeSourceID(task.Source)
			destID := uc.registryClient.BuildDestImageID(output.BuildDestOptions{
				SourceID:          sourceID,
				RegistryHost:      uc.syncConfig.StagingRegistryHost(),
				RegistryNamespace: uc.syncConfig.StagingRegistryNamespace(),
			})

			uc.logger.Debugf("Checking image existence: %s -> %s", sourceID, destID)
			exists, err := uc.registryClient.CheckImageExists(ctx, output.RegistryAuthOptions{
				ImageID:  destID,
				Username: uc.syncConfig.StagingRegistryUsername(),
				Password: uc.syncConfig.StagingRegistryPassword(),
			})
			if err != nil {
				uc.logger.Warn("Failed to check image existence for ", destID, ": ", err)
			}
			if exists {
				uc.logger.Debugf("Image exists in staging registry: %s", destID)
			} else {
				uc.logger.Debugf("Image does not exist in staging registry: %s", destID)
			}

			// Notify progress: checking complete (100%)
			if in.ProgressCallback != nil {
				in.ProgressCallback(task.Source, value_objects.SyncStageChecking, "", 100)
			}

			return diffResult{
				Task:         task,
				RemoteExists: exists,
				Error:        err,
			}, nil
		}))

	// Extract values from Result wrapper
	results := make([]diffResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = r.Value
	}
	return results
}

// syncToStaging triggers GitHub Actions to sync images to staging registry
// Uses semaphore to limit concurrent operations to workerCount
func (uc *SyncCommandUseCaseImpl) syncToStaging(ctx context.Context, tasks []*entities.SyncTask, in input.SyncCommandInput, result *input.SyncPhaseResult) {
	uc.logger.Infof("Syncing %d images to staging registry with %d workers", len(tasks), in.WorkerCount)

	var mu sync.Mutex

	_ = concurrency.ParallelForEachSimple(ctx, tasks, in.WorkerCount, func(ctx context.Context, task *entities.SyncTask) error {
		if err := uc.syncSingleImageToStaging(ctx, task, in); err != nil {
			mu.Lock()
			// Add to Failed list
			_ = task.Fail(err)
			result.Failed = append(result.Failed, task)
			result.Errors = append(result.Errors, fmt.Errorf("failed to sync %s: %w", task.Source, err))
			mu.Unlock()
			return err
		}
		// Add to NewlySynced list on success
		mu.Lock()
		result.NewlySynced = append(result.NewlySynced, task)
		mu.Unlock()
		return nil
	})
}

// syncSingleImageToStaging syncs a single image to the staging registry
// Note: Image existence is already checked in diffStagingRegistry phase,
// so we don't need to check again here.
func (uc *SyncCommandUseCaseImpl) syncSingleImageToStaging(ctx context.Context, task *entities.SyncTask, in input.SyncCommandInput) error {
	sourceID := uc.imageIDService.NormalizeSourceID(task.Source)
	destID := uc.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          sourceID,
		RegistryHost:      uc.syncConfig.StagingRegistryHost(),
		RegistryNamespace: uc.syncConfig.StagingRegistryNamespace(),
	})

	arch := task.Arch
	if arch == "" {
		arch = uc.syncConfig.DefaultArch()
	}
	osType := task.Os
	if osType == "" {
		osType = uc.syncConfig.DefaultOS()
	}

	// Notify progress: sync stage started (0%)
	if in.ProgressCallback != nil {
		in.ProgressCallback(task.Source, value_objects.SyncStageSyncing, "", 0)
	}

	// Trigger GitHub Actions workflow
	// Note: diffStagingRegistry already checked existence, so we proceed directly
	uc.logger.Infof("Triggering GitHub Actions for %s -> %s", sourceID, destID)
	runID, err := uc.githubClient.TriggerWorkflowWithRetry(ctx, sourceID, destID, arch, osType)
	if err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}

	// Notify progress: sync stage in progress (50%)
	if in.ProgressCallback != nil {
		in.ProgressCallback(task.Source, value_objects.SyncStageSyncing, "", 50)
	}

	// Wait for workflow to complete
	uc.logger.Infof("Waiting for workflow %s to complete", runID)
	if err := uc.githubClient.WaitForWorkflowSimple(ctx, runID); err != nil {
		return fmt.Errorf("workflow failed: %w", err)
	}

	// Notify progress: sync stage complete (100%)
	if in.ProgressCallback != nil {
		in.ProgressCallback(task.Source, value_objects.SyncStageSyncing, "", 100)
	}

	uc.logger.Infof("Successfully synced %s to staging registry", sourceID)
	return nil
}

// buildDistributeTasks builds distribute tasks from sync result
// Returns tasks and a set of image sources that were skipped in sync phase
func (uc *SyncCommandUseCaseImpl) buildDistributeTasks(syncResult *input.SyncPhaseResult, in input.SyncCommandInput) ([]*entities.DistributeTask, map[string]bool) {
	// Collect all synced image IDs
	allImages := make([]*entities.SyncTask, 0, len(syncResult.AlreadyExisted)+len(syncResult.NewlySynced))
	allImages = append(allImages, syncResult.AlreadyExisted...)
	allImages = append(allImages, syncResult.NewlySynced...)

	// Track which images were skipped in sync phase
	skippedInSync := make(map[string]bool)
	for _, task := range syncResult.AlreadyExisted {
		skippedInSync[task.Source] = true
	}

	tasks := make([]*entities.DistributeTask, len(allImages))
	for i, syncTask := range allImages {
		// Use normalized source ID as SourceImageID - the distribution strategies
		// will build the full staging registry path using BuildDestImageID
		sourceID := uc.imageIDService.NormalizeSourceID(syncTask.Source)

		uc.logger.Debugf("Building distribute task: %s -> %s", syncTask.Source, sourceID)
		tasks[i] = entities.NewDistributeTask(sourceID, syncTask.Source, syncTask.Arch, syncTask.Os, in.Targets)
	}
	return tasks, skippedInSync
}

// executeDistributePhase executes Phase 2: distribute to targets
// Uses semaphore to limit concurrent operations to workerCount
func (uc *SyncCommandUseCaseImpl) executeDistributePhase(ctx context.Context, tasks []*entities.DistributeTask, skippedInSync map[string]bool, in input.SyncCommandInput) (*input.DistributePhaseResult, error) {
	targets := uc.syncConfig.GetDistributionTargets(in.Targets)

	if len(targets) == 0 {
		uc.logger.Info("No distribution targets specified, skipping distribute phase")
		// When no distribution targets, notify task complete for all tasks
		if in.TaskComplete != nil {
			for _, task := range tasks {
				// Check if this task was skipped in sync phase
				wasSkippedInSync := skippedInSync[task.OriginalSource]
				in.TaskComplete(task.OriginalSource, wasSkippedInSync, nil)
			}
		}
		return &input.DistributePhaseResult{}, nil
	}

	uc.logger.Infof("Phase 2: Distributing %d images to %d targets with %d workers", len(tasks), len(targets), in.WorkerCount)

	result := &input.DistributePhaseResult{
		Tasks:  tasks,
		Errors: make([]input.TargetError, 0),
	}

	// Build distribution targets
	distributionTargets := uc.targetBuilder.BuildTargets(targets)

	var mu sync.Mutex

	_ = concurrency.ParallelForEachSimple(ctx, tasks, in.WorkerCount, func(ctx context.Context, t *entities.DistributeTask) error {
		// Start the task
		if err := t.Start(); err != nil {
			mu.Lock()
			result.Errors = append(result.Errors, input.TargetError{
				ImageName:  t.OriginalSource,
				TargetName: "internal",
				Error:      err,
			})
			// Notify task complete with error
			if in.TaskComplete != nil {
				in.TaskComplete(t.OriginalSource, false, err)
			}
			mu.Unlock()
			return err
		}

		// Notify progress: distribution stage started (0%)
		if in.ProgressCallback != nil {
			in.ProgressCallback(t.OriginalSource, value_objects.SyncStageDistributing, "", 0)
		}

		// Use distributor to distribute to all targets
		distResult := uc.distributor.DistributeToAll(
			ctx, t, distributionTargets,
			output.StagingRegistryConfig{
				Host:      uc.syncConfig.StagingRegistryHost(),
				Namespace: uc.syncConfig.StagingRegistryNamespace(),
				Username:  uc.syncConfig.StagingRegistryUsername(),
				Password:  uc.syncConfig.StagingRegistryPassword(),
			},
			in.Force,
		)

		mu.Lock()
		// Add results to task
		totalTargets := len(distResult.Results)
		for i, r := range distResult.Results {
			t.AddResult(r)
			if r.Error != nil {
				result.Errors = append(result.Errors, input.TargetError{
					ImageName:  t.OriginalSource,
					TargetName: r.TargetName,
					Error:      r.Error,
				})
			}
			// Notify progress for each target with cumulative percentage
			if in.ProgressCallback != nil {
				percent := float64(i+1) * 100.0 / float64(totalTargets)
				in.ProgressCallback(t.OriginalSource, value_objects.SyncStageDistributing, r.TargetName, percent)
			}
		}

		// Update task state based on results
		var taskErr error
		if t.HasErrors() {
			taskErr = fmt.Errorf("distribution completed with errors")
			t.Fail(taskErr)
		} else {
			if completeErr := t.Complete(); completeErr != nil {
				uc.logger.Warn("Failed to mark task as complete: ", completeErr)
			}
		}

		// Notify task complete
		if in.TaskComplete != nil {
			in.TaskComplete(t.OriginalSource, false, taskErr)
		}
		mu.Unlock()
		return nil
	})

	// Calculate totals
	for _, t := range tasks {
		result.SuccessCount += t.SuccessCount()
		result.SkippedCount += t.SkippedCount()
		result.FailedCount += t.FailedCount()
	}

	uc.logger.Infof("Distribution complete: %d success, %d skipped, %d failed",
		result.SuccessCount, result.SkippedCount, result.FailedCount)

	return result, nil
}

// Ensure SyncCommandUseCaseImpl implements the interface
var _ input.SyncCommandUseCase = (*SyncCommandUseCaseImpl)(nil)
