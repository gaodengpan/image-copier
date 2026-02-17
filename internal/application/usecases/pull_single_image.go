package use_cases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/pkg/retry"
)

var (
	ErrSkipped = fmt.Errorf("image already exists locally")
	ErrDryRun  = fmt.Errorf("dry-run: no changes made")
)

type PullSingleImageUseCaseImpl struct {
	dockerClient     ports.DockerClient
	registryClient   ports.RegistryClient
	githubClient     ports.GitHubClient
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
	githubClient ports.GitHubClient,
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
		registryClient:   registryClient,
		githubClient:     githubClient,
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
		return PullSingleImageOutput{}, fmt.Errorf("invalid image name: %s", sanitizeForLog(input.ImageID))
	}

	uc.logger.Infof("Processing image: %s", sanitizeForLog(input.ImageID))

	sourceID := normalizeSourceID(input.ImageID)
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

	normalizedID := normalizeSourceID(imageID)

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
	suffix := fmt.Sprintf("--%d", time.Now().Unix())

	data := map[string]interface{}{
		"ref": "master",
		"inputs": map[string]string{
			"imageId":     sourceID,
			"destImageId": destImageID,
			"suffix":      suffix,
			"arch":        arch,
			"os":          osType,
		},
	}

	err := retry.Retry(ctx, retry.DefaultConfig(), func() error {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}

		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches",
			uc.githubOwner, uc.githubRepo, uc.githubWorkflowID)

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		const (
			githubMediaType  = "application/vnd.github+json"
			githubAPIVersion = "2022-11-28"
		)

		req.Header.Set("Accept", githubMediaType)
		req.Header.Set("Authorization", "Bearer "+uc.githubToken)
		req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
		req.Header.Set("Content-Type", "application/json")

		resp, err := uc.httpClient.Do(req)
		if err != nil {
			return retry.NewRetryableError(fmt.Errorf("failed to send request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return nil
		}

		uc.logger.Debugf("GitHub API response status: %d", resp.StatusCode)

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retry.NewRetryableError(fmt.Errorf("unexpected status code: %d", resp.StatusCode))
		}

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	})
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}

	runID, err := uc.findWorkflowRunID(ctx, sourceID, destImageID, suffix)
	if err != nil {
		return "", fmt.Errorf("failed to find workflow run ID: %w", err)
	}

	uc.logger.Infof("Triggered workflow run ID: %s", sanitizeForLog(runID))
	return runID, nil
}

func (uc *PullSingleImageUseCaseImpl) findWorkflowRunID(ctx context.Context, sourceID, destImageID, suffix string) (string, error) {
	expectedName := fmt.Sprintf("copy %s to %s%s", sourceID, destImageID, suffix)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs",
		uc.githubOwner, uc.githubRepo, uc.githubWorkflowID)

	const (
		githubMediaType  = "application/vnd.github+json"
		githubAPIVersion = "2022-11-28"
		maxRetries       = 60
		pollInterval     = 2 * time.Second
	)

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("Accept", githubMediaType)
		req.Header.Set("Authorization", "Bearer "+uc.githubToken)
		req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

		resp, err := uc.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var result struct {
			WorkflowRuns []struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"workflow_runs"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		for _, run := range result.WorkflowRuns {
			if run.Name == expectedName {
				return fmt.Sprintf("%d", run.ID), nil
			}
		}

		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "", fmt.Errorf("workflow run not found after %d attempts", maxRetries)
}

func (uc *PullSingleImageUseCaseImpl) waitForWorkflow(ctx context.Context, runID string) error {
	const (
		githubMediaType  = "application/vnd.github+json"
		githubAPIVersion = "2022-11-28"
		maxRetries       = 300
		pollInterval     = 2 * time.Second
	)

	for i := 0; i < maxRetries; i++ {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s",
			uc.githubOwner, uc.githubRepo, runID)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("Accept", githubMediaType)
		req.Header.Set("Authorization", "Bearer "+uc.githubToken)
		req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

		resp, err := uc.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var result struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		if result.Status == "completed" {
			if result.Conclusion == "success" {
				return nil
			}
			return fmt.Errorf("workflow failed with conclusion: %s", result.Conclusion)
		}

		uc.notifyStage(StageWaitWorkflow, i)

		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("workflow timed out after %d attempts", maxRetries)
}

func (uc *PullSingleImageUseCaseImpl) downloadAndLoadImage(ctx context.Context, registryImageID, userImageTag, username, password string) error {
	if !uc.imageValidator.IsValidImageName(registryImageID) {
		return fmt.Errorf("invalid image name: %s", sanitizeForLog(registryImageID))
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
		return fmt.Errorf("pre-pull validation failed: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

func sanitizeForLog(input string) string {
	return services.SanitizeForLog(input)
}

func normalizeSourceID(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		normalized = fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		normalized = normalizeImageSegment(segs[0]) + "/" + segs[1]
	default:
		normalized = imageID
	}

	lastSlash := strings.LastIndex(normalized, "/")
	tail := normalized
	if lastSlash >= 0 {
		tail = normalized[lastSlash+1:]
	}
	if !hasTagOrDigest(tail) {
		normalized += ":latest"
	}

	return normalized
}

func normalizeImageSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		return "docker.io/" + segment
	}
	return segment
}

func hasTagOrDigest(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.Split(s, "/")
	tailSegment := parts[len(parts)-1]
	if strings.Contains(tailSegment, "@") {
		return true
	}
	colonParts := strings.Split(tailSegment, ":")
	if len(colonParts) > 2 || len(colonParts) == 2 {
		return true
	}
	return false
}
