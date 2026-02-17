package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gaodengpan/image-copier/internal/adapters/secondary/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/filesystem"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/github"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/registry"
	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/application/usecases"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// SyncTask implements the WorkItem interface for image pull task processing
type SyncTask struct {
	Source string // 镜像源地址 (如 redis:latest, ghcr.io/tektoncd/pipeline/controller:v1.1.0)
	Arch   string // 架构 (如 amd64, arm64)
	Os     string // 操作系统 (如 linux)
}

func (t SyncTask) DisplayName() string {
	if t.Arch == "" && t.Os == "" {
		return t.Source
	}
	return fmt.Sprintf("%s (%s/%s)", t.Source, t.Os, t.Arch)
}

// SyncTasksProcessor handles processing of sync tasks from YAML manifests
type SyncTasksProcessor struct {
	logger         *logrus.Logger
	force          bool
	dockerClient   ports.DockerClient
	registryClient ports.RegistryClient
	githubClient   ports.GitHubClient
	fileSystem     ports.FileSystem
	httpClient     *http.Client
}

func NewSyncTasksProcessor(logger *logrus.Logger, force bool) *SyncTasksProcessor {
	return &SyncTasksProcessor{
		logger:         logger,
		force:          force,
		dockerClient:   docker.NewExecDockerAdapter(),
		registryClient: registry.NewSkopeoAdapter(),
		githubClient:   github.NewAPIAdapter(nil, "", "", ""),
		fileSystem:     filesystem.NewOSAdapter(),
		httpClient:     &http.Client{},
	}
}

// Process handles a single sync task
func (p *SyncTasksProcessor) Process(ctx context.Context, task WorkerPoolTask[SyncTask], progressMgr *progress.Progress, workerIdx int) error {
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

// GetStageCallback returns a stage callback function
func (p *SyncTasksProcessor) GetStageCallback(workerIdx int, label string, startTime time.Time) func(use_cases.PullStage, int) {
	return nil
}
