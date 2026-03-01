package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	use_cases "github.com/gaodengpan/image-copier/internal/application/usecases"
	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
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
	targetStrategy output.SyncTargetStrategy
	targetType     output.SyncTargetType
	targetRegistry string
	privateRegs    []struct {
		Name     string
		Host     string
		Username string
		Password string
	}
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

func (p *SyncTasksProcessor) WithTargetStrategy(strategy output.SyncTargetStrategy, targetType output.SyncTargetType, targetRegistry string, privateRegs []struct {
	Name     string
	Host     string
	Username string
	Password string
}) *SyncTasksProcessor {
	p.targetStrategy = strategy
	p.targetType = targetType
	p.targetRegistry = targetRegistry
	p.privateRegs = privateRegs
	return p
}

// Process handles a single sync task
func (p *SyncTasksProcessor) Process(ctx context.Context, task WorkerPoolTask[entities.SyncTask], progressMgr *progress.Progress, workerIdx int) error {
	taskCfg := task.Config
	taskCfg.Registry.Arch = task.Item.Arch
	taskCfg.Registry.Os = task.Item.Os
	taskCfg.Force = p.force

	startTime := time.Now()

	if p.targetStrategy != nil && p.targetType == output.SyncTargetRegistry {
		return p.processWithTargetStrategy(ctx, task, taskCfg, startTime, progressMgr, workerIdx)
	}

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

func (p *SyncTasksProcessor) processWithTargetStrategy(ctx context.Context, task WorkerPoolTask[entities.SyncTask], taskCfg *config.Config, startTime time.Time, progressMgr *progress.Progress, workerIdx int) error {
	if progressMgr != nil {
		progressMgr.UpdateStage(workerIdx, progress.StageInfo{
			Label:     task.Item.DisplayName(),
			StageName: "ensuring image in domestic registry",
			Percent:   10,
			StartAt:   startTime,
		})
	}

	stageCallback := func(stage use_cases.PullStage, polls int) {}

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

	if err != nil {
		return fmt.Errorf("failed to ensure image in domestic registry: %w", err)
	}

	if progressMgr != nil {
		progressMgr.UpdateStage(workerIdx, progress.StageInfo{
			Label:     task.Item.DisplayName(),
			StageName: "syncing to private registry",
			Percent:   80,
			StartAt:   startTime,
		})
	}

	targetReg := p.getTargetRegistry()
	if targetReg == nil {
		return fmt.Errorf("target registry %s not found", p.targetRegistry)
	}

	opts := output.SyncTargetOptions{
		SourceImageID:  task.Item.Source,
		TargetImageTag: task.Item.Source,

		SourceRegistryHost:     taskCfg.Registry.Host,
		SourceRegistryUsername: taskCfg.Registry.Username,
		SourceRegistryPassword: taskCfg.Registry.Password,
		SourceRegistryNS:       taskCfg.Registry.Namespace,

		TargetRegistryHost:     targetReg.Host,
		TargetRegistryUsername: targetReg.Username,
		TargetRegistryPassword: targetReg.Password,
	}

	exists, err := p.targetStrategy.ExistsInTarget(ctx, opts)
	if err != nil {
		p.logger.Warn("Failed to check if image exists in target registry: ", err)
	}

	if exists && !p.force {
		p.logger.Infof("Image %s already exists in target registry, skipping", task.Item.Source)
		return nil
	}

	if taskCfg.DryRun {
		p.logger.Infof("[dry-run] Would sync %s to private registry %s", task.Item.Source, targetReg.Host)
		return nil
	}

	return p.targetStrategy.SyncFromRegistry(ctx, opts)
}

func (p *SyncTasksProcessor) getTargetRegistry() *config.PrivateRegistry {
	for _, reg := range p.privateRegs {
		if reg.Name == p.targetRegistry {
			return &config.PrivateRegistry{
				Name:     reg.Name,
				Host:     reg.Host,
				Username: reg.Username,
				Password: reg.Password,
			}
		}
	}
	return nil
}
