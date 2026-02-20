package use_cases

import (
	"context"
	"testing"

	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestSyncUseCase(
	docker *mockDockerClient,
	registry *mockRegistryClient,
	github *mockGitHubClient,
	fs *mockFileSystem,
	logger *mockLogger,
) *SyncImagesUseCaseImpl {
	systemClient := new(mockSystemClient)
	systemClient.On("CommandExists", mock.Anything, "skopeo").Return(true, nil)
	systemClient.On("CommandExists", mock.Anything, "docker").Return(true, nil)
	systemClient.On("DockerRunning", mock.Anything).Return(true, nil)

	return NewSyncImagesUseCase(
		docker, registry, github, fs, nil, logger,
		systemClient, services.NewImageIDService(),
		SyncImagesConfig{
			RegistryHost:   "registry.com",
			RegistryUser:   "user",
			RegistryPass:   "pass",
			RegistryNS:     "ns",
			RegistryArch:   "amd64",
			RegistryOs:     "linux",
			GithubOwner:    "owner",
			GithubRepo:     "repo",
			GithubToken:    "token",
			GithubWorkflow: "workflow",
			Force:          false,
			DryRun:         false,
		},
	)
}

type mockSystemClient struct {
	mock.Mock
}

func (m *mockSystemClient) CommandExists(ctx context.Context, cmd string) (bool, error) {
	args := m.Called(ctx, cmd)
	return args.Bool(0), args.Error(1)
}

func (m *mockSystemClient) DockerRunning(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func TestSyncImagesUseCase_Diff_AllImagesNeedSync(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(false, nil)
	registry.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	registry.On("BuildDestImageID", "nginx", "registry.com", "ns").Return("registry.com/ns/nginx:latest")

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
	}

	synced, needsSync, err := uc.Diff(context.Background(), tasks, 1, false)

	assert.NoError(t, err)
	assert.Len(t, synced, 0)
	assert.Len(t, needsSync, 1)
	assert.Equal(t, "nginx", needsSync[0].Source)
}

func TestSyncImagesUseCase_Diff_AllImagesSynced(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(true, nil)
	registry.On("BuildDestImageID", "nginx", "registry.com", "ns").Return("registry.com/ns/nginx:latest")
	registry.On("CheckImageExists", mock.Anything, "registry.com/ns/nginx:latest", "user", "pass").Return(false, nil)

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
	}

	synced, needsSync, err := uc.Diff(context.Background(), tasks, 1, false)

	assert.NoError(t, err)
	assert.Len(t, synced, 1)
	assert.Len(t, needsSync, 0)
	assert.Equal(t, "nginx", synced[0].Source)
}

func TestSyncImagesUseCase_Diff_ForceOverridesLocalCheck(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(true, nil)
	registry.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	registry.On("BuildDestImageID", "nginx", "registry.com", "ns").Return("registry.com/ns/nginx:latest")

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
	}

	synced, needsSync, err := uc.Diff(context.Background(), tasks, 1, true)

	assert.NoError(t, err)
	assert.Len(t, synced, 0)
	assert.Len(t, needsSync, 1)
	assert.Equal(t, "nginx", needsSync[0].Source)
}

func TestSyncImagesUseCase_Diff_MultipleImages(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(true, nil)
	docker.On("ImageExists", mock.Anything, "redis").Return(false, nil)
	docker.On("ImageExists", mock.Anything, "postgres").Return(false, nil)

	registry.On("BuildDestImageID", mock.Anything, "registry.com", "ns").Return("registry.com/ns/image:latest")
	registry.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
		{Source: "redis", Arch: "amd64", Os: "linux"},
		{Source: "postgres", Arch: "amd64", Os: "linux"},
	}

	synced, needsSync, err := uc.Diff(context.Background(), tasks, 2, false)

	assert.NoError(t, err)
	assert.Len(t, synced, 1)
	assert.Len(t, needsSync, 2)
	assert.Equal(t, "nginx", synced[0].Source)
}

func TestSyncImagesUseCase_Diff_WithErrors(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, "nginx").Return(false, assert.AnError)
	registry.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	registry.On("BuildDestImageID", "nginx", "registry.com", "ns").Return("registry.com/ns/nginx:latest")

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
	}

	synced, needsSync, err := uc.Diff(context.Background(), tasks, 1, false)

	assert.NoError(t, err)
	assert.Len(t, synced, 0)
	assert.Len(t, needsSync, 1)
}

func TestSyncImagesUseCase_Execute_DryRun(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	docker.On("ImageExists", mock.Anything, mock.Anything).Return(false, nil)
	registry.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).Return("registry.com/ns/image:latest")
	registry.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	uc := NewSyncImagesUseCase(
		docker, registry, github, fs, nil, logger,
		nil, services.NewImageIDService(),
		SyncImagesConfig{
			RegistryHost: "registry.com",
			RegistryUser: "user",
			RegistryPass: "pass",
			RegistryNS:   "ns",
			Force:        false,
			DryRun:       true,
		},
	)

	tasks := []SyncTask{
		{Source: "nginx", Arch: "amd64", Os: "linux"},
	}

	input := SyncImagesInput{
		Tasks:       tasks,
		WorkerCount: 1,
		Force:       false,
		DryRun:      true,
	}

	synced, needsSync, err := uc.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.Len(t, synced, 0)
	assert.Len(t, needsSync, 1)
}

func TestSyncImagesUseCase_Execute_NoTasks(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	uc := newTestSyncUseCase(docker, registry, github, fs, logger)

	input := SyncImagesInput{
		Tasks:       []SyncTask{},
		WorkerCount: 1,
	}

	synced, needsSync, err := uc.Execute(context.Background(), input)

	assert.NoError(t, err)
	assert.Len(t, synced, 0)
	assert.Len(t, needsSync, 0)
}

type mockSyncCallback struct {
	mock.Mock
}

func (m *mockSyncCallback) OnStart(workerIdx, taskIdx int, task SyncTask) {
	m.Called(workerIdx, taskIdx, task)
}

func (m *mockSyncCallback) OnComplete(workerIdx, taskIdx int, task SyncTask, err error) {
	m.Called(workerIdx, taskIdx, task, err)
}

func (m *mockSyncCallback) OnProgress(workerIdx int, progress ProgressInfo) {
	m.Called(workerIdx, progress)
}

func TestSyncImagesUseCase_WithCallback(t *testing.T) {
	docker := new(mockDockerClient)
	registry := new(mockRegistryClient)
	github := new(mockGitHubClient)
	fs := new(mockFileSystem)
	logger := new(mockLogger)

	callback := new(mockSyncCallback)

	uc := NewSyncImagesUseCase(
		docker, registry, github, fs, nil, logger,
		nil, services.NewImageIDService(),
		SyncImagesConfig{
			RegistryHost: "registry.com",
			RegistryUser: "user",
			RegistryPass: "pass",
			RegistryNS:   "ns",
			Force:        false,
			DryRun:       true,
		},
	)

	_ = callback
	_ = uc
}
