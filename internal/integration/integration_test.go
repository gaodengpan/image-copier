package integration

import (
	"context"
	"testing"

	dockeradapter "github.com/gaodengpan/image-copier/internal/adapters/docker"
	"github.com/gaodengpan/image-copier/internal/adapters/filesystem"
	githubadapter "github.com/gaodengpan/image-copier/internal/adapters/github"
	registryadapter "github.com/gaodengpan/image-copier/internal/adapters/registry"
	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newPullerWithRealAdapters(cfg *core.Config, logger *logrus.Logger) *core.Puller {
	dockerClient := dockeradapter.NewExecDockerAdapter()
	registryClient := registryadapter.NewSkopeoAdapter()
	githubClient := githubadapter.NewAPIAdapter(nil, cfg.GithubToken, cfg.GithubOwner, cfg.GithubRepo)
	fs := filesystem.NewOSAdapter()
	return core.NewPullerWithPorts(cfg, logger, dockerClient, registryClient, githubClient, fs)
}

// IntegrationTestSuite groups integration tests
type IntegrationTestSuite struct {
	logger *logrus.Logger
}

// setUp initializes the test suite
func setUp() *IntegrationTestSuite {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	return &IntegrationTestSuite{
		logger: logger,
	}
}

// TestConfigPullerIntegration tests the integration between config and puller
func TestConfigPullerIntegration(t *testing.T) {
	suite := setUp()

	// Create a basic config
	cfg := config.NewConfigBuilder().
		WithGithubOwner("test-owner").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithGithubWorkflowID("test-workflow").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		WithRegistryNamespace("test-namespace").
		WithRegistryArch("amd64").
		WithRegistryOs("linux").
		WithLogLevel("info").
		Build()

	// Create puller with the config
	puller := newPullerWithRealAdapters(&core.Config{
		GithubOwner:       cfg.Github.Owner,
		GithubRepo:        cfg.Github.Repo,
		GithubToken:       cfg.Github.Token,
		GithubWorkflowID:  cfg.Github.WorkflowID,
		RegistryHost:      cfg.Registry.Host,
		RegistryUsername:  cfg.Registry.Username,
		RegistryPassword:  cfg.Registry.Password,
		RegistryNamespace: cfg.Registry.Namespace,
		RegistryArch:      cfg.Registry.Arch,
		RegistryOs:        cfg.Registry.Os,
		RetryConfig:       cfg.ParseRetryConfig(),
	}, suite.logger)

	// Verify puller was created with correct config
	assert.Equal(t, cfg.Github.Owner, puller.Config.GithubOwner)
	assert.Equal(t, cfg.Github.Repo, puller.Config.GithubRepo)
	assert.Equal(t, cfg.Github.Token, puller.Config.GithubToken)
	assert.Equal(t, cfg.Github.WorkflowID, puller.Config.GithubWorkflowID)
	assert.Equal(t, cfg.Registry.Host, puller.Config.RegistryHost)
	assert.Equal(t, cfg.Registry.Username, puller.Config.RegistryUsername)
	assert.Equal(t, cfg.Registry.Password, puller.Config.RegistryPassword)
	assert.Equal(t, cfg.Registry.Namespace, puller.Config.RegistryNamespace)
	assert.Equal(t, cfg.Registry.Arch, puller.Config.RegistryArch)
	assert.Equal(t, cfg.Registry.Os, puller.Config.RegistryOs)
	assert.Equal(t, cfg.ParseRetryConfig(), puller.RetryConfig)
}

// TestConfigValidationIntegration tests config validation integration
func TestConfigValidationIntegration(t *testing.T) {
	suite := setUp()

	// Test valid config
	validConfig := config.NewConfigBuilder().
		WithGithubOwner("test-owner").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		Build()

	err := config.ValidateConfig(validConfig)
	assert.NoError(t, err)

	// Test invalid config (missing required fields)
	invalidConfig := config.NewConfigBuilder().
		WithGithubOwner("").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		Build()

	err = config.ValidateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "github owner is required")

	// Create puller with valid config to test retry integration
	puller := newPullerWithRealAdapters(&core.Config{
		GithubOwner:       validConfig.Github.Owner,
		GithubRepo:        validConfig.Github.Repo,
		GithubToken:       validConfig.Github.Token,
		GithubWorkflowID:  validConfig.Github.WorkflowID,
		RegistryHost:      validConfig.Registry.Host,
		RegistryUsername:  validConfig.Registry.Username,
		RegistryPassword:  validConfig.Registry.Password,
		RegistryNamespace: validConfig.Registry.Namespace,
		RegistryArch:      validConfig.Registry.Arch,
		RegistryOs:        validConfig.Registry.Os,
		RetryConfig:       validConfig.ParseRetryConfig(),
	}, suite.logger)

	assert.Equal(t, validConfig.ParseRetryConfig(), puller.RetryConfig)
}

// TestRetryConfigIntegration tests retry configuration integration
func TestRetryConfigIntegration(t *testing.T) {
	suite := setUp()

	// Test with custom retry config
	customConfig := config.NewConfigBuilder().
		WithGithubOwner("test-owner").
		WithGithubRepo("test-repo").
		WithGithubToken("test-token").
		WithRegistryHost("registry.example.com").
		WithRegistryUsername("test-user").
		WithRegistryPassword("test-pass").
		WithRetryConfig("5", "2s", "60s").
		Build()

	parsedRetryConfig := customConfig.ParseRetryConfig()
	expectedRetryConfig := &retry.Config{
		MaxAttempts:     5,
		InitialInterval: 2 * 1000 * 1000 * 1000,  // 2 seconds in nanoseconds
		MaxInterval:     60 * 1000 * 1000 * 1000, // 60 seconds in nanoseconds
	}

	assert.Equal(t, expectedRetryConfig.MaxAttempts, parsedRetryConfig.MaxAttempts)
	assert.Equal(t, expectedRetryConfig.MaxInterval, parsedRetryConfig.MaxInterval)

	// Create puller and verify retry config is applied
	puller := newPullerWithRealAdapters(&core.Config{
		GithubOwner:       customConfig.Github.Owner,
		GithubRepo:        customConfig.Github.Repo,
		GithubToken:       customConfig.Github.Token,
		GithubWorkflowID:  customConfig.Github.WorkflowID,
		RegistryHost:      customConfig.Registry.Host,
		RegistryUsername:  customConfig.Registry.Username,
		RegistryPassword:  customConfig.Registry.Password,
		RegistryNamespace: customConfig.Registry.Namespace,
		RegistryArch:      customConfig.Registry.Arch,
		RegistryOs:        customConfig.Registry.Os,
		RetryConfig:       parsedRetryConfig,
	}, suite.logger)

	assert.Equal(t, expectedRetryConfig.MaxAttempts, puller.RetryConfig.MaxAttempts)
	assert.Equal(t, expectedRetryConfig.MaxInterval, puller.RetryConfig.MaxInterval)
}

// TestImageNameProcessingIntegration tests the integration between image name processing functions
func TestImageNameProcessingIntegration(t *testing.T) {
	// Test normalize -> build dest integration
	sourceImage := "nginx:latest"
	normalized := core.NormalizeSourceID(sourceImage)

	// Build destination using normalized source
	host := "myregistry.com"
	namespace := "mynamespace"
	destImage := core.BuildDestImageID(host, namespace, normalized)

	// The normalized image should have been further processed in BuildDestImageID
	assert.Contains(t, destImage, host)
	assert.Contains(t, destImage, namespace)

	// Test that both functions work together properly
	assert.NotEqual(t, sourceImage, normalized)
	assert.Contains(t, normalized, "docker.io")
	assert.Contains(t, normalized, "library")
	assert.Contains(t, normalized, "nginx:latest")

	// Test validation integration
	validator := core.NewImageValidator()
	isValid := validator.IsValidImageName(sourceImage)
	assert.True(t, isValid)

	isValidAfterNormalization := validator.IsValidImageName(normalized)
	assert.True(t, isValidAfterNormalization)
}

// TestPrePullValidationIntegration tests the integration of pre-pull validation
func TestPrePullValidationIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a puller
	puller := newPullerWithRealAdapters(&core.Config{
		GithubOwner:       "test-owner",
		GithubRepo:        "test-repo",
		GithubToken:       "test-token",
		GithubWorkflowID:  "test-workflow",
		RegistryHost:      "registry.example.com",
		RegistryUsername:  "test-user",
		RegistryPassword:  "test-pass",
		RegistryNamespace: "test-namespace",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}, logger)

	// Run validation
	err := puller.PrePullValidate()

	// The validation may succeed if tools are installed, or fail if not
	// The important part is that the function executes without panicking
	// We just need to ensure the validation ran once (due to sync.Once mechanism)
	err2 := puller.PrePullValidate()

	// Both calls should return the same result since validation is only run once
	assert.Equal(t, err, err2)
}

// TestImageExistenceFunctionsIntegration tests the integration of image existence checking functions
func TestImageExistenceFunctionsIntegration(t *testing.T) {
	skopeoAdapter := registryadapter.NewSkopeoAdapter()
	ctx := context.Background()

	exists, err := skopeoAdapter.CheckImageExists(ctx, "example.com/fake/image:tag", "fakeuser", "fakepass")

	assert.False(t, exists)
	assert.NoError(t, err)

	_, err = skopeoAdapter.CheckImageExists(ctx, "invalid;command", "fakeuser", "fakepass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")

	_, err = skopeoAdapter.CheckImageExists(ctx, "example.com/fake/image:tag", "fake;user", "fakepass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}
