package use_cases

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
)

type DiffResult struct {
	Task         entities.SyncTask
	RemoteExists bool
	LocalExists  bool
	RemoteError  error
	LocalError   error
}

type SyncImagesUseCaseImpl struct {
	dockerClient     output.DockerClient
	registryClient   output.RegistryClient
	githubClient     output.GitHubClientWithRetry
	fileSystem       output.FileSystem
	httpClient       output.HTTPClient
	logger           output.Logger
	systemClient     output.SystemClient
	imageIDService   *services.ImageIDService
	imageValidator   *validators.ImageValidator
	githubOwner      string
	githubRepo       string
	githubToken      string
	githubWorkflowID string
	cfg              *SyncImagesConfig
}

type SyncImagesConfig struct {
	*config.Config
}

func NewSyncImagesUseCase(
	dockerClient output.DockerClient,
	registryClient output.RegistryClient,
	githubClient output.GitHubClientWithRetry,
	fileSystem output.FileSystem,
	httpClient output.HTTPClient,
	logger output.Logger,
	systemClient output.SystemClient,
	imageIDService *services.ImageIDService,
	cfg SyncImagesConfig,
) *SyncImagesUseCaseImpl {
	return &SyncImagesUseCaseImpl{
		dockerClient:     dockerClient,
		registryClient:   registryClient,
		githubClient:     githubClient,
		fileSystem:       fileSystem,
		httpClient:       httpClient,
		logger:           logger,
		systemClient:     systemClient,
		imageIDService:   imageIDService,
		imageValidator:   validators.NewImageValidator(),
		githubOwner:      cfg.Config.Github.Owner,
		githubRepo:       cfg.Config.Github.Repo,
		githubToken:      cfg.Config.Github.Token,
		githubWorkflowID: cfg.Config.Github.WorkflowID,
		cfg:              &cfg,
	}
}

func (uc *SyncImagesUseCaseImpl) Execute(ctx context.Context, input SyncImagesInput) (
	synced []entities.SyncTask, needsSync []entities.SyncTask, err error,
) {
	uc.logger.Infof("Starting sync for %d images", len(input.Tasks))

	results := uc.diffPhase(ctx, input.Tasks, input.WorkerCount)
	synced, needsSync = uc.partitionResults(results, input.Force, input)

	uc.logger.Infof("Diff complete: %d synced, %d need sync", len(synced), len(needsSync))

	if input.DryRun {
		uc.logger.Infof("[dry-run] Would sync %d images", len(needsSync))
		return synced, needsSync, nil
	}

	if len(needsSync) == 0 {
		return synced, needsSync, nil
	}

	err = uc.syncPhase(ctx, needsSync, input)
	return synced, needsSync, err
}

func (uc *SyncImagesUseCaseImpl) Diff(ctx context.Context, tasks []entities.SyncTask, workerCount int, force bool) (
	synced []entities.SyncTask, needsSync []entities.SyncTask, err error,
) {
	results := uc.diffPhase(ctx, tasks, workerCount)
	synced, needsSync = uc.partitionResults(results, force, SyncImagesInput{})
	return synced, needsSync, nil
}

func (uc *SyncImagesUseCaseImpl) Sync(ctx context.Context, tasks []entities.SyncTask, workerCount int) error {
	if len(tasks) == 0 {
		return nil
	}

	input := SyncImagesInput{
		Tasks:       tasks,
		WorkerCount: workerCount,
		Force:       uc.cfg.Force,
		DryRun:      uc.cfg.DryRun,
	}

	return uc.syncPhase(ctx, tasks, input)
}

func (uc *SyncImagesUseCaseImpl) diffPhase(ctx context.Context, tasks []entities.SyncTask, workerCount int) []DiffResult {
	results := make([]DiffResult, len(tasks))

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, task entities.SyncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			sourceID := task.Source
			destID := uc.registryClient.BuildDestImageID(sourceID, uc.cfg.Config.Registry.Host, uc.cfg.Config.Registry.Namespace)
			remoteExists, remoteErr := uc.registryClient.CheckImageExists(ctx, destID, uc.cfg.Config.Registry.Username, uc.cfg.Config.Registry.Password)
			localExists, localErr := uc.dockerClient.ImageExists(ctx, task.Source)

			if remoteErr != nil {
				uc.logger.Warn("Failed to check remote image", destID, remoteErr)
			}
			if localErr != nil {
				uc.logger.Warn("Failed to check local image", task.Source, localErr)
			}

			results[idx] = DiffResult{
				Task:         task,
				RemoteExists: remoteExists,
				LocalExists:  localExists,
				RemoteError:  remoteErr,
				LocalError:   localErr,
			}
		}(i, t)
	}

	wg.Wait()
	return results
}

func (uc *SyncImagesUseCaseImpl) partitionResults(results []DiffResult, force bool, input SyncImagesInput) (synced, needsSync []entities.SyncTask) {
	for _, r := range results {
		shouldSync := !r.LocalExists || force

		if !shouldSync && input.TargetType == output.SyncTargetRegistry && input.TargetStrategy != nil {
			shouldSync = true
		}

		if shouldSync {
			needsSync = append(needsSync, r.Task)
		} else {
			synced = append(synced, r.Task)
		}
	}
	return synced, needsSync
}

func (uc *SyncImagesUseCaseImpl) syncPhase(ctx context.Context, tasks []entities.SyncTask, input SyncImagesInput) error {
	if len(tasks) == 0 {
		return nil
	}

	uc.logger.Infof("Starting sync phase for %d images with %d workers", len(tasks), input.WorkerCount)

	sem := make(chan struct{}, input.WorkerCount)
	var wg sync.WaitGroup
	var taskIdx int64 = 0
	var failCount int32 = 0

	for i := 0; i < input.WorkerCount; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			for {
				idx := int(atomic.AddInt64(&taskIdx, 1) - 1)
				if idx >= len(tasks) {
					break
				}

				sem <- struct{}{}
				defer func() { <-sem }()

				select {
				case <-ctx.Done():
					return
				default:
				}

				task := tasks[idx]
				err := uc.processSingleImage(ctx, task, input)
				if err != nil {
					uc.logger.Errorf("Failed to sync %s: %v", task.Source, err)
					atomic.AddInt32(&failCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if failCount > 0 {
		return fmt.Errorf("%d image(s) failed to sync", failCount)
	}

	return nil
}

func (uc *SyncImagesUseCaseImpl) processSingleImage(ctx context.Context, task entities.SyncTask, input SyncImagesInput) error {
	uc.logger.Infof("Processing image: %s", task.Source)

	if input.TargetType == output.SyncTargetRegistry && input.TargetStrategy != nil {
		return uc.syncToPrivateRegistry(ctx, task, input)
	}

	stageCallback := func(stage PullStage, polls int) {}

	useCase := NewPullSingleImageUseCase(
		uc.dockerClient,
		uc.registryClient,
		uc.githubClient,
		uc.fileSystem,
		uc.httpClient,
		uc.logger,
		uc.systemClient,
		uc.imageIDService,
		uc.githubOwner,
		uc.githubRepo,
		uc.githubToken,
		uc.githubWorkflowID,
		stageCallback,
	)

	_, err := useCase.Execute(ctx, PullSingleImageInput{
		ImageID:      task.Source,
		RegistryHost: uc.cfg.Config.Registry.Host,
		RegistryUser: uc.cfg.Config.Registry.Username,
		RegistryPass: uc.cfg.Config.Registry.Password,
		RegistryNS:   uc.cfg.Config.Registry.Namespace,
		RegistryArch: task.Arch,
		RegistryOs:   task.Os,
		Force:        uc.cfg.Config.Force,
		DryRun:       uc.cfg.Config.DryRun,
	})

	return err
}

func (uc *SyncImagesUseCaseImpl) syncToPrivateRegistry(ctx context.Context, task entities.SyncTask, input SyncImagesInput) error {
	if input.TargetStrategy == nil {
		return fmt.Errorf("target strategy is required for registry sync")
	}

	targetRegistry := uc.getTargetRegistry(input)
	if targetRegistry == nil {
		return fmt.Errorf("target registry %s not found in configuration", input.TargetRegistry)
	}

	opts := output.SyncTargetOptions{
		SourceImageID:  task.Source,
		TargetImageTag: task.Source,

		SourceRegistryHost:     uc.cfg.Config.Registry.Host,
		SourceRegistryUsername: uc.cfg.Config.Registry.Username,
		SourceRegistryPassword: uc.cfg.Config.Registry.Password,
		SourceRegistryNS:       uc.cfg.Config.Registry.Namespace,

		TargetRegistryHost:     targetRegistry.Host,
		TargetRegistryUsername: targetRegistry.Username,
		TargetRegistryPassword: targetRegistry.Password,
	}

	exists, err := input.TargetStrategy.ExistsInTarget(ctx, opts)
	if err != nil {
		uc.logger.Warn("Failed to check if image exists in target registry: ", err)
	}

	if exists && !input.Force {
		uc.logger.Infof("Image %s already exists in target registry, skipping", task.Source)
		return nil
	}

	if input.DryRun {
		uc.logger.Infof("[dry-run] Would sync %s to private registry %s", task.Source, targetRegistry.Host)
		return nil
	}

	return input.TargetStrategy.SyncFromRegistry(ctx, opts)
}

func (uc *SyncImagesUseCaseImpl) getTargetRegistry(input SyncImagesInput) *config.PrivateRegistry {
	if input.TargetRegistry == "" {
		return nil
	}

	for _, reg := range input.PrivateRegistries {
		if reg.Name == input.TargetRegistry {
			return &config.PrivateRegistry{
				Name:     reg.Name,
				Host:     reg.Host,
				Username: reg.Username,
				Password: reg.Password,
			}
		}
	}

	return uc.cfg.Config.GetPrivateRegistryByName(input.TargetRegistry)
}
