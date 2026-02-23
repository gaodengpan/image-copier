package use_cases

import (
	"context"
	"fmt"
	"strings"

	domainerrors "github.com/gaodengpan/image-copier/internal/domain/errors"
	"github.com/gaodengpan/image-copier/internal/domain/ports"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
)

var (
	ErrSkipped = fmt.Errorf("image already exists locally")
	ErrDryRun  = fmt.Errorf("dry-run: no changes made")
)

type PullSingleImageUseCaseImpl struct {
	dockerClient     ports.DockerClient
	registryClient   ports.RegistryClient
	githubClient     ports.GitHubClientWithRetry
	fileSystem       ports.FileSystem
	httpClient       ports.HTTPClient
	logger           ports.Logger
	systemClient     ports.SystemClient
	imageIDService   *services.ImageIDService
	githubOwner      string
	githubRepo       string
	githubToken      string
	githubWorkflowID string
	imageValidator   *validators.ImageValidator
	stageCallback    func(stage PullStage, polls int)
}

func NewPullSingleImageUseCase(
	dockerClient ports.DockerClient,
	registryClient ports.RegistryClient,
	githubClient ports.GitHubClientWithRetry,
	fileSystem ports.FileSystem,
	httpClient ports.HTTPClient,
	logger ports.Logger,
	systemClient ports.SystemClient,
	imageIDService *services.ImageIDService,
	githubOwner, githubRepo, githubToken, githubWorkflowID string,
	stageCallback func(stage PullStage, polls int),
) *PullSingleImageUseCaseImpl {
	return &PullSingleImageUseCaseImpl{
		dockerClient:     dockerClient,
		githubClient:     githubClient,
		registryClient:   registryClient,
		fileSystem:       fileSystem,
		httpClient:       httpClient,
		logger:           logger,
		systemClient:     systemClient,
		imageIDService:   imageIDService,
		githubOwner:      githubOwner,
		githubRepo:       githubRepo,
		githubToken:      githubToken,
		githubWorkflowID: githubWorkflowID,
		imageValidator:   validators.NewImageValidator(),
		stageCallback:    stageCallback,
	}
}

func (uc *PullSingleImageUseCaseImpl) Execute(ctx context.Context, input PullSingleImageInput) (PullSingleImageOutput, error) {
	if err := uc.prePullValidate(ctx); err != nil {
		return PullSingleImageOutput{}, err
	}

	if !uc.imageValidator.IsValidImageName(input.ImageID) {
		return PullSingleImageOutput{}, domainerrors.NewImageError(
			domainerrors.ErrInvalidImageName,
			input.ImageID,
			"invalid image name format",
		)
	}

	uc.logger.Infof("Processing image: %s", sanitizeForLog(input.ImageID))

	sourceID := uc.imageIDService.NormalizeSourceID(input.ImageID)
	destImageID := uc.registryClient.BuildDestImageID(sourceID, input.RegistryHost, input.RegistryNS)

	uc.notifyStage(StageCheckLocal, 0)
	if !input.Force {
		localExists, err := uc.checkLocalImageExists(ctx, input.ImageID)
		if err != nil {
			uc.logger.Errorf("Error checking local image: %v", err)
			return PullSingleImageOutput{}, fmt.Errorf("failed to check local image: %w", err)
		} else if localExists {
			uc.logger.Infof("Image %s already exists locally, skipping (use --force to override)", sanitizeForLog(sourceID))
			return PullSingleImageOutput{Skipped: true}, nil
		}
	}

	if ctx.Err() != nil {
		return PullSingleImageOutput{}, ctx.Err()
	}

	uc.notifyStage(StageCheckRegistry, 0)
	exists, err := uc.registryClient.ImageExists(ctx, destImageID, input.RegistryUser, input.RegistryPass)
	if err != nil {
		return PullSingleImageOutput{}, fmt.Errorf("failed to check if image exists: %w", err)
	}

	if ctx.Err() != nil {
		return PullSingleImageOutput{}, ctx.Err()
	}

	if input.DryRun {
		if exists {
			uc.logger.Infof("[dry-run] %s → %s: image exists in registry, would copy and load", sanitizeForLog(sourceID), sanitizeForLog(destImageID))
		} else {
			uc.logger.Infof("[dry-run] %s → %s: would trigger workflow, then copy and load", sanitizeForLog(sourceID), sanitizeForLog(destImageID))
		}
		return PullSingleImageOutput{DryRun: true}, nil
	}

	if !exists {
		uc.logger.Info("Image not found in destination registry, triggering GitHub workflow")

		uc.notifyStage(StageTriggerWorkflow, 0)
		runID, err := uc.triggerWorkflow(ctx, sourceID, destImageID, input.RegistryArch, input.RegistryOs)
		if err != nil {
			return PullSingleImageOutput{}, fmt.Errorf("failed to trigger workflow: %w", err)
		}

		if ctx.Err() != nil {
			return PullSingleImageOutput{}, ctx.Err()
		}

		uc.notifyStage(StageWaitWorkflow, 0)
		if err := uc.waitForWorkflow(ctx, runID); err != nil {
			return PullSingleImageOutput{}, fmt.Errorf("workflow failed: %w", err)
		}
	} else {
		uc.logger.Info("Image already exists in destination registry")
	}

	if ctx.Err() != nil {
		return PullSingleImageOutput{}, ctx.Err()
	}

	uc.notifyStage(StageDownloadImage, 0)
	if err := uc.downloadAndLoadImage(ctx, destImageID, input.ImageID, input.RegistryUser, input.RegistryPass); err != nil {
		return PullSingleImageOutput{}, fmt.Errorf("failed to copy and import image: %w", err)
	}

	uc.logger.Infof("Successfully processed image: %s", sanitizeForLog(input.ImageID))
	return PullSingleImageOutput{}, nil
}

func (uc *PullSingleImageUseCaseImpl) notifyStage(stage PullStage, polls int) {
	if uc.stageCallback != nil {
		uc.stageCallback(stage, polls)
	}
}

func (uc *PullSingleImageUseCaseImpl) checkLocalImageExists(ctx context.Context, imageID string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	normalizedID := uc.imageIDService.NormalizeSourceID(imageID)

	exists, err := uc.dockerClient.ImageExists(ctx, imageID)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	if normalizedID != imageID {
		exists, err := uc.dockerClient.ImageExists(ctx, normalizedID)
		if err != nil {
			return false, err
		}
		return exists, nil
	}

	return false, nil
}

func (uc *PullSingleImageUseCaseImpl) triggerWorkflow(ctx context.Context, sourceID, destImageID, arch, osType string) (string, error) {
	runID, err := uc.githubClient.TriggerWorkflowWithRetry(ctx, sourceID, destImageID, arch, osType)
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}

	uc.logger.Infof("Triggered workflow run ID: %s", sanitizeForLog(runID))
	return runID, nil
}

func (uc *PullSingleImageUseCaseImpl) findWorkflowRunID(ctx context.Context, sourceID, destImageID, suffix string) (string, error) {
	return uc.githubClient.FindWorkflowRunID(ctx, uc.githubOwner, uc.githubRepo, uc.githubWorkflowID, sourceID, destImageID, suffix)
}

func (uc *PullSingleImageUseCaseImpl) waitForWorkflow(ctx context.Context, runID string) error {
	return uc.githubClient.WaitForWorkflowSimple(ctx, runID)
}

func (uc *PullSingleImageUseCaseImpl) downloadAndLoadImage(ctx context.Context, registryImageID, userImageTag, username, password string) error {
	if !uc.imageValidator.IsValidImageName(registryImageID) {
		return domainerrors.NewImageError(
			domainerrors.ErrInvalidImageName,
			registryImageID,
			"invalid image name format",
		)
	}

	tmpPath, err := uc.fileSystem.CreateTempFile("image-copier-*.tar")
	if err != nil {
		return err
	}

	cleanup := func() {
		if err := uc.fileSystem.RemoveFile(tmpPath); err != nil {
			uc.logger.Debugf("Failed to remove temp file %s: %v", tmpPath, err)
		}
	}
	defer cleanup()

	if err := uc.registryClient.SaveImageToFile(ctx, registryImageID, userImageTag, tmpPath, username, password); err != nil {
		return err
	}

	uc.notifyStage(StageLoadImage, 0)

	if err := uc.dockerClient.LoadImage(ctx, tmpPath); err != nil {
		return err
	}

	return nil
}

func (uc *PullSingleImageUseCaseImpl) prePullValidate(ctx context.Context) error {
	var validationErrors []string

	uc.logger.Debugf("Performing pre-pull validation...")

	skopeoExists, err := uc.systemClient.CommandExists(ctx, "skopeo")
	if err != nil {
		uc.logger.Errorf("Error checking skopeo command: %v", err)
		return fmt.Errorf("error checking skopeo command: %w", err)
	}
	if !skopeoExists {
		validationErrors = append(validationErrors, "skopeo command not found in PATH")
	}

	dockerExists, err := uc.systemClient.CommandExists(ctx, "docker")
	if err != nil {
		uc.logger.Errorf("Error checking docker command: %v", err)
		return fmt.Errorf("error checking docker command: %w", err)
	}
	if !dockerExists {
		validationErrors = append(validationErrors, "docker command not found in PATH")
	}

	if skopeoExists && dockerExists {
		_, err := uc.systemClient.DockerRunning(ctx)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Docker service is not running: %v", err))
		}
	}

	if len(validationErrors) > 0 {
		return domainerrors.NewImageError(
			domainerrors.ErrValidationFailed,
			"",
			"pre-pull validation failed: "+strings.Join(validationErrors, "; "),
		)
	}

	return nil
}

var sanitizeForLog = services.SanitizeForLog
