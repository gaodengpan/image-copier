package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

// PullStage represents a stage in the image pull pipeline.
type PullStage int

const (
	StageCheckLocal    PullStage = iota // 检查本地镜像
	StageCheckRegistry                  // 检查远端仓库
	StageTriggerWorkflow                // 触发 GitHub Workflow
	StageWaitWorkflow                   // 等待 Workflow 完成
	StageCopyImage                      // skopeo copy (下载镜像)
	StageLoadImage                      // docker load (导入本地)
)

// StageCallback is called when PullSingle transitions between stages.
// polls is only meaningful for StageWaitWorkflow — it carries the current poll count.
type StageCallback func(stage PullStage, polls int)

// Puller handles the image pulling process
type Puller struct {
	Config        *Config
	RetryConfig   *retry.Config
	Logger        *logrus.Logger
	StageCallback StageCallback
}

// Config holds the configuration needed for Puller
type Config struct {
	GithubOwner    string
	GithubRepo     string
	GithubToken    string
	GithubWorkflowID string
	RegistryHost   string
	RegistryUsername string
	RegistryPassword string
	RegistryNamespace string
	RegistryArch   string
	RegistryOs     string
	Force          bool
}

// ErrSkipped indicates an image was skipped because it already exists locally.
var ErrSkipped = fmt.Errorf("image already exists locally")

// CheckLocalImageExists checks whether the given image is available in the local Docker daemon.
func (p *Puller) CheckLocalImageExists(imageID string) (bool, error) {
	cmd := exec.Command("docker", "image", "inspect", imageID)
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err != nil {
		return false, nil
	}
	return true, nil
}

// NewPuller creates a new Puller instance
func NewPuller(config *Config, logger *logrus.Logger) *Puller {
	return &Puller{
		Config:      config,
		RetryConfig: retry.DefaultConfig(),
		Logger:      logger,
	}
}

func (p *Puller) notifyStage(stage PullStage, polls int) {
	if p.StageCallback != nil {
		p.StageCallback(stage, polls)
	}
}

// PullSingle pulls a single image through GitHub Actions
func (p *Puller) PullSingle(ctx context.Context, imageID string) error {
	p.Logger.Infof("Processing image: %s", imageID)

	sourceID := p.normalizeSourceID(imageID)
	destImageID := p.buildDestImageID(sourceID)

	// Check if image already exists in the local Docker daemon
	p.notifyStage(StageCheckLocal, 0)
	if !p.Config.Force {
		localExists, err := p.CheckLocalImageExists(sourceID)
		if err != nil {
			p.Logger.Warnf("Failed to check local image, continuing: %v", err)
		} else if localExists {
			p.Logger.Infof("Image %s already exists locally, skipping (use --force to override)", sourceID)
			return ErrSkipped
		}
	}

	// Check if image already exists
	p.notifyStage(StageCheckRegistry, 0)
	exists, err := p.checkImageExists(destImageID)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	if !exists {
		p.Logger.Info("Image not found in destination registry, triggering GitHub workflow")

		// Trigger GitHub workflow
		p.notifyStage(StageTriggerWorkflow, 0)
		runID, err := p.triggerWorkflow(ctx, sourceID, destImageID)
		if err != nil {
			return fmt.Errorf("failed to trigger workflow: %w", err)
		}

		// Wait for workflow completion
		if err := p.waitForWorkflow(ctx, runID); err != nil {
			return fmt.Errorf("workflow failed: %w", err)
		}
	} else {
		p.Logger.Info("Image already exists in destination registry")
	}

	// Copy and import image
	p.notifyStage(StageCopyImage, 0)
	if err := p.copyAndImportImage(destImageID, sourceID); err != nil {
		return fmt.Errorf("failed to copy and import image: %w", err)
	}

	p.Logger.Infof("Successfully processed image: %s", imageID)
	return nil
}

func (p *Puller) normalizeSourceID(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		// No registry specified, assume docker.io/library
		normalized = fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		// Check if first segment looks like a domain
		if !strings.Contains(segs[0], ".") && !strings.Contains(segs[0], ":") {
			// Not a domain, prepend docker.io
			normalized = fmt.Sprintf("docker.io/%s", imageID)
		} else {
			normalized = imageID
		}
	default:
		// Already fully qualified
		normalized = imageID
	}

	// docker-daemon: transport requires a tag or digest; append :latest if missing.
	lastSlash := strings.LastIndex(normalized, "/")
	tail := normalized
	if lastSlash >= 0 {
		tail = normalized[lastSlash+1:]
	}
	if !strings.Contains(tail, ":") && !strings.Contains(tail, "@") {
		normalized += ":latest"
	}

	return normalized
}

func (p *Puller) buildDestImageID(sourceID string) string {
	if p.Config.RegistryNamespace == "" {
		return fmt.Sprintf("%s/%s", p.Config.RegistryHost, sourceID)
	}
	
	// Replace slashes with underscores and limit length
	normalized := strings.ReplaceAll(sourceID, "/", "_")
	if len(normalized) > 40 {
		normalized = normalized[:40]
	}
	
	return fmt.Sprintf("%s/%s/%s", p.Config.RegistryHost, p.Config.RegistryNamespace, normalized)
}

func (p *Puller) checkImageExists(destImageID string) (bool, error) {
	creds := fmt.Sprintf("%s:%s", p.Config.RegistryUsername, p.Config.RegistryPassword)
	cmd := exec.Command("skopeo", "inspect", "--creds="+creds, "docker://"+destImageID)
	
	_, err := cmd.Output()
	if err != nil {
		// If command fails, assume image doesn't exist
		return false, nil
	}
	
	return true, nil
}

func (p *Puller) triggerWorkflow(ctx context.Context, sourceID, destImageID string) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())

	data := map[string]interface{}{
		"ref": "master",
		"inputs": map[string]string{
			"imageId":      sourceID,
			"destImageId":  destImageID,
			"suffix":       suffix,
			"arch":         p.Config.RegistryArch,
			"os":           p.Config.RegistryOs,
		},
	}

	var err error
	err = retry.Retry(ctx, p.RetryConfig, func() error {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}

		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches",
			p.Config.GithubOwner, p.Config.GithubRepo, p.Config.GithubWorkflowID)

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return retry.NewRetryableError(fmt.Errorf("failed to send request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return nil
		}

		// Retry on certain HTTP errors
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return retry.NewRetryableError(fmt.Errorf("unexpected status code: %d", resp.StatusCode))
		}

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	})
	if err != nil {
		return "", fmt.Errorf("failed to trigger workflow: %w", err)
	}

	// Find the workflow run ID
	runID, err := p.findWorkflowRunID(ctx, sourceID, destImageID, suffix)
	if err != nil {
		return "", fmt.Errorf("failed to find workflow run ID: %w", err)
	}

	p.Logger.Infof("Triggered workflow run ID: %s", runID)
	return runID, nil
}

func (p *Puller) findWorkflowRunID(ctx context.Context, sourceID, destImageID, suffix string) (string, error) {
	expectedName := fmt.Sprintf("copy %s to %s%s", sourceID, destImageID, suffix)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs",
		p.Config.GithubOwner, p.Config.GithubRepo, p.Config.GithubWorkflowID)

	// Poll for up to 30 seconds
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTimer(0)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("workflow run not found after 30 seconds")
		case <-ticker.C:
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				// Network error, retry
				p.Logger.Debugf("Network error, retrying: %v", err)
				ticker.Reset(time.Second)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var result struct {
					WorkflowRuns []struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					} `json:"workflow_runs"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					p.Logger.Debugf("Failed to decode response, retrying: %v", err)
					ticker.Reset(time.Second)
					continue
				}

				for _, run := range result.WorkflowRuns {
					if run.Name == expectedName {
						return fmt.Sprintf("%d", run.ID), nil
					}
				}
			} else if resp.StatusCode >= 500 {
				// Server error, retry
				p.Logger.Debugf("Server error %d, retrying", resp.StatusCode)
				ticker.Reset(time.Second)
				continue
			} else {
				return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}

			// Not found yet, wait and retry
			ticker.Reset(time.Second)
		}
	}
}

func (p *Puller) waitForWorkflow(ctx context.Context, runID string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s",
		p.Config.GithubOwner, p.Config.GithubRepo, runID)

	link := fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s",
		p.Config.GithubOwner, p.Config.GithubRepo, runID)

	p.Logger.Infof("Workflow run link: %s", link)

	pollCount := 0
	for {
		pollCount++
		p.notifyStage(StageWaitWorkflow, pollCount)

		var run struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}

		err := retry.Retry(ctx, p.RetryConfig, func() error {
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return retry.NewRetryableError(fmt.Errorf("failed to send request: %w", err))
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// Retry on certain HTTP errors
				if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
					return retry.NewRetryableError(fmt.Errorf("unexpected status code: %d", resp.StatusCode))
				}
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}

			if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to check workflow status: %w", err)
		}

		switch run.Status {
		case "completed":
			if run.Conclusion == "success" {
				p.Logger.Info("Workflow completed successfully")
				return nil
			}
			return fmt.Errorf("workflow failed with conclusion: %s", run.Conclusion)
		case "queued", "in_progress":
			p.Logger.Infof("Workflow is %s...", run.Status)
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			p.Logger.Warnf("Unknown workflow status: %s", run.Status)
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (p *Puller) copyAndImportImage(destImageID, sourceID string) error {
	tmpFile, err := os.CreateTemp("", "image-copier-*.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	creds := fmt.Sprintf("%s:%s", p.Config.RegistryUsername, p.Config.RegistryPassword)
	cmd := exec.Command("skopeo", "copy", "--src-creds="+creds,
		"docker://"+destImageID, "docker-archive:"+tmpPath+":"+sourceID)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy failed: %s, output: %s", err, string(output))
	}

	p.notifyStage(StageLoadImage, 0)
	cmd = exec.Command("docker", "load", "-i", tmpPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker load failed: %s, output: %s", err, string(output))
	}

	return nil
}
