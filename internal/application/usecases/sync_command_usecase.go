package use_cases

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
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
		distributeTasks := uc.buildDistributeTasks(result.SyncPhase, in)
		distributeResult, err := uc.executeDistributePhase(ctx, distributeTasks, in)
		if err != nil {
			return nil, err
		}
		result.DistributePhase = distributeResult
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
	return uc.executeDistributePhase(ctx, tasks, in)
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
	diffResults := uc.diffStagingRegistry(ctx, tasks, in.WorkerCount)

	for _, diff := range diffResults {
		if diff.RemoteExists && !in.Force {
			result.AlreadyExisted = append(result.AlreadyExisted, diff.Task)
		} else {
			result.NewlySynced = append(result.NewlySynced, diff.Task)
		}
		if diff.Error != nil {
			result.Errors = append(result.Errors, diff.Error)
		}
	}

	uc.logger.Infof("Diff complete: %d already existed, %d need sync", len(result.AlreadyExisted), len(result.NewlySynced))

	if in.DryRun {
		uc.logger.Infof("[dry-run] Would sync %d images to staging registry", len(result.NewlySynced))
		return result, nil
	}

	// Sync phase: trigger GitHub Actions for images that need sync
	if len(result.NewlySynced) > 0 {
		uc.syncToStaging(ctx, result.NewlySynced, in, result)
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
func (uc *SyncCommandUseCaseImpl) diffStagingRegistry(ctx context.Context, tasks []*entities.SyncTask, workerCount int) []diffResult {
	results := make([]diffResult, len(tasks))

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, task *entities.SyncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sourceID := uc.imageIDService.NormalizeSourceID(task.Source)
			destID := uc.registryClient.BuildDestImageID(sourceID, uc.syncConfig.StagingRegistryHost(), uc.syncConfig.StagingRegistryNamespace())

			uc.logger.Debugf("Checking image existence: %s -> %s", sourceID, destID)
			exists, err := uc.registryClient.CheckImageExists(ctx, destID, uc.syncConfig.StagingRegistryUsername(), uc.syncConfig.StagingRegistryPassword())
			if err != nil {
				uc.logger.Warn("Failed to check image existence for ", destID, ": ", err)
			}
			if exists {
				uc.logger.Debugf("Image exists in staging registry: %s", destID)
			} else {
				uc.logger.Debugf("Image does not exist in staging registry: %s", destID)
			}

			results[idx] = diffResult{
				Task:         task,
				RemoteExists: exists,
				Error:        err,
			}
		}(i, t)
	}

	wg.Wait()
	return results
}

// syncToStaging triggers GitHub Actions to sync images to staging registry
// Uses semaphore to limit concurrent operations to workerCount
func (uc *SyncCommandUseCaseImpl) syncToStaging(ctx context.Context, tasks []*entities.SyncTask, in input.SyncCommandInput, result *input.SyncPhaseResult) {
	uc.logger.Infof("Syncing %d images to staging registry with %d workers", len(tasks), in.WorkerCount)

	sem := make(chan struct{}, in.WorkerCount)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t *entities.SyncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := uc.syncSingleImageToStaging(ctx, t); err != nil {
				mu.Lock()
				// Remove from NewlySynced and add to Failed
				result.NewlySynced = removeTask(result.NewlySynced, t.Source)
				t.Fail(err) // Set error on task
				result.Failed = append(result.Failed, t)
				result.Errors = append(result.Errors, fmt.Errorf("failed to sync %s: %w", t.Source, err))
				mu.Unlock()
			}
			// If successful, task is already in NewlySynced from diff phase
		}(task)
	}

	wg.Wait()
}

// removeTask removes a task from a slice by source identifier
func removeTask(tasks []*entities.SyncTask, source string) []*entities.SyncTask {
	for i, t := range tasks {
		if t.Source == source {
			return append(tasks[:i], tasks[i+1:]...)
		}
	}
	return tasks
}

// syncSingleImageToStaging syncs a single image to the staging registry
// Note: Image existence is already checked in diffStagingRegistry phase,
// so we don't need to check again here.
func (uc *SyncCommandUseCaseImpl) syncSingleImageToStaging(ctx context.Context, task *entities.SyncTask) error {
	sourceID := uc.imageIDService.NormalizeSourceID(task.Source)
	destID := uc.registryClient.BuildDestImageID(sourceID, uc.syncConfig.StagingRegistryHost(), uc.syncConfig.StagingRegistryNamespace())

	arch := task.Arch
	if arch == "" {
		arch = uc.syncConfig.DefaultArch()
	}
	osType := task.Os
	if osType == "" {
		osType = uc.syncConfig.DefaultOS()
	}

	// Trigger GitHub Actions workflow
	// Note: diffStagingRegistry already checked existence, so we proceed directly
	uc.logger.Infof("Triggering GitHub Actions for %s -> %s", sourceID, destID)
	runID, err := uc.githubClient.TriggerWorkflowWithRetry(ctx, sourceID, destID, arch, osType)
	if err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}

	// Wait for workflow to complete
	uc.logger.Infof("Waiting for workflow %s to complete", runID)
	if err := uc.githubClient.WaitForWorkflowSimple(ctx, runID); err != nil {
		return fmt.Errorf("workflow failed: %w", err)
	}

	uc.logger.Infof("Successfully synced %s to staging registry", sourceID)
	return nil
}

// buildDistributeTasks builds distribute tasks from sync result
func (uc *SyncCommandUseCaseImpl) buildDistributeTasks(syncResult *input.SyncPhaseResult, in input.SyncCommandInput) []*entities.DistributeTask {
	// Collect all synced image IDs
	allImages := make([]*entities.SyncTask, 0, len(syncResult.AlreadyExisted)+len(syncResult.NewlySynced))
	allImages = append(allImages, syncResult.AlreadyExisted...)
	allImages = append(allImages, syncResult.NewlySynced...)

	tasks := make([]*entities.DistributeTask, len(allImages))
	for i, syncTask := range allImages {
		// Use normalized source ID as SourceImageID - the distribution strategies
		// will build the full staging registry path using BuildDestImageID
		sourceID := uc.imageIDService.NormalizeSourceID(syncTask.Source)

		uc.logger.Debugf("Building distribute task: %s -> %s", syncTask.Source, sourceID)
		tasks[i] = entities.NewDistributeTask(sourceID, syncTask.Source, syncTask.Arch, syncTask.Os, in.Targets)
	}
	return tasks
}

// executeDistributePhase executes Phase 2: distribute to targets
// Uses semaphore to limit concurrent operations to workerCount
func (uc *SyncCommandUseCaseImpl) executeDistributePhase(ctx context.Context, tasks []*entities.DistributeTask, in input.SyncCommandInput) (*input.DistributePhaseResult, error) {
	targets := uc.syncConfig.GetDistributionTargets(in.Targets)

	if len(targets) == 0 {
		uc.logger.Info("No distribution targets specified, skipping distribute phase")
		return &input.DistributePhaseResult{}, nil
	}

	uc.logger.Infof("Phase 2: Distributing %d images to %d targets with %d workers", len(tasks), len(targets), in.WorkerCount)

	result := &input.DistributePhaseResult{
		Tasks:  tasks,
		Errors: make([]input.TargetError, 0),
	}

	// Build distribution targets
	distributionTargets := uc.targetBuilder.BuildTargets(targets)

	// Distribute in parallel with semaphore to limit concurrency
	sem := make(chan struct{}, in.WorkerCount)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t *entities.DistributeTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Start the task
			if err := t.Start(); err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, input.TargetError{
					ImageName:  t.OriginalSource,
					TargetName: "internal",
					Error:      err,
				})
				mu.Unlock()
				return
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
			for _, r := range distResult.Results {
				t.AddResult(r)
				if r.Error != nil {
					result.Errors = append(result.Errors, input.TargetError{
						ImageName:  t.OriginalSource,
						TargetName: r.TargetName,
						Error:      r.Error,
					})
				}
			}

			// Update task state based on results
			if t.HasErrors() {
				t.Fail(fmt.Errorf("distribution completed with errors"))
			} else {
				t.Complete()
			}
			mu.Unlock()
		}(task)
	}

	wg.Wait()

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
