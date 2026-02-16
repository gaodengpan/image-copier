package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/ports"
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
	StageDownloadImage                    // Download image from registry to local file
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
	MaxCacheSize    int                        // Maximum number of entries in the cache
	refreshMutex    sync.Mutex                 // Mutex for preventing concurrent cache refreshes
	ImageValidator  *validators.ImageValidator // Validator for image names and credentials

	// Port interfaces for external dependencies
	DockerClient   ports.DockerClient
	RegistryClient ports.RegistryClient
	GitHubClient   ports.GitHubClient
	FileSystem     ports.FileSystem

	// Validation cache fields
	validationOnce    sync.Once // Ensures validation runs only once
	validationResult  error     // Stores the result of validation
	validationChecked bool      // Tracks whether validation has been performed
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
	return services.SanitizeForLog(input)
}

// validateImageNameInput validates an image name to prevent command injection
func validateImageNameInput(name string) bool {
	validator := validators.NewImageValidator()
	return validator.ValidateImageNameInput(name)
}

// isValidImageName validates an image name to prevent command injection
func isValidImageName(name string) bool {
	validator := validators.NewImageValidator()
	return validator.IsValidImageName(name)
}

// createTempFile creates a temporary file for image operations
func (p *Puller) createTempFile() (string, error) {
	return p.FileSystem.CreateTempFile("image-copier-*.tar")
}

// downloadImageFromRegistry downloads an image from registry to a local file
func (p *Puller) downloadImageFromRegistry(ctx context.Context, registryImageID, userImageTag, outputPath, username, password string) error {
	return p.RegistryClient.SaveImageToFile(ctx, registryImageID, userImageTag, outputPath, username, password)
}

// executeDockerLoad executes the docker load command
func (p *Puller) executeDockerLoad(ctx context.Context, dockerCmd, tmpPath string) error {
	return p.DockerClient.LoadImage(ctx, tmpPath)
}

// CheckLocalImageExists checks whether the given image is available in the local Docker daemon.
func (p *Puller) CheckLocalImageExists(ctx context.Context, imageID string) (bool, error) {
	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(imageID) {
		return false, fmt.Errorf("%s: %s", "invalid image name", sanitizeForLog(imageID))
	}

	// Check if the exact imageID exists in cache
	p.cacheMutex.RLock()
	if cachedResult, exists := p.LocalImageCache[imageID]; exists {
		// Check if cache is still valid (less than 30 seconds old)
		if time.Since(p.CacheTimestamp) < DefaultCacheTTL {
			p.cacheMutex.RUnlock()
			return cachedResult, nil
		}
	}
	p.cacheMutex.RUnlock()

	// Try to find image under various alias forms
	// First check with the normalized form that matches Docker's canonical format
	normalizedID := NormalizeSourceID(imageID)

	// If the normalized ID is different from the original, check if either exists
	if normalizedID != imageID {
		// Check cache for the normalized ID
		p.cacheMutex.RLock()
		if cachedResult, exists := p.LocalImageCache[normalizedID]; exists {
			if time.Since(p.CacheTimestamp) < DefaultCacheTTL {
				p.cacheMutex.RUnlock()
				return cachedResult, nil
			}
		}
		p.cacheMutex.RUnlock()
	}

	// Need to refresh cache and check both forms
	return p.checkLocalImageWithCacheRefreshAndAliases(ctx, imageID, normalizedID)
}

// checkLocalImageWithCacheRefreshAndAliases performs the actual check and refreshes the cache if needed
// It checks for image existence considering both the original and normalized image names
func (p *Puller) checkLocalImageWithCacheRefreshAndAliases(ctx context.Context, originalID, normalizedID string) (bool, error) {
	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(originalID) {
		return false, fmt.Errorf("%s: %s", "invalid image name", sanitizeForLog(originalID))
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

			// Use DockerClient to check individual image
			exists, err := p.DockerClient.ImageExists(ctx, originalID)
			if err == nil && exists {
				return true, nil
			}

			// Check normalized ID
			exists, err = p.DockerClient.ImageExists(ctx, normalizedID)
			if err == nil && exists {
				return true, nil
			}

			return false, fmt.Errorf("failed to check image existence: %w", err)
		}

		// Update the cache with refreshed data
		p.cacheMutex.Lock()
		p.LocalImageCache = allImages
		p.CacheTimestamp = time.Now()
		p.cacheMutex.Unlock()
	}

	// Now check the cached result for both the original and normalized forms
	p.cacheMutex.RLock()
	originalExists := p.LocalImageCache[originalID]
	normalizedExists := p.LocalImageCache[normalizedID]
	p.cacheMutex.RUnlock()

	// If either form exists in the cache, return true
	return originalExists || normalizedExists, nil
}

// checkLocalImageWithCacheRefresh performs the actual check and refreshes the cache if needed
func (p *Puller) checkLocalImageWithCacheRefresh(ctx context.Context, imageID string) (bool, error) {
	// Validate input to prevent command injection
	if !p.ImageValidator.IsValidImageName(imageID) {
		return false, fmt.Errorf("%s: %s", "invalid image name", sanitizeForLog(imageID))
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

			// Use DockerClient to check individual image
			exists, err := p.DockerClient.ImageExists(ctx, imageID)
			if err != nil {
				return false, fmt.Errorf("failed to check image existence: %w", err)
			}
			return exists, nil
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

// getAllLocalImages gets all local images and returns them as a map for fast lookup
func (p *Puller) getAllLocalImages(ctx context.Context) (map[string]bool, error) {
	images, err := p.DockerClient.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for _, img := range images {
		result[img] = true
	}
	return result, nil
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

// NewPullerWithPorts creates a new Puller instance with explicit port implementations
func NewPullerWithPorts(config *Config, logger *logrus.Logger,
	dockerClient ports.DockerClient,
	registryClient ports.RegistryClient,
	githubClient ports.GitHubClient,
	fs ports.FileSystem) *Puller {

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
		CacheTimestamp:  time.Time{},
		cacheMutex:      sync.RWMutex{},
		MaxCacheSize:    MaxCacheSizeDefault,
		refreshMutex:    sync.Mutex{},
		ImageValidator:  validators.NewImageValidator(),

		DockerClient:   dockerClient,
		RegistryClient: registryClient,
		GitHubClient:   githubClient,
		FileSystem:     fs,
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
		localExists, err := p.CheckLocalImageExists(ctx, imageID)
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
	exists, err := p.checkImageExists(ctx, destImageID, p.Config.RegistryUsername, p.Config.RegistryPassword)
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

	// Download image from registry and load to Docker
	p.notifyStage(StageDownloadImage, 0)
	if err := p.downloadAndLoadImage(ctx, destImageID, imageID, p.Config.RegistryUsername, p.Config.RegistryPassword); err != nil {
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
	// Extract tag and digest before normalization to preserve them intact
	var tag, digest, imageName string

	// Find digest part (contains @)
	digestIndex := strings.LastIndex(sourceID, "@")
	if digestIndex != -1 {
		digest = sourceID[digestIndex:] // includes @
		imageName = sourceID[:digestIndex]
	} else {
		imageName = sourceID
	}

	// Find tag part (contains :)
	if digestIndex == -1 {
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:] // includes :
			imageName = imageName[:tagIndex]
		}
	} else {
		// Check for tag in the part before digest
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:] // includes :
			imageName = imageName[:tagIndex]
		}
	}

	// If host is empty, always normalize the source ID to avoid path issues
	if registryHost == "" {
		// Replace slashes, colons, and hyphens with underscores to avoid issues with Docker image names
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		// Calculate max length accounting for tag and digest
		maxBaseLen := MaxNormalizedLen
		if tag != "" {
			maxBaseLen -= len(tag)
		}
		if digest != "" {
			maxBaseLen -= len(digest)
		}

		// Ensure maxBaseLen is not negative
		if maxBaseLen < 0 {
			maxBaseLen = 0
		}

		if len(normalized) > maxBaseLen {
			normalized = normalized[:maxBaseLen]
		}

		// Remove trailing underscores to ensure valid Docker image name
		normalized = strings.TrimRight(normalized, "_")

		// Append tag and digest back
		normalized = normalized + tag + digest

		if registryNamespace == "" {
			return fmt.Sprintf("/%s", normalized)
		}
		return fmt.Sprintf("/%s/%s", registryNamespace, normalized)
	}

	// If host is not empty
	if registryNamespace == "" {
		// Replace slashes, colons, and hyphens with underscores to avoid issues with Docker image names
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		// Calculate max length accounting for tag and digest
		maxBaseLen := MaxNormalizedLen
		if tag != "" {
			maxBaseLen -= len(tag)
		}
		if digest != "" {
			maxBaseLen -= len(digest)
		}

		// Ensure maxBaseLen is not negative
		if maxBaseLen < 0 {
			maxBaseLen = 0
		}

		if len(normalized) > maxBaseLen {
			normalized = normalized[:maxBaseLen]
		}

		// Remove trailing underscores to ensure valid Docker image name
		normalized = strings.TrimRight(normalized, "_")

		// Append tag and digest back
		normalized = normalized + tag + digest

		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	// Host and namespace are both non-empty, normalize the source ID
	normalized := strings.ReplaceAll(imageName, "/", "_")
	normalized = strings.ReplaceAll(normalized, ":", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	// Calculate max length accounting for tag and digest
	maxBaseLen := MaxNormalizedLen
	if tag != "" {
		maxBaseLen -= len(tag)
	}
	if digest != "" {
		maxBaseLen -= len(digest)
	}

	// Ensure maxBaseLen is not negative
	if maxBaseLen < 0 {
		maxBaseLen = 0
	}

	if len(normalized) > maxBaseLen {
		normalized = normalized[:maxBaseLen]
	}

	// Remove trailing underscores to ensure valid Docker image name
	normalized = strings.TrimRight(normalized, "_")

	// Append tag and digest back
	normalized = normalized + tag + digest

	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}

// checkImageExists is a method on Puller that uses the RegistryClient port
func (p *Puller) checkImageExists(ctx context.Context, destImageID, username, password string) (bool, error) {
	return p.RegistryClient.ImageExists(ctx, destImageID, username, password)
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

func (p *Puller) downloadAndLoadImage(ctx context.Context, registryImageID, userImageTag, username, password string) error {
	if !p.ImageValidator.IsValidImageName(registryImageID) {
		return fmt.Errorf("%s: %s", "invalid image name", sanitizeForLog(registryImageID))
	}

	tmpPath, err := p.createTempFile()
	if err != nil {
		return err
	}

	cleanup := func() {
		if err := p.FileSystem.RemoveFile(tmpPath); err != nil {
			p.Logger.Warnf("Failed to remove temp file %s: %v", tmpPath, err)
		}
	}
	defer cleanup()

	if err := p.downloadImageFromRegistry(ctx, registryImageID, userImageTag, tmpPath, username, password); err != nil {
		return err
	}

	p.notifyStage(StageLoadImage, 0)

	if err := p.executeDockerLoad(ctx, DockerCommand, tmpPath); err != nil {
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
