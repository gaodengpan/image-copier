package cli

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/application/usecases"
	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// SyncTasksProcessor handles processing of sync tasks from YAML manifests
type SyncTasksProcessor struct {
	logger         *logrus.Logger
	force          bool
	dockerClient   output.DockerClient
	registryClient output.RegistryClient
	githubClient   output.GitHubClientWithRetry
	fileSystem     output.FileSystem
	httpClient     output.HTTPClient
	systemClient   output.SystemClient
	imageIDService *services.ImageIDService
}

func NewSyncTasksProcessor(
	logger *logrus.Logger,
	force bool,
	dockerClient output.DockerClient,
	registryClient output.RegistryClient,
	githubClient output.GitHubClientWithRetry,
	fileSystem output.FileSystem,
	httpClient output.HTTPClient,
	systemClient output.SystemClient,
	imageIDService *services.ImageIDService,
) *SyncTasksProcessor {
	return &SyncTasksProcessor{
		logger:         logger,
		force:          force,
		dockerClient:   dockerClient,
		registryClient: registryClient,
		githubClient:   githubClient,
		fileSystem:     fileSystem,
		httpClient:     httpClient,
		systemClient:   systemClient,
		imageIDService: imageIDService,
	}
}

// Process handles a single sync task
func (p *SyncTasksProcessor) Process(ctx context.Context, task WorkerPoolTask[entities.SyncTask], progressMgr *progress.Progress, workerIdx int) error {
	taskCfg := task.Config
	taskCfg.Registry.Arch = task.Item.Arch
	taskCfg.Registry.Os = task.Item.Os
	taskCfg.Force = p.force

	startTime := time.Now()
	stageCallback := func(stage use_cases.PullStage, polls int) {
		if progressMgr != nil {
			stageNames := [6]string{
				"checking local",
				"checking registry",
				"triggering workflow",
				"workflow running",
				"downloading",
				"loading",
			}
			stageIdx := int(stage)
			var pct float64

			if stage == use_cases.StageWaitWorkflow && polls > 0 {
				base := 20.0
				ceiling := 80.0
				pct = base + (ceiling-base)*(1-1/(1+0.05*float64(polls)))
			} else if stageIdx > 0 {
				stageWeights := [6]float64{5, 15, 20, 80, 95, 100}
				pct = stageWeights[stageIdx-1]
			}

			stageName := ""
			if stageIdx >= 0 && stageIdx < len(stageNames) {
				stageName = stageNames[stageIdx]
			}

			progressMgr.UpdateStage(workerIdx, progress.StageInfo{
				Label:     task.Item.DisplayName(),
				StageName: stageName,
				Percent:   pct,
				StartAt:   startTime,
			})
		}
	}

	useCase := use_cases.NewPullSingleImageUseCase(
		p.dockerClient,
		p.registryClient,
		p.githubClient,
		p.fileSystem,
		p.httpClient,
		p.logger,
		p.systemClient,
		p.imageIDService,
		taskCfg.Github.Owner,
		taskCfg.Github.Repo,
		taskCfg.Github.Token,
		taskCfg.Github.WorkflowID,
		stageCallback,
	)

	_, err := useCase.Execute(ctx, use_cases.PullSingleImageInput{
		ImageID:      task.Item.Source,
		RegistryHost: taskCfg.Registry.Host,
		RegistryUser: taskCfg.Registry.Username,
		RegistryPass: taskCfg.Registry.Password,
		RegistryNS:   taskCfg.Registry.Namespace,
		RegistryArch: taskCfg.Registry.Arch,
		RegistryOs:   taskCfg.Registry.Os,
		Force:        taskCfg.Force,
		DryRun:       taskCfg.DryRun,
	})

	return err
}
