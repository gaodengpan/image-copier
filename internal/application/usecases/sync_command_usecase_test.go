package use_cases

import (
	"context"
	"errors"
	"testing"

	"github.com/gaodengpan/image-copier/internal/application/usecases/mocks"
	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// syncTestMocks holds all mocks needed for SyncCommandUseCase tests
type syncTestMocks struct {
	registryClient *mockRegistryClient
	githubClient   *mockGitHubClient
	logger         *mockLogger
	syncConfig     *mocks.MockSyncConfig
	targetBuilder  *mocks.MockDistributionTargetBuilder
	distributor    *mocks.MockMultiTargetDistributor
}

func newSyncTestMocks() *syncTestMocks {
	return &syncTestMocks{
		registryClient: new(mockRegistryClient),
		githubClient:   new(mockGitHubClient),
		logger:         new(mockLogger),
		syncConfig:     new(mocks.MockSyncConfig),
		targetBuilder:  new(mocks.MockDistributionTargetBuilder),
		distributor:    new(mocks.MockMultiTargetDistributor),
	}
}

func newSyncTestUseCase(m *syncTestMocks) *SyncCommandUseCaseImpl {
	return NewSyncCommandUseCase(
		m.registryClient,
		m.githubClient,
		m.logger,
		services.NewImageIDService(),
		m.syncConfig,
		m.targetBuilder,
		m.distributor,
	)
}

// setupBasicSyncExpectations sets up common mock expectations for sync tests
func setupBasicSyncExpectations(m *syncTestMocks) {
	m.syncConfig.On("StagingRegistryHost").Return("staging.registry.com")
	m.syncConfig.On("StagingRegistryNamespace").Return("ns")
	m.syncConfig.On("StagingRegistryUsername").Return("user")
	m.syncConfig.On("StagingRegistryPassword").Return("pass")
	m.syncConfig.On("DefaultArch").Return("amd64")
	m.syncConfig.On("DefaultOS").Return("linux")
	m.registryClient.On("BuildDestImageID", mock.Anything, mock.Anything, mock.Anything).
		Return("staging.registry.com/ns/nginx:latest")
}

func TestSyncCommandUseCase_Execute_DryRun(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: image does not exist in staging registry
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		DryRun:       true,
		WorkerCount:  1,
		SkipDistribute: true,
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.SyncPhase)
	assert.Len(t, result.SyncPhase.NewlySynced, 1)
	assert.Len(t, result.SyncPhase.AlreadyExisted, 0)
}

func TestSyncCommandUseCase_Execute_ImageAlreadySynced(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: image already exists in staging registry
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		DryRun:       false,
		Force:        false,
		WorkerCount:  1,
		SkipDistribute: true,
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SyncPhase.AlreadyExisted, 1)
	assert.Len(t, result.SyncPhase.NewlySynced, 0)
}

func TestSyncCommandUseCase_Execute_ForceSync(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: image exists but Force=true
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)
	// syncSingleImageToStaging calls ImageExists to check again
	m.registryClient.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil)
	m.githubClient.On("TriggerWorkflowWithRetry", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("run-123", nil)
	m.githubClient.On("WaitForWorkflowSimple", mock.Anything, "run-123").Return(nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		DryRun:       false,
		Force:        true,
		WorkerCount:  1,
		SkipDistribute: true,
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SyncPhase.NewlySynced, 1)
}

func TestSyncCommandUseCase_Execute_SkipSync(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup distribution expectations
	m.syncConfig.On("GetDistributionTargets", []string{"docker"}).Return([]string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	m.targetBuilder.On("BuildTargets", []string{"docker"}).Return([]*value_objects.DistributionTarget{dockerTarget})
	m.distributor.On("DistributeToAll",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(output.DistributeResult{
		Task:    entities.NewDistributeTask("test-id", "nginx:latest", "amd64", "linux", []string{"docker"}),
		Results: []entities.TargetResult{{TargetName: "docker", Success: true}},
	})

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		SkipSync:     true,
		WorkerCount:  1,
		Targets:      []string{"docker"},
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SyncPhase.AlreadyExisted, 1)
}

func TestSyncCommandUseCase_Execute_SkipDistribute(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: image exists in staging
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:         []string{"nginx:latest"},
		SkipDistribute: true,
		WorkerCount:    1,
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.SyncPhase)
	assert.Nil(t, result.DistributePhase.Tasks)
}

func TestSyncCommandUseCase_Execute_DistributePhase(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup sync phase: image already synced
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	// Setup distribute phase
	m.syncConfig.On("GetDistributionTargets", []string{"docker"}).Return([]string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	m.targetBuilder.On("BuildTargets", []string{"docker"}).Return([]*value_objects.DistributionTarget{dockerTarget})

	task := entities.NewDistributeTask("staging.registry.com/ns/nginx:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	m.distributor.On("DistributeToAll",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(output.DistributeResult{
		Task:    task,
		Results: []entities.TargetResult{{TargetName: "docker", Success: true}},
	})

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		WorkerCount:  1,
		Targets:      []string{"docker"},
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.DistributePhase.SuccessCount)
}

func TestSyncCommandUseCase_Execute_DistributeWithErrors(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup sync phase: image already synced
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	// Setup distribute phase with error
	m.syncConfig.On("GetDistributionTargets", []string{"registry1"}).Return([]string{"registry1"})
	registryTarget, _ := value_objects.NewRegistryTarget("registry1", "registry1.example.com", "user", "pass")
	m.targetBuilder.On("BuildTargets", []string{"registry1"}).Return([]*value_objects.DistributionTarget{registryTarget})

	task := entities.NewDistributeTask("staging.registry.com/ns/nginx:latest", "nginx:latest", "amd64", "linux", []string{"registry1"})
	m.distributor.On("DistributeToAll",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(output.DistributeResult{
		Task:    task,
		Results: []entities.TargetResult{{TargetName: "registry1", Success: false, Error: errors.New("connection refused")}},
	})

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		WorkerCount:  1,
		Targets:      []string{"registry1"},
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.DistributePhase.SuccessCount)
	assert.Equal(t, 1, result.DistributePhase.FailedCount)
	assert.Len(t, result.DistributePhase.Errors, 1)
}

func TestSyncCommandUseCase_Execute_CheckError(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: error checking image existence
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, errors.New("network error"))

	// When check fails, the task is still added to NeedsSync, which triggers syncSingleImageToStaging
	// We need to mock ImageExists as well (it's called to double-check before syncing)
	m.registryClient.On("ImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil)
	m.githubClient.On("TriggerWorkflowWithRetry", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("run-123", nil)
	m.githubClient.On("WaitForWorkflowSimple", mock.Anything, "run-123").Return(nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:         []string{"nginx:latest"},
		WorkerCount:    1,
		SkipDistribute: true,
	}

	result, err := uc.Execute(context.Background(), testInput)

	// Should not fail - errors are recorded in result.Errors
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SyncPhase.Errors, 1)
}

func TestSyncCommandUseCase_Execute_NoDistributionTargets(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup sync phase: image already synced
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	// Setup: no targets
	m.syncConfig.On("GetDistributionTargets", []string{}).Return([]string{})

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		WorkerCount:  1,
		Targets:      []string{},
	}

	result, err := uc.Execute(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.DistributePhase.SuccessCount)
}

func TestSyncCommandUseCase_SyncPhase(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup: image does not exist in staging
	m.registryClient.On("CheckImageExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil)

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		Images:       []string{"nginx:latest"},
		DryRun:       true,
		WorkerCount:  1,
	}

	result, err := uc.SyncPhase(context.Background(), testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.NewlySynced, 1)
}

func TestSyncCommandUseCase_DistributePhase(t *testing.T) {
	m := newSyncTestMocks()
	setupBasicSyncExpectations(m)

	// Setup distribution expectations
	m.syncConfig.On("GetDistributionTargets", []string{"docker"}).Return([]string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	m.targetBuilder.On("BuildTargets", []string{"docker"}).Return([]*value_objects.DistributionTarget{dockerTarget})

	task := entities.NewDistributeTask("test-id", "nginx:latest", "amd64", "linux", []string{"docker"})
	m.distributor.On("DistributeToAll",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(output.DistributeResult{
		Task:    task,
		Results: []entities.TargetResult{{TargetName: "docker", Success: true}},
	})

	uc := newSyncTestUseCase(m)

	testInput := input.SyncCommandInput{
		WorkerCount: 1,
		Targets:     []string{"docker"},
	}

	result, err := uc.DistributePhase(context.Background(), []string{"nginx:latest"}, testInput)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.SuccessCount)
}