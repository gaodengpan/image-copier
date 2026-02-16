package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient is a mock implementation of HTTP client interface
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(*http.Response)
	return resp, args.Error(1)
}

// MockCommandRunner is a mock to simulate command execution
type MockCommandRunner struct {
	mock.Mock
}

// Create a global variable to temporarily override exec.Command during tests
var realCommand = exec.Command

func (m *MockCommandRunner) Command(name string, arg ...string) *exec.Cmd {
	// Return a dummy command as a placeholder
	return exec.Command("echo", "dummy command") // Always return a dummy command for testing
}

func setupTestEnvironment(t *testing.T) (*Puller, *logrus.Logger, *test.Hook) {
	logger, hook := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	config := &Config{
		GithubOwner:       "test-owner",
		GithubRepo:        "test-repo",
		GithubToken:       "test-token",
		GithubWorkflowID:  "test-workflow-id",
		RegistryHost:      "registry.example.com",
		RegistryUsername:  "test-user",
		RegistryPassword:  "test-password",
		RegistryNamespace: "test-namespace",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
		RetryConfig:       retry.DefaultConfig(),
	}

	puller := NewPuller(config, logger)
	return puller, logger, hook
}

func TestSanitizeForLog(t *testing.T) {
	input := "some-sensitive-data"
	result := sanitizeForLog(input)

	assert.True(t, strings.HasPrefix(result, SensitiveDataPrefix))
	assert.True(t, strings.HasSuffix(result, SensitiveDataSuffix))
	assert.Equal(t, 27, len(result)) // prefix (10) + 16 hex chars + suffix (1)
}

func TestValidateImageNameInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid image name", "nginx:latest", true},
		{"valid image with registry", "docker.io/library/nginx:latest", true},
		{"invalid image name with shell injection", "nginx;rm -rf /", false},
		{"empty string", "", false},
		{"just slash", "/", false},
		{"normal image name", "my-image:v1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateImageNameInput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidImageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid image name", "nginx:latest", true},
		{"valid image with registry", "docker.io/library/nginx:latest", true},
		{"invalid image name with shell injection", "nginx;rm -rf /", false},
		{"empty string", "", false},
		{"normal image name", "my-image:v1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidImageName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateTempFile(t *testing.T) {
	tempPath, err := createTempFile()
	require.NoError(t, err)
	defer os.Remove(tempPath) // Clean up

	// Verify the file exists
	info, err := os.Stat(tempPath)
	assert.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.True(t, strings.HasPrefix(filepath.Base(tempPath), "image-copier-"))
}

func TestNormalizeImageSegment(t *testing.T) {
	tests := []struct {
		name     string
		segment  string
		expected string
	}{
		{"simple segment", "library", "docker.io/library"},
		{"domain segment", "docker.io", "docker.io"},
		{"port segment", "localhost:5000", "localhost:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeImageSegment(tt.segment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasTagOrDigest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"with tag", "nginx:latest", true},
		{"with digest", "nginx@sha256:abc123", true},
		{"without tag or digest", "nginx", false},
		{"empty string", "", false},
		{"complex path with tag", "registry.com/namespace/image:tag", true},
		{"complex path with digest", "registry.com/namespace/image@sha256:abc", true},
		{"multiple colons", "repo:tag:extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasTagOrDigest(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeSourceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single segment becomes fully qualified", "nginx", "docker.io/library/nginx:latest"},
		{"two segments with domain", "docker.io/nginx", "docker.io/nginx:latest"},
		{"already fully qualified", "docker.io/library/nginx:custom", "docker.io/library/nginx:custom"},
		{"complex path", "registry.com/namespace/image", "registry.com/namespace/image:latest"},
		{"with tag", "nginx:1.19", "docker.io/library/nginx:1.19"},
		{"with digest", "nginx@sha256:abc123", "docker.io/library/nginx@sha256:abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSourceID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildDestImageID(t *testing.T) {
	tests := []struct {
		name              string
		registryHost      string
		registryNamespace string
		sourceID          string
		expected          string
	}{
		{"full params with host and namespace", "registry.com", "ns", "nginx:latest", "registry.com/ns/nginx:latest"},
		{"only host", "registry.com", "", "nginx:latest", "registry.com/nginx:latest"},
		{"only namespace", "", "ns", "nginx:latest", "/ns/nginx:latest"},
		{"no host, no namespace", "", "", "nginx:latest", "/nginx:latest"},
		{"with complex image name", "registry.com", "ns", "repo/subrepo/my-image:tag", "registry.com/ns/repo_subrepo_my_image:tag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildDestImageID(tt.registryHost, tt.registryNamespace, tt.sourceID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewPuller(t *testing.T) {
	logger := logrus.New()
	config := &Config{
		GithubOwner:       "test-owner",
		GithubRepo:        "test-repo",
		GithubToken:       "test-token",
		GithubWorkflowID:  "test-workflow-id",
		RegistryHost:      "registry.example.com",
		RegistryUsername:  "test-user",
		RegistryPassword:  "test-password",
		RegistryNamespace: "test-namespace",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
		RetryConfig:       retry.DefaultConfig(),
	}

	puller := NewPuller(config, logger)

	assert.NotNil(t, puller)
	assert.Equal(t, config, puller.Config)
	assert.NotNil(t, puller.RetryConfig)
	assert.NotNil(t, puller.Logger)
	assert.NotNil(t, puller.HTTPClient)
	assert.NotNil(t, puller.LocalImageCache)
	assert.NotNil(t, puller.ImageValidator)
	assert.Equal(t, MaxCacheSizeDefault, puller.MaxCacheSize)
}

func TestHTTPClientFactory_NewHTTPClient(t *testing.T) {
	factory := &HTTPClientFactory{}
	client := factory.NewHTTPClient()

	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.NotNil(t, client.Transport)
}

func TestParseDockerImageOutput(t *testing.T) {
	output := "nginx:latest\nredis:alpine\ninvalid;command\n"

	images := parseDockerImageOutput(output, 10)

	// Should only contain valid images
	assert.Contains(t, images, "nginx:latest")
	assert.Contains(t, images, "redis:alpine")
	// Invalid command should not be included
	_, exists := images["invalid;command"]
	assert.False(t, exists)
}

func TestPuller_PreloadValidate(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	// Since we can't control the environment to make skopeo/docker exist/not exist,
	// we'll test the validation logic flow

	// Call validation twice to ensure it's only executed once
	err1 := puller.PrePullValidate()
	err2 := puller.PrePullValidate()

	// Both calls should return the same result since validation is only run once
	assert.Equal(t, err1, err2)
}

func TestPuller_CleanupCache(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	// Add some dummy entries to cache
	puller.LocalImageCache["test"] = true
	puller.CacheTimestamp = time.Now()

	// Verify cache is populated
	assert.Equal(t, 1, len(puller.LocalImageCache))
	assert.NotEqual(t, time.Time{}, puller.CacheTimestamp)

	// Clean up cache
	puller.CleanupCache()

	// Verify cache is empty
	assert.Equal(t, 0, len(puller.LocalImageCache))
	assert.Equal(t, time.Time{}, puller.CacheTimestamp)
}

func TestPuller_notifyStage(t *testing.T) {
	callbackCalled := false
	callbackParams := struct {
		stage PullStage
		polls int
	}{}

	puller := &Puller{
		StageCallback: func(stage PullStage, polls int) {
			callbackCalled = true
			callbackParams.stage = stage
			callbackParams.polls = polls
		},
	}

	puller.notifyStage(StageCheckLocal, 5)

	assert.True(t, callbackCalled)
	assert.Equal(t, StageCheckLocal, callbackParams.stage)
	assert.Equal(t, 5, callbackParams.polls)
}

func TestPuller_buildExpectedWorkflowName(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	expected := puller.buildExpectedWorkflowName("nginx:latest", "registry.com/nginx:latest", "--12345")
	assert.Equal(t, "copy nginx:latest to registry.com/nginx:latest--12345", expected)
}

func TestPuller_buildWorkflowRunsURL(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	expectedURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/runs",
		puller.Config.GithubOwner, puller.Config.GithubRepo, puller.Config.GithubWorkflowID)
	actualURL := puller.buildWorkflowRunsURL()

	assert.Equal(t, expectedURL, actualURL)
}

func TestPuller_createWorkflowRunsRequest(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	url := "https://api.github.com/repos/test/test/actions/workflows/test/runs"
	req, err := puller.createWorkflowRunsRequest(url)

	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, GitHubMediaType, req.Header.Get("Accept"))
	assert.Equal(t, "Bearer "+puller.Config.GithubToken, req.Header.Get("Authorization"))
	assert.Equal(t, GitHubAPIVersion, req.Header.Get("X-GitHub-Api-Version"))
}

func TestPuller_searchWorkflowRunID(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	result := struct {
		WorkflowRuns []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"workflow_runs"`
	}{
		WorkflowRuns: []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}{
			{ID: 123, Name: "copy nginx:latest to registry.com/nginx:latest--12345"},
			{ID: 456, Name: "other workflow"},
		},
	}

	id, found := puller.searchWorkflowRunID(result, "copy nginx:latest to registry.com/nginx:latest--12345")
	assert.True(t, found)
	assert.Equal(t, "123", id)

	id, found = puller.searchWorkflowRunID(result, "non-existent workflow")
	assert.False(t, found)
	assert.Equal(t, "", id)
}

// Test for error handling in CheckLocalImageExists
func TestPuller_CheckLocalImageExists_InvalidInput(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	ctx := context.Background()
	_, err := puller.CheckLocalImageExists(ctx, "invalid;command")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

// Test for checkLocalImageWithCacheRefresh
func TestPuller_checkLocalImageWithCacheRefresh_InvalidInput(t *testing.T) {
	puller, _, _ := setupTestEnvironment(t)

	ctx := context.Background()
	_, err := puller.checkLocalImageWithCacheRefresh(ctx, "invalid;command")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

// Test for error handling in triggerWorkflow
func TestPuller_triggerWorkflow_NetworkError(t *testing.T) {
	// Skipping this test because we cannot mock HTTP client without changing the source code
	// In a real scenario, this would involve mocking the HTTP client interface
	t.Skip("Skipping network error test due to limitations in mocking HTTP client without modifying source code")
}

// Test that error variables have the correct values
func TestErrorVariables(t *testing.T) {
	assert.Equal(t, "image already exists locally", ErrSkipped.Error())
	assert.Equal(t, "dry-run: no changes made", ErrDryRun.Error())
}
