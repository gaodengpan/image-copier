package use_cases

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDockerClient struct {
	mock.Mock
}

func (m *mockDockerClient) ImageExists(ctx context.Context, imageID string) (bool, error) {
	args := m.Called(ctx, imageID)
	return args.Bool(0), args.Error(1)
}

func (m *mockDockerClient) ListImages(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDockerClient) LoadImage(ctx context.Context, tarPath string) error {
	args := m.Called(ctx, tarPath)
	return args.Error(0)
}

type mockRegistryClient struct {
	mock.Mock
}

func (m *mockRegistryClient) ImageExists(ctx context.Context, imageID, username, password string) (bool, error) {
	args := m.Called(ctx, imageID, username, password)
	return args.Bool(0), args.Error(1)
}

func (m *mockRegistryClient) SaveImageToFile(ctx context.Context, imageID, imageTag, outputPath, username, password string) error {
	args := m.Called(ctx, imageID, imageTag, outputPath, username, password)
	return args.Error(0)
}

func (m *mockRegistryClient) CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error) {
	args := m.Called(ctx, imageID, username, password)
	return args.Bool(0), args.Error(1)
}

func (m *mockRegistryClient) BuildDestImageID(sourceID, registryHost, registryNamespace string) string {
	args := m.Called(sourceID, registryHost, registryNamespace)
	return args.String(0)
}

type mockGitHubClient struct {
	mock.Mock
}

func (m *mockGitHubClient) TriggerWorkflow(ctx context.Context, owner, repo, workflowID string, inputs map[string]string) (string, error) {
	args := m.Called(ctx, owner, repo, workflowID, inputs)
	return args.String(0), args.Error(1)
}

func (m *mockGitHubClient) GetWorkflowStatus(ctx context.Context, owner, repo, runID string) (string, error) {
	args := m.Called(ctx, owner, repo, runID)
	return args.String(0), args.Error(1)
}

type mockFileSystem struct {
	mock.Mock
}

func (m *mockFileSystem) CreateTempFile(pattern string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}

func (m *mockFileSystem) RemoveFile(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

type mockLogger struct {
	mock.Mock
}

func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Info(args ...interface{})                  {}
func (m *mockLogger) Warn(args ...interface{})                  {}
func (m *mockLogger) Error(args ...interface{})                 {}

func TestPullSingleImageUseCase_Execute_InvalidImageName(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "invalid;command",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
	}

	_, err := uc.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestPullSingleImageUseCase_Execute_SkippedWhenLocalExists(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
	}

	output, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.True(t, output.Skipped)
}

func TestPullSingleImageUseCase_Execute_DryRun_ImageExistsInRegistry(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
		DryRun:       true,
	}

	output, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.True(t, output.DryRun)
}

func TestPullSingleImageUseCase_Execute_DryRun_ImageNotExists(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
		DryRun:       true,
	}

	output, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.True(t, output.DryRun)
}

func TestPullSingleImageUseCase_Execute_ImageExistsInRegistry(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)
	httpClient := &http.Client{}

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")
	registry.On("SaveImageToFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	docker.On("LoadImage", mock.Anything, mock.Anything).Return(nil)
	fs.On("CreateTempFile", mock.Anything).Return("/tmp/test.tar", nil)
	fs.On("RemoveFile", mock.Anything).Return(nil)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, httpClient, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
		DryRun:       false,
	}

	output, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.False(t, output.Skipped)
	github.AssertNotCalled(t, "TriggerWorkflow")
}

func TestPullSingleImageUseCase_Execute_ForcePull(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)
	httpClient := &http.Client{}

	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")
	registry.On("SaveImageToFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	docker.On("LoadImage", mock.Anything, mock.Anything).Return(nil)
	fs.On("CreateTempFile", mock.Anything).Return("/tmp/test.tar", nil)
	fs.On("RemoveFile", mock.Anything).Return(nil)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, httpClient, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        true,
		DryRun:       false,
	}

	output, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.False(t, output.Skipped)
	docker.AssertNotCalled(t, "ImageExists")
}

func TestPullSingleImageUseCase_Execute_CheckLocalError(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, errors.New("docker not running"))
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, errors.New("docker not running"))
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
	}

	_, err := uc.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check local image")
}

func TestPullSingleImageUseCase_Execute_CheckRegistryError(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("registry error"))
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
	}

	_, err := uc.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check if image exists")
}

func TestPullSingleImageUseCase_Execute_DownloadAndLoadFailure(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)
	httpClient := &http.Client{}

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")
	registry.On("SaveImageToFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("download failed"))
	fs.On("CreateTempFile", mock.Anything).Return("/tmp/test.tar", nil)
	fs.On("RemoveFile", mock.Anything).Return(nil)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, httpClient, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
		DryRun:       false,
	}

	_, err := uc.Execute(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy and import image")
}

func TestPullSingleImageUseCase_StageCallback(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)
	httpClient := &http.Client{}

	var capturedStages []PullStage
	stageCallback := func(stage PullStage, polls int) {
		capturedStages = append(capturedStages, stage)
	}

	docker.On("ImageExists", mock.Anything, "nginx:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/nginx:latest").Return(false, nil)
	registry.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/nginx:latest")
	registry.On("SaveImageToFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	docker.On("LoadImage", mock.Anything, mock.Anything).Return(nil)
	fs.On("CreateTempFile", mock.Anything).Return("/tmp/test.tar", nil)
	fs.On("RemoveFile", mock.Anything).Return(nil)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, httpClient, logger,
		"owner", "repo", "token", "workflow",
		stageCallback,
	)

	input := PullSingleImageInput{
		ImageID:      "nginx:latest",
		RegistryHost: "registry.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		RegistryNS:   "ns",
		Force:        false,
		DryRun:       false,
	}

	_, err := uc.Execute(context.Background(), input)
	assert.NoError(t, err)

	assert.Contains(t, capturedStages, StageCheckLocal)
	assert.Contains(t, capturedStages, StageCheckRegistry)
	assert.Contains(t, capturedStages, StageDownloadImage)
	assert.Contains(t, capturedStages, StageLoadImage)
}

func TestNewPullSingleImageUseCase(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	assert.NotNil(t, uc)
	assert.NotNil(t, uc.imageValidator)
	assert.IsType(t, &validators.ImageValidator{}, uc.imageValidator)
}

func TestPullSingleImageUseCase_CancellationDuringCheckLocal(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "redis:latest").Return(false, nil).Maybe()
	docker.On("ImageExists", mock.Anything, "docker.io/library/redis:latest").Return(false, nil).Maybe()
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.example.com/ns/redis:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := uc.Execute(ctx, PullSingleImageInput{
		ImageID:      "redis:latest",
		RegistryHost: "registry.example.com",
		RegistryUser: "user",
		RegistryPass: "pass",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestPullSingleImageUseCase_CancellationDuringCheckRegistry(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "redis:latest").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "docker.io/library/redis:latest").Return(false, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.example.com/ns/redis:latest")

	uc := NewPullSingleImageUseCase(
		docker, registry, github, fs, nil, logger,
		"owner", "repo", "token", "workflow",
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := uc.Execute(ctx, PullSingleImageInput{
		ImageID:      "redis:latest",
		RegistryHost: "registry.example.com",
		RegistryUser: "user",
		RegistryPass: "pass",
		Force:        true,
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
