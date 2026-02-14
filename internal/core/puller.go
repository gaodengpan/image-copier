package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gaodengpan/image-copier/internal/utils"
	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

// Define constants - Updated to English comments
const (
	// Using constants from constants.go
)

// 正则 expression precompiled - removed as it's now in validation.go

// PullStage represents a stage in the image pull pipeline.
type PullStage int

const (
	StageCheckLocal      PullStage = iota // Check local image
	StageCheckRegistry                    // Check remote registry
	StageTriggerWorkflow                  // Trigger GitHub Workflow
	StageWaitWorkflow                     // Wait for Workflow completion
	StageCopyImage                        // skopeo copy (download image)
	StageLoadImage                        // docker load (import locally)
)

// StageCallback is called when PullSingle transitions between stages.
// polls is only meaningful for StageWaitWorkflow — it carries the current poll count.
type StageCallback func(stage PullStage, polls int)

// Puller handles the image pulling process
type Puller struct {
	Config          *Config
	RetryConfig     *retry.Config
	Logger          *logrus.Logger
	StageCallback   StageCallback
	HTTPClient      *http.Client
	LocalImageCache map[string]bool
	CacheTimestamp  time.Time
	cacheMutex      sync.RWMutex
	MaxCacheSize    int // Maximum number of entries in the cache
	refreshMutex    sync.Mutex // Mutex for preventing concurrent cache refreshes
	ImageValidator  *ImageValidator // Validator for image names and credentials
	// Validation cache fields
	validationOnce    sync.Once       // Ensures validation runs only once
	validationResult  error           // Stores the result of validation
	validationChecked bool          // Tracks whether validation has been performed
}

// Config holds the configuration needed for Puller
type Config struct {
	GithubOwner       string
	GithubRepo        string
	GithubToken       string
	GithubWorkflowID  string
	RegistryHost      string
	RegistryUsername  string
	RegistryPassword  string
	RegistryNamespace string
	RegistryArch      string
	RegistryOs        string
	Force             bool
	RetryConfig       *retry.Config
	DryRun            bool
}

// ErrSkipped indicates an image was skipped because it already exists locally.
var ErrSkipped = fmt.Errorf("image already exists locally")

// ErrDryRun indicates no changes were made because dry-run mode is enabled.
var ErrDryRun = fmt.Errorf("dry-run: no changes made")

// sanitizeForLog sanitizes potentially sensitive information before logging
func sanitizeForLog(input string) string {
	// Create a hash of the input to use for logging
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%s%x%s", SensitiveDataPrefix, hash[:8], SensitiveDataSuffix)
}

// validateImageNameInput validates an image name to prevent command injection
func validateImageNameInput(name string) bool {
	validator := NewImageValidator()
	return validator.ValidateImageNameInput(name)
}

// isValidImageName validates an image name to prevent command injection
func isValidImageName(name string) bool {
	validator := NewImageValidator()
	return validator.IsValidImageName(name)
}

// createTempFile creates a temporary file for image operations
func createTempFile() (string, error) {
	tmpFile, err := os.CreateTemp("", "image-copier-*.tar")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	return tmpPath, nil
}

// executeSkopeoCopy executes the skopeo copy command
func executeSkopeoCopy(ctx context.Context, skopeoCmd, creds, destImageID, tmpPath, sourceID string) error {
	skopeoCtx, cancel := context.WithTimeout(ctx, SkopeoCopyTimeout)
	defer cancel()

	cmd := exec.CommandContext(skopeoCtx, skopeoCmd, "copy", "--src-creds="+creds,
		"docker://"+destImageID, "docker-archive:"+tmpPath+":"+sourceID)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s, output: %s", ErrCommandFailed, err, string(output))
	}
	return nil
}

// executeDockerLoad executes the docker load command
func executeDockerLoad(ctx context.Context, dockerCmd, tmpPath string) error {
	loadCtx, loadCancel := context.WithTimeout(ctx, DockerLoadTimeout)
	defer loadCancel()

	cmd := exec.CommandContext(loadCtx, dockerCmd, "load", "-i", tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s, output: %s", ErrCommandFailed, err, string(output))
	}
	return nil
}

// normalizeImageSegment normalizes a single segment of an image name
func normalizeImageSegment(segment string) string {
	if !strings.Contains(segment, ".") && !strings.Contains(segment, ":") {
		// Not a domain, prepend docker.io
		return fmt.Sprintf("docker.io/%s", segment)
	}
	return segment
}

// hasTagOrDigest checks if the tail segment of a string contains a tag or digest
func hasTagOrDigest(s string) bool {
	if s == "" {
		return false
	}

	// Get the last segment after splitting by '/'
	parts := strings.Split(s, "/")
	tailSegment := parts[len(parts)-1]  // Get the last part

	// If tail segment contains @ (for digest), return true
	if strings.Contains(tailSegment, "@") {
		return true
	}

	// Split the tail segment by ':' to analyze the format
	colonParts := strings.Split(tailSegment, ":")

	// If more than one colon (e.g., "name:tag:something"), likely a tag format, return true
	if len(colonParts) > 2 {
		return true
	}

	// If exactly one colon (e.g., "name:tag")
	if len(colonParts) == 2 {
		// If the original string had multiple path segments (e.g., "repo/image:tag"),
		// then treat the colon as part of the path, not as a tag separator
		if len(parts) > 1 {
			return false
		}
		// If it's just a simple "name:tag" without path segments, it's a tag format
		return true
	}

	// No colon found
	return false
}

// CheckLocalImageExists checks whether the given image is available in the local Docker daemon.
func (p *Puller) CheckLocalImageExists(ctx context.Context, imageID string) (bool, error) {
	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(imageID) {
		return false, fmt.Errorf("%s: %s", ErrInvalidImageName, sanitizeForLog(imageID))
	}

	// Check cache first
	p.cacheMutex.RLock()
	if cachedResult, exists := p.LocalImageCache[imageID]; exists {
		// Check if cache is still valid (less than 30 seconds old)
		if time.Since(p.CacheTimestamp) < DefaultCacheTTL {
			p.cacheMutex.RUnlock()
			return cachedResult, nil
		}
	}
	p.cacheMutex.RUnlock()

	// Need to refresh cache
	return p.checkLocalImageWithCacheRefresh(ctx, imageID)
}

// checkLocalImageWithCacheRefresh performs the actual check and refreshes the cache if needed
func (p *Puller) checkLocalImageWithCacheRefresh(ctx context.Context, imageID string) (bool, error) {
	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(imageID) {
		return false, fmt.Errorf("%s: %s", ErrInvalidImageName, sanitizeForLog(imageID))
	}

	// Use refreshMutex to prevent multiple goroutines from refreshing cache simultaneously
	p.refreshMutex.Lock()
	defer p.refreshMutex.Unlock()

	// Double-check condition after acquiring write lock
	p.cacheMutex.Lock()
	needsRefresh := time.Since(p.CacheTimestamp) >= DefaultCacheTTL ||
		len(p.LocalImageCache) == 0 ||
		len(p.LocalImageCache) >= p.MaxCacheSize // Check if cache is too large
	p.cacheMutex.Unlock()

	if needsRefresh {
		// Refresh the entire cache
		allImages, err := p.getAllLocalImages(ctx)
		if err != nil {
			p.Logger.Warnf("Failed to refresh local image cache: %v", err)

			// Fallback to individual check if unable to refresh cache
			// Use context with timeout to prevent hanging
			ctx, cancel := context.WithTimeout(ctx, CheckLocalTimeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, DockerCommand, "image", "inspect", imageID)
			cmd.Stdout = nil
			cmd.Stderr = nil
			err := cmd.Run()
			if err != nil {
				// Explicitly log the error from getAllLocalImages to avoid silent failures
				return false, fmt.Errorf("primary cache refresh failed (%w), fallback individual check also failed", err)
			}
			return true, nil
		}

		// Update the cache with refreshed data
		p.cacheMutex.Lock()
		p.LocalImageCache = allImages
		p.CacheTimestamp = time.Now()
		p.cacheMutex.Unlock()
	}

	// Now check the cached result
	p.cacheMutex.RLock()
	exists := p.LocalImageCache[imageID]
	p.cacheMutex.RUnlock()
	return exists, nil
}

// CleanupCache removes the local image cache to free memory (useful for long-running operations)
func (p *Puller) CleanupCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	p.LocalImageCache = make(map[string]bool)
	p.CacheTimestamp = time.Time{}
}

// parseDockerImageOutput parses the output from docker image ls command
func parseDockerImageOutput(output string, maxCacheSize int) map[string]bool {
	images := make(map[string]bool)
	validator := NewImageValidator() // Create a validator instance to use its validation methods
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && validator.ValidateImageNameInput(line) && count < maxCacheSize { // Only add valid image names to cache and respect size limit
			images[line] = true
			count++
		}
	}
	return images
}

// getAllLocalImages gets all local images and returns them as a map for fast lookup
func (p *Puller) getAllLocalImages(ctx context.Context) (map[string]bool, error) {
	// Validate that we're not running this command with unsafe inputs
	// Although we're not accepting user input here, this is a good safety check
	ctx, cancel := context.WithTimeout(ctx, ListImagesTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, DockerCommand, "image", "ls", "--format", DockerImageFormat)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list local images: %w", err)
	}

	images := parseDockerImageOutput(string(output), p.MaxCacheSize)
	return images, nil
}

// HTTPClientFactory creates HTTP clients with common settings
type HTTPClientFactory struct{}

// NewHTTPClient creates a new HTTP client with common configuration
func (f *HTTPClientFactory) NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// NewPuller creates a new Puller instance
func NewPuller(config *Config, logger *logrus.Logger) *Puller {
	rc := config.RetryConfig
	if rc == nil {
		rc = retry.DefaultConfig()
	}

	httpClient := (&HTTPClientFactory{}).NewHTTPClient()

	return &Puller{
		Config:          config,
		RetryConfig:     rc,
		Logger:          logger,
		HTTPClient:      httpClient,
		LocalImageCache: make(map[string]bool),
		CacheTimestamp:  time.Time{}, // Zero time initially
		cacheMutex:      sync.RWMutex{},
		MaxCacheSize:    MaxCacheSizeDefault, // Use default maximum cache size
		refreshMutex:    sync.Mutex{},        // Initialize mutex for cache refresh synchronization
		ImageValidator:  NewImageValidator(), // Initialize image validator
	}
}

func (p *Puller) notifyStage(stage PullStage, polls int) {
	if p.StageCallback != nil {
		p.StageCallback(stage, polls)
	}
}

// PullSingle pulls a single image through GitHub Actions
func (p *Puller) PullSingle(ctx context.Context, imageID string) error {
	// Run pre-pull validation first
	if err := p.PrePullValidate(); err != nil {
		return err
	}

	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(imageID) {
		return fmt.Errorf("invalid image name: %s", sanitizeForLog(imageID))
	}

	p.Logger.Infof("Processing image: %s", sanitizeForLog(imageID))

	sourceID := NormalizeSourceID(imageID)
	destImageID := BuildDestImageID(p.Config.RegistryHost, p.Config.RegistryNamespace, sourceID)

	// Check if image already exists in the local Docker daemon
	p.notifyStage(StageCheckLocal, 0)
	if !p.Config.Force {
		localExists, err := p.CheckLocalImageExists(ctx, sourceID)
		if err != nil {
			// Log the error from CheckLocalImageExists to prevent silent failures
			p.Logger.Errorf("Error checking local image: %v", err)
			return fmt.Errorf("failed to check local image: %w", err)
		} else if localExists {
			p.Logger.Infof("Image %s already exists locally, skipping (use --force to override)", sanitizeForLog(sourceID))
			return ErrSkipped
		}
	}

	// Check if image already exists
	p.notifyStage(StageCheckRegistry, 0)
	exists, err := CheckImageExists(ctx, destImageID, p.Config.RegistryUsername, p.Config.RegistryPassword)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	if p.Config.DryRun {
		if exists {
			p.Logger.Infof("[dry-run] %s → %s: image exists in registry, would copy and load", sanitizeForLog(sourceID), sanitizeForLog(destImageID))
		} else {
			p.Logger.Infof("[dry-run] %s → %s: would trigger workflow, then copy and load", sanitizeForLog(sourceID), sanitizeForLog(destImageID))
		}
		return ErrDryRun
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
	if err := p.copyAndImportImage(ctx, destImageID, sourceID); err != nil {
		return fmt.Errorf("failed to copy and import image: %w", err)
	}

	p.Logger.Infof("Successfully processed image: %s", sanitizeForLog(imageID))
	return nil
}

// NormalizeSourceID normalizes a user-provided image ID to a fully-qualified
// source reference (e.g. "nginx" → "docker.io/library/nginx:latest").
func NormalizeSourceID(imageID string) string {
	segs := strings.Split(imageID, "/")

	var normalized string
	switch len(segs) {
	case 1:
		// No registry specified, assume docker.io/library
		normalized = fmt.Sprintf("docker.io/library/%s", imageID)
	case 2:
		// Check if first segment looks like a domain
		normalized = normalizeImageSegment(segs[0]) + "/" + segs[1]
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
	if !hasTagOrDigest(tail) {
		normalized += ":latest"
	}

	return normalized
}

// BuildDestImageID constructs the destination registry image path from a
// normalized source ID and registry configuration.
func BuildDestImageID(registryHost, registryNamespace, sourceID string) string {
	// If host is empty, always normalize the source ID to avoid path issues
	if registryHost == "" {
		// Replace slashes, colons, dots, and hyphens with underscores to avoid issues with Docker image names
		normalized := strings.ReplaceAll(sourceID, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")
		if len(normalized) > MaxNormalizedLen {
			normalized = normalized[:MaxNormalizedLen]
		}

		if registryNamespace == "" {
			return fmt.Sprintf("/%s", normalized)
		}
		return fmt.Sprintf("/%s/%s", registryNamespace, normalized)
	}

	// If host is not empty
	if registryNamespace == "" {
		// Replace slashes, colons, dots, and hyphens with underscores to avoid issues with Docker image names
		normalized := strings.ReplaceAll(sourceID, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")
		if len(normalized) > MaxNormalizedLen {
			normalized = normalized[:MaxNormalizedLen]
		}
		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	// Host and namespace are both non-empty, normalize the source ID
	normalized := strings.ReplaceAll(sourceID, "/", "_")
	normalized = strings.ReplaceAll(normalized, ":", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if len(normalized) > MaxNormalizedLen {
		normalized = normalized[:MaxNormalizedLen]
	}

	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}

// CheckImageExists checks if an image exists in a registry using skopeo.
func CheckImageExists(ctx context.Context, destImageID, username, password string) (bool, error) {
	// Validate inputs to prevent command injection
	validator := NewImageValidator() // Create a validator instance to use its validation methods
	if !validator.IsValidImageName(destImageID) {
		return false, fmt.Errorf("%s: %s", ErrInvalidImageName, sanitizeForLog(destImageID))
	}

	// Validate username and password to ensure they don't contain command injection chars
	if !validator.ValidateCredentials(username, password) {
		return false, fmt.Errorf(ErrInvalidCredentials)
	}

	creds := fmt.Sprintf("%s%s%s", username, CredentialsSeparator, password)

	// Create context with timeout to prevent indefinite hanging
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, SkopeoCommand, "inspect", "--creds="+creds, "docker://"+destImageID)

	_, err := cmd.Output()
	if err != nil {
		// If command fails or times out, assume image doesn't exist
		return false, nil
	}

	return true, nil
}

func (p *Puller) triggerWorkflow(ctx context.Context, sourceID, destImageID string) (string, error) {
	suffix := fmt.Sprintf("--%d", time.Now().Unix())

	data := map[string]interface{}{
		"ref": "master",
		"inputs": map[string]string{
			"imageId":     sourceID,
			"destImageId": destImageID,
			"suffix":      suffix,
			"arch":        p.Config.RegistryArch,
			"os":          p.Config.RegistryOs,
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

		req.Header.Set("Accept", GitHubMediaType)
		req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
		req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			return retry.NewRetryableError(fmt.Errorf("failed to send request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return nil
		}

		// Log the response status for debugging (but mask sensitive data)
		p.Logger.Debugf("GitHub API response status: %d", resp.StatusCode)

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

	p.Logger.Infof("Triggered workflow run ID: %s", sanitizeForLog(runID))
	return runID, nil
}

// Helper function to build the expected workflow name
func (p *Puller) buildExpectedWorkflowName(sourceID, destImageID, suffix string) string {
	return fmt.Sprintf("copy %s to %s%s", sourceID, destImageID, suffix)
}

// Helper function to build the GitHub API URL for listing workflow runs
func (p *Puller) buildWorkflowRunsURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs",
		p.Config.GithubOwner, p.Config.GithubRepo, p.Config.GithubWorkflowID)
}

// Helper function to create the HTTP request for getting workflow runs
func (p *Puller) createWorkflowRunsRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", GitHubMediaType)
	req.Header.Set("Authorization", "Bearer "+p.Config.GithubToken)
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)

	return req, nil
}

// Helper function to search for the workflow run ID in the API response
func (p *Puller) searchWorkflowRunID(result struct {
	WorkflowRuns []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workflow_runs"`
}, expectedName string) (string, bool) {
	for _, run := range result.WorkflowRuns {
		if run.Name == expectedName {
			return fmt.Sprintf("%d", run.ID), true
		}
	}
	return "", false
}

func (p *Puller) findWorkflowRunID(ctx context.Context, sourceID, destImageID, suffix string) (string, error) {
	expectedName := p.buildExpectedWorkflowName(sourceID, destImageID, suffix)
	url := p.buildWorkflowRunsURL()

	// Poll for up to 30 seconds
	timeoutCtx, cancel := context.WithTimeout(ctx, WorkflowPollTimeout)
	defer cancel()

	ticker := time.NewTimer(0)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("workflow run not found after 30 seconds")
		case <-ticker.C:
			req, err := p.createWorkflowRunsRequest(url)
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := p.HTTPClient.Do(req)
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

				runID, found := p.searchWorkflowRunID(result, expectedName)
				if found {
					return runID, nil
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

			resp, err := p.HTTPClient.Do(req)
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

func (p *Puller) copyAndImportImage(ctx context.Context, destImageID, sourceID string) error {
	// Validate inputs to prevent command injection
	if !p.ImageValidator.IsValidImageName(destImageID) {
		return fmt.Errorf("%s: %s", ErrInvalidImageName, sanitizeForLog(destImageID))
	}
	if !p.ImageValidator.IsValidImageName(sourceID) {
		return fmt.Errorf("%s: %s", ErrInvalidImageName, sanitizeForLog(sourceID))
	}

	// Create temporary file
	tmpPath, err := createTempFile()
	if err != nil {
		return err
	}

	// Register cleanup function
	cleanup := func() {
		if err := os.Remove(tmpPath); err != nil {
			p.Logger.Warnf("Failed to remove temp file %s: %v", tmpPath, err)
		}
	}

	// Ensure cleanup happens even if function exits early
	defer cleanup()

	// Validate credentials to prevent command injection
	username := p.Config.RegistryUsername
	password := p.Config.RegistryPassword
	if !p.ImageValidator.ValidateCredentials(username, password) {
		return fmt.Errorf(ErrInvalidCredentials)
	}

	creds := fmt.Sprintf("%s%s%s", username, CredentialsSeparator, password)

	// Execute skopeo copy
	if err := executeSkopeoCopy(ctx, SkopeoCommand, creds, destImageID, tmpPath, sourceID); err != nil {
		return err
	}

	p.notifyStage(StageLoadImage, 0)

	// Execute docker load
	if err := executeDockerLoad(ctx, DockerCommand, tmpPath); err != nil {
		return err
	}

	return nil
}

// PrePullValidate checks if required commands and services are available before executing pull
func (p *Puller) PrePullValidate() error {
	var validationErrors []string

	// Check if validation has already been performed
	if p.validationChecked {
		return p.validationResult
	}

	p.Logger.Debug("Performing pre-pull validation...")

	// Perform validation only once
	p.validationOnce.Do(func() {
		// Check if skopeo command exists
		skopeoExists, err := utils.CheckCommandExists(SkopeoCommand)
		if err != nil {
			p.Logger.Errorf("Error checking skopeo command: %v", err)
			p.validationResult = fmt.Errorf("error checking skopeo command: %w", err)
			return
		}
		if !skopeoExists {
			validationErrors = append(validationErrors, fmt.Sprintf(
				"skopeo command not found in PATH. Please install skopeo:\n"+
					"- macOS: brew install skopeo\n"+
					"- Ubuntu/Debian: sudo apt-get install skopeo\n"+
					"- CentOS/RHEL: sudo yum install skopeo\n"+
					"- Arch Linux: sudo pacman -S skopeo"))
		} else {
			p.Logger.Debug("Skopeo command found in PATH")
		}

		// Check if docker command exists
		dockerExists, err := utils.CheckCommandExists(DockerCommand)
		if err != nil {
			p.Logger.Errorf("Error checking docker command: %v", err)
			p.validationResult = fmt.Errorf("error checking docker command: %w", err)
			return
		}
		if !dockerExists {
			validationErrors = append(validationErrors, fmt.Sprintf(
				"docker command not found in PATH. Please install Docker:\n"+
					"- Visit https://docs.docker.com/get-docker/ for installation instructions"))
		} else {
			p.Logger.Debug("Docker command found in PATH")
		}

		// If both commands exist, check Docker service status
		if skopeoExists && dockerExists {
			dockerRunning, err := utils.CheckDockerService()
			if err != nil {
				p.Logger.Warnf("Docker service check resulted in error: %v", err)
				validationErrors = append(validationErrors, fmt.Sprintf(
					"Docker service is not running or accessible: %v\n"+
						"- macOS/Linux: Start Docker Desktop or run 'sudo systemctl start docker'\n"+
						"- Windows: Start Docker Desktop\n"+
						"- Verify Docker works by running 'docker ps'", err))
			} else if !dockerRunning {
				p.Logger.Warn("Docker service is not running")
				validationErrors = append(validationErrors, fmt.Sprintf(
					"Docker service is not running:\n"+
						"- macOS/Linux: Start Docker Desktop or run 'sudo systemctl start docker'\n"+
						"- Windows: Start Docker Desktop\n"+
						"- Verify Docker works by running 'docker ps'"))
			} else {
				p.Logger.Debug("Docker service is running")
			}
		}

		// Store validation result
		if len(validationErrors) > 0 {
			p.validationResult = fmt.Errorf("validation failed:\n%s", strings.Join(validationErrors, "\n"))
		} else {
			p.validationResult = nil
			p.Logger.Debug("Pre-pull validation passed")
		}

		p.validationChecked = true
	})

	return p.validationResult
}