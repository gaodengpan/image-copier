package use_cases

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/gaodengpan/image-copier/internal/adapters/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/filesystem"
	"github.com/gaodengpan/image-copier/internal/adapters/github"
	"github.com/gaodengpan/image-copier/internal/adapters/registry"
)

type SyncImagesUseCaseImpl struct {
	registryHost     string
	registryUser     string
	registryPass     string
	registryNS       string
	force            bool
	dryRun           bool
	githubToken      string
	githubOwner      string
	githubRepo       string
	githubWorkflowID string
}

func NewSyncImagesUseCase(
	registryHost, registryUser, registryPass, registryNS string,
	force, dryRun bool,
	githubToken, githubOwner, githubRepo, githubWorkflowID string,
) *SyncImagesUseCaseImpl {
	return &SyncImagesUseCaseImpl{
		registryHost:     registryHost,
		registryUser:     registryUser,
		registryPass:     registryPass,
		registryNS:       registryNS,
		force:            force,
		dryRun:           dryRun,
		githubToken:      githubToken,
		githubOwner:      githubOwner,
		githubRepo:       githubRepo,
		githubWorkflowID: githubWorkflowID,
	}
}

func (uc *SyncImagesUseCaseImpl) Execute(ctx context.Context, input SyncImagesInput) (synced []SyncTask, needsSync []SyncTask, err error) {
	tasks := input.Tasks
	logger := input.Logger
	workerCount := input.WorkerCount
	stageCallback := input.StageCallback

	registryClient := registry.NewSkopeoAdapter()
	dockerClient := docker.NewExecDockerAdapter()

	type diffResult struct {
		task         SyncTask
		remoteExists bool
		localExists  bool
	}
	results := make([]diffResult, len(tasks))

	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	maxWorkersByCPU := runtime.NumCPU()
	if workerCount > maxWorkersByCPU {
		workerCount = maxWorkersByCPU
	}

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, task SyncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sourceID := normalizeSourceID(task.Source)
			destID := registryClient.BuildDestImageID(sourceID, uc.registryHost, uc.registryNS)

			remoteExists, _ := registryClient.CheckImageExists(ctx, destID, uc.registryUser, uc.registryPass)
			localExists, _ := dockerClient.ImageExists(ctx, task.Source)

			results[idx] = diffResult{task: task, remoteExists: remoteExists, localExists: localExists}
		}(i, t)
	}
	wg.Wait()

	for _, r := range results {
		if r.localExists && !uc.force {
			synced = append(synced, r.task)
		} else {
			needsSync = append(needsSync, r.task)
		}
	}

	if uc.dryRun {
		return synced, needsSync, nil
	}

	if len(needsSync) == 0 {
		return synced, needsSync, nil
	}

	syncWorkerCount := workerCount
	if syncWorkerCount > len(needsSync) {
		syncWorkerCount = len(needsSync)
	}

	githubClient := github.NewAPIAdapter(nil, uc.githubToken, uc.githubOwner, uc.githubRepo)
	fs := filesystem.NewOSAdapter()
	httpClient := &http.Client{}

	syncSem := make(chan struct{}, syncWorkerCount)
	var syncWg sync.WaitGroup
	failedCount := 0
	var mu sync.Mutex

	for i, t := range needsSync {
		syncWg.Add(1)
		go func(idx int, task SyncTask) {
			defer syncWg.Done()
			syncSem <- struct{}{}
			defer func() { <-syncSem }()

			useCase := NewPullSingleImageUseCase(
				dockerClient,
				registryClient,
				githubClient,
				fs,
				httpClient,
				logger,
				uc.githubOwner,
				uc.githubRepo,
				uc.githubToken,
				uc.githubWorkflowID,
				func(stage PullStage, polls int) {
					if stageCallback != nil {
						stageCallback(stage, polls)
					}
				},
			)

			_, syncErr := useCase.Execute(ctx, PullSingleImageInput{
				ImageID:      task.Source,
				RegistryHost: uc.registryHost,
				RegistryUser: uc.registryUser,
				RegistryPass: uc.registryPass,
				RegistryNS:   uc.registryNS,
				RegistryArch: task.Arch,
				RegistryOs:   task.Os,
				Force:        uc.force,
				DryRun:       uc.dryRun,
			})

			if syncErr != nil {
				mu.Lock()
				failedCount++
				mu.Unlock()
			}

			_ = idx
		}(i, t)
	}
	syncWg.Wait()

	if failedCount > 0 {
		return synced, needsSync, fmt.Errorf("%d image(s) failed to sync", failedCount)
	}

	return synced, needsSync, nil
}
