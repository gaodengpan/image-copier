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

type PullImagesUseCaseImpl struct {
	registryHost     string
	registryUser     string
	registryPass     string
	registryNS       string
	registryArch     string
	registryOs       string
	force            bool
	dryRun           bool
	githubToken      string
	githubOwner      string
	githubRepo       string
	githubWorkflowID string
}

func NewPullImagesUseCase(
	registryHost, registryUser, registryPass, registryNS, registryArch, registryOs string,
	force, dryRun bool,
	githubToken, githubOwner, githubRepo, githubWorkflowID string,
) *PullImagesUseCaseImpl {
	return &PullImagesUseCaseImpl{
		registryHost:     registryHost,
		registryUser:     registryUser,
		registryPass:     registryPass,
		registryNS:       registryNS,
		registryArch:     registryArch,
		registryOs:       registryOs,
		force:            force,
		dryRun:           dryRun,
		githubToken:      githubToken,
		githubOwner:      githubOwner,
		githubRepo:       githubRepo,
		githubWorkflowID: githubWorkflowID,
	}
}

func (uc *PullImagesUseCaseImpl) Execute(ctx context.Context, input PullImagesInput) (failedCount int, err error) {
	images := input.Images
	logger := input.Logger
	workerCount := input.WorkerCount
	stageCallback := input.StageCallback

	if workerCount < 1 {
		workerCount = 1
	}

	if workerCount > len(images) {
		workerCount = len(images)
	}

	maxWorkersByCPU := runtime.NumCPU()
	if workerCount > maxWorkersByCPU {
		workerCount = maxWorkersByCPU
	}

	dockerClient := docker.NewExecDockerAdapter()
	registryClient := registry.NewSkopeoAdapter()
	githubClient := github.NewAPIAdapter(nil, uc.githubToken, uc.githubOwner, uc.githubRepo)
	fs := filesystem.NewOSAdapter()
	httpClient := &http.Client{}

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	failedCount = 0
	var mu sync.Mutex

	for i, img := range images {
		wg.Add(1)
		go func(idx int, imageID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

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

			_, err := useCase.Execute(ctx, PullSingleImageInput{
				ImageID:      imageID,
				RegistryHost: uc.registryHost,
				RegistryUser: uc.registryUser,
				RegistryPass: uc.registryPass,
				RegistryNS:   uc.registryNS,
				RegistryArch: uc.registryArch,
				RegistryOs:   uc.registryOs,
				Force:        uc.force,
				DryRun:       uc.dryRun,
			})

			if err != nil {
				mu.Lock()
				failedCount++
				mu.Unlock()
			}
		}(i, img)
	}

	wg.Wait()

	if failedCount > 0 {
		return failedCount, fmt.Errorf("%d image(s) failed", failedCount)
	}
	return 0, nil
}
