package gateways

import (
	"context"
	"errors"
	"testing"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockDistributionStrategy is a mock for DistributionStrategy
type mockDistributionStrategy struct {
	mock.Mock
}

func (m *mockDistributionStrategy) Distribute(ctx context.Context, opts output.DistributionOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *mockDistributionStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	args := m.Called(ctx, opts)
	return args.Bool(0), args.Error(1)
}

func (m *mockDistributionStrategy) TargetType() value_objects.TargetType {
	return m.Called().Get(0).(value_objects.TargetType)
}

// mockLogger is a mock for Logger
type mockDistLogger struct {
	mock.Mock
}

func (m *mockDistLogger) Infof(format string, args ...interface{})  {}
func (m *mockDistLogger) Debugf(format string, args ...interface{}) {}
func (m *mockDistLogger) Errorf(format string, args ...interface{}) {}
func (m *mockDistLogger) Info(args ...interface{})                  {}
func (m *mockDistLogger) Warn(args ...interface{})                  {}
func (m *mockDistLogger) Error(args ...interface{})                 {}

func TestMultiTargetDistributor_DistributeToAll_DockerTarget(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	dockerStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	dockerStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil)

	distributor := NewMultiTargetDistributor(dockerStrategy, nil, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	targets := []*value_objects.DistributionTarget{dockerTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Success)
	assert.Equal(t, "docker", result.Results[0].TargetName)
	dockerStrategy.AssertExpectations(t)
}

func TestMultiTargetDistributor_DistributeToAll_RegistryTarget(t *testing.T) {
	registryStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	registryStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	registryStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil)

	distributor := NewMultiTargetDistributor(nil, registryStrategy, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"my-registry"})
	registryTarget, _ := value_objects.NewRegistryTarget("my-registry", "registry.example.com", "user", "pass")
	targets := []*value_objects.DistributionTarget{registryTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Success)
	assert.Equal(t, "my-registry", result.Results[0].TargetName)
	registryStrategy.AssertExpectations(t)
}

func TestMultiTargetDistributor_DistributeToAll_SkipExisting(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	dockerStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(true, nil)
	// Distribute should NOT be called when image exists

	distributor := NewMultiTargetDistributor(dockerStrategy, nil, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	targets := []*value_objects.DistributionTarget{dockerTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false, // force = false
	)

	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Skipped)
	assert.False(t, result.Results[0].Success)
	dockerStrategy.AssertNotCalled(t, "Distribute")
}

func TestMultiTargetDistributor_DistributeToAll_ForceDistribute(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	// ExistsInDistributionTarget should NOT be called when force = true
	dockerStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil)

	distributor := NewMultiTargetDistributor(dockerStrategy, nil, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	targets := []*value_objects.DistributionTarget{dockerTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		true, // force = true
	)

	assert.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Success)
	assert.False(t, result.Results[0].Skipped)
	dockerStrategy.AssertNotCalled(t, "ExistsInDistributionTarget")
}

func TestMultiTargetDistributor_DistributeToAll_DistributionError(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	dockerStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	dockerStrategy.On("Distribute", mock.Anything, mock.Anything).Return(errors.New("connection refused"))

	distributor := NewMultiTargetDistributor(dockerStrategy, nil, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	targets := []*value_objects.DistributionTarget{dockerTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Success)
	assert.NotNil(t, result.Results[0].Error)
	assert.Contains(t, result.Results[0].Error.Error(), "connection refused")
}

func TestMultiTargetDistributor_DistributeToAll_UnsupportedTargetType(t *testing.T) {
	logger := new(mockDistLogger)

	// No strategies provided
	distributor := NewMultiTargetDistributor(nil, nil, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker"})
	dockerTarget := value_objects.NewDockerTarget()
	targets := []*value_objects.DistributionTarget{dockerTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].Success)
	assert.NotNil(t, result.Results[0].Error)
	assert.Contains(t, result.Results[0].Error.Error(), "unsupported target type")
}

func TestMultiTargetDistributor_DistributeToAll_MultipleTargets(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	registryStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	dockerStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	dockerStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil)

	registryStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	registryStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil)

	distributor := NewMultiTargetDistributor(dockerStrategy, registryStrategy, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker", "my-registry"})
	dockerTarget := value_objects.NewDockerTarget()
	registryTarget, _ := value_objects.NewRegistryTarget("my-registry", "registry.example.com", "user", "pass")
	targets := []*value_objects.DistributionTarget{dockerTarget, registryTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 2)
	// Both should succeed
	successCount := 0
	for _, r := range result.Results {
		if r.Success {
			successCount++
		}
	}
	assert.Equal(t, 2, successCount)

	dockerStrategy.AssertExpectations(t)
	registryStrategy.AssertExpectations(t)
}

func TestMultiTargetDistributor_DistributeToAll_PartialFailure(t *testing.T) {
	dockerStrategy := new(mockDistributionStrategy)
	registryStrategy := new(mockDistributionStrategy)
	logger := new(mockDistLogger)

	dockerStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	dockerStrategy.On("Distribute", mock.Anything, mock.Anything).Return(nil) // Success

	registryStrategy.On("ExistsInDistributionTarget", mock.Anything, mock.Anything).Return(false, nil)
	registryStrategy.On("Distribute", mock.Anything, mock.Anything).Return(errors.New("registry error")) // Failure

	distributor := NewMultiTargetDistributor(dockerStrategy, registryStrategy, logger)

	task := entities.NewDistributeTask("test-image:latest", "nginx:latest", "amd64", "linux", []string{"docker", "my-registry"})
	dockerTarget := value_objects.NewDockerTarget()
	registryTarget, _ := value_objects.NewRegistryTarget("my-registry", "registry.example.com", "user", "pass")
	targets := []*value_objects.DistributionTarget{dockerTarget, registryTarget}

	result := distributor.DistributeToAll(
		context.Background(),
		task,
		targets,
		output.StagingRegistryConfig{Host: "staging.registry.com", Namespace: "ns", Username: "user", Password: "pass"},
		false,
	)

	assert.Len(t, result.Results, 2)

	// Check results - one success, one failure
	successCount := 0
	failedCount := 0
	for _, r := range result.Results {
		if r.Success {
			successCount++
		}
		if r.Error != nil {
			failedCount++
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, failedCount)
}
