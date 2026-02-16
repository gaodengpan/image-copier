package cli

import (
	"context"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/adapters/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/filesystem"
	"github.com/gaodengpan/image-copier/internal/adapters/github"
	"github.com/gaodengpan/image-copier/internal/adapters/registry"
	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/internal/ports"
	"github.com/gaodengpan/image-copier/internal/use_cases"
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
	logger         *logrus.Logger
	dockerClient   ports.DockerClient
	registryClient ports.RegistryClient
	githubClient   ports.GitHubClient
	fileSystem     ports.FileSystem
	httpClient     *http.Client
}

func NewCLIImagesProcessor(logger *logrus.Logger) *CLIImagesProcessor {
	return &CLIImagesProcessor{
		logger:         logger,
		dockerClient:   docker.NewExecDockerAdapter(),
		registryClient: registry.NewSkopeoAdapter(),
		githubClient:   github.NewAPIAdapter(nil, "", "", ""),
		fileSystem:     filesystem.NewOSAdapter(),
		httpClient:     &http.Client{},
	}
}

// Process handles a single CLI image task
func (p *CLIImagesProcessor) Process(ctx context.Context, task WorkerPoolTask[CLIImage], progressMgr *progress.Progress, workerIdx int) error {
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
		task.Config.GithubOwner,
		task.Config.GithubRepo,
		task.Config.GithubToken,
		task.Config.GithubWorkflowID,
		stageCallback,
	)

	_, err := useCase.Execute(ctx, use_cases.PullSingleImageInput{
		ImageID:      task.Item.ImageID,
		RegistryHost: task.Config.RegistryHost,
		RegistryUser: task.Config.RegistryUsername,
		RegistryPass: task.Config.RegistryPassword,
		RegistryNS:   task.Config.RegistryNamespace,
		RegistryArch: task.Config.RegistryArch,
		RegistryOs:   task.Config.RegistryOs,
		Force:        task.Config.Force,
		DryRun:       task.Config.DryRun,
	})

	return err
}

// GetStageCallback returns a stage callback function
func (p *CLIImagesProcessor) GetStageCallback(workerIdx int, label string, startTime time.Time) func(core.PullStage, int) {
	return nil
}
