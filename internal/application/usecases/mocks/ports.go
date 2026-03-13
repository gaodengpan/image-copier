package mocks

import (
	"context"
	"net/http"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/stretchr/testify/mock"
)

// MockRegistryClient implements output.RegistryClient for testing
type MockRegistryClient struct {
	mock.Mock
}

func (m *MockRegistryClient) ImageExists(ctx context.Context, opts output.RegistryAuthOptions) (bool, error) {
	args := m.Called(ctx, opts)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegistryClient) SaveImageToFile(ctx context.Context, opts output.RegistrySaveOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *MockRegistryClient) SaveImageToWriter(ctx context.Context, opts output.RegistrySaveOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *MockRegistryClient) BuildDestImageID(opts output.BuildDestOptions) string {
	args := m.Called(opts)
	return args.String(0)
}

var _ output.RegistryClient = (*MockRegistryClient)(nil)

// MockGitHubClient implements output.GitHubClientWithRetry for testing
type MockGitHubClient struct {
	mock.Mock
}

func (m *MockGitHubClient) TriggerWorkflow(ctx context.Context, owner, repo, workflowID string, inputs map[string]string) (string, error) {
	args := m.Called(ctx, owner, repo, workflowID, inputs)
	return args.String(0), args.Error(1)
}

func (m *MockGitHubClient) GetWorkflowStatus(ctx context.Context, owner, repo, runID string) (string, error) {
	args := m.Called(ctx, owner, repo, runID)
	return args.String(0), args.Error(1)
}

func (m *MockGitHubClient) WaitForWorkflow(ctx context.Context, owner, repo, runID string) error {
	args := m.Called(ctx, owner, repo, runID)
	return args.Error(0)
}

func (m *MockGitHubClient) FindWorkflowRunID(ctx context.Context, owner, repo, workflowID, sourceID, destImageID, suffix string) (string, error) {
	args := m.Called(ctx, owner, repo, workflowID, sourceID, destImageID, suffix)
	return args.String(0), args.Error(1)
}

func (m *MockGitHubClient) TriggerWorkflowWithRetry(ctx context.Context, imageID, destImageID, arch, osType string) (string, error) {
	args := m.Called(ctx, imageID, destImageID, arch, osType)
	return args.String(0), args.Error(1)
}

func (m *MockGitHubClient) WaitForWorkflowSimple(ctx context.Context, runID string) error {
	args := m.Called(ctx, runID)
	return args.Error(0)
}

var _ output.GitHubClientWithRetry = (*MockGitHubClient)(nil)

// MockLogger implements output.Logger for testing
// This is a simple mock that ignores all log calls for convenience in tests.
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

func (m *MockLogger) Info(args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

func (m *MockLogger) Warn(args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

func (m *MockLogger) Error(args ...interface{}) {
	// Ignore log calls in tests - no need to set expectations
}

var _ output.Logger = (*MockLogger)(nil)

type MockSystemClient struct {
	mock.Mock
}

func (m *MockSystemClient) CommandExists(ctx context.Context, cmd string) (bool, error) {
	args := m.Called(ctx, cmd)
	return args.Bool(0), args.Error(1)
}

func (m *MockSystemClient) DockerRunning(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

var _ output.SystemClient = (*MockSystemClient)(nil)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

var _ output.HTTPClient = (*MockHTTPClient)(nil)

// MockSyncConfig implements output.SyncConfig for testing
type MockSyncConfig struct {
	mock.Mock
}

func (m *MockSyncConfig) StagingRegistryHost() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) StagingRegistryNamespace() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) StagingRegistryUsername() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) StagingRegistryPassword() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) DefaultArch() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) DefaultOS() string {
	return m.Called().String(0)
}

func (m *MockSyncConfig) GetDistributionTargets(targets []string) []string {
	args := m.Called(targets)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]string)
}

func (m *MockSyncConfig) GetPrivateRegistry(name string) *output.PrivateRegistryConfig {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*output.PrivateRegistryConfig)
}

var _ output.SyncConfig = (*MockSyncConfig)(nil)

// MockDistributionTargetBuilder implements output.DistributionTargetBuilder for testing
type MockDistributionTargetBuilder struct {
	mock.Mock
}

func (m *MockDistributionTargetBuilder) BuildTargets(targetNames []string) []*value_objects.DistributionTarget {
	args := m.Called(targetNames)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*value_objects.DistributionTarget)
}

var _ output.DistributionTargetBuilder = (*MockDistributionTargetBuilder)(nil)

// MockMultiTargetDistributor implements output.MultiTargetDistributor for testing
type MockMultiTargetDistributor struct {
	mock.Mock
}

func (m *MockMultiTargetDistributor) DistributeToAll(
	ctx context.Context,
	task *entities.DistributeTask,
	targets []*value_objects.DistributionTarget,
	stagingConfig output.StagingRegistryConfig,
	force bool,
) output.DistributeResult {
	args := m.Called(ctx, task, targets, stagingConfig, force)
	return args.Get(0).(output.DistributeResult)
}

var _ output.MultiTargetDistributor = (*MockMultiTargetDistributor)(nil)

// MockDistributionStrategy implements output.DistributionStrategy for testing
type MockDistributionStrategy struct {
	mock.Mock
}

func (m *MockDistributionStrategy) Distribute(ctx context.Context, opts output.DistributionOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *MockDistributionStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	args := m.Called(ctx, opts)
	return args.Bool(0), args.Error(1)
}

func (m *MockDistributionStrategy) TargetType() value_objects.TargetType {
	return m.Called().Get(0).(value_objects.TargetType)
}

var _ output.DistributionStrategy = (*MockDistributionStrategy)(nil)
