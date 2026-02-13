package core

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestConstantsDefinedCorrectly(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, 30*time.Second, DefaultCacheTTL)
	assert.Equal(t, 10000, MaxCacheSizeDefault)
	assert.Equal(t, "docker", DockerCommand)
	assert.Equal(t, "skopeo", SkopeoCommand)
	assert.Equal(t, "{{.Repository}}:{{.Tag}}", DockerImageFormat)
	assert.Equal(t, 40, MaxNormalizedLen)
	assert.Equal(t, ":", CredentialsSeparator)

	// Test API constants
	assert.Equal(t, "2022-11-28", GitHubAPIVersion)
	assert.Equal(t, "application/vnd.github+json", GitHubMediaType)

	// Test timeout constants
	assert.Equal(t, 30*time.Second, WorkflowPollTimeout)
	assert.Equal(t, 120*time.Second, SkopeoCopyTimeout)
	assert.Equal(t, 60*time.Second, DockerLoadTimeout)
	assert.Equal(t, 15*time.Second, ListImagesTimeout)
	assert.Equal(t, 10*time.Second, CheckLocalTimeout)

	// Test regex patterns
	assert.NotEmpty(t, ImageValidationPattern)
	assert.NotEmpty(t, ValidShellChars)
	assert.NotEmpty(t, PathTraversalChars)

	// Test error messages
	assert.NotEmpty(t, ErrInvalidImageName)
	assert.NotEmpty(t, ErrInvalidCredentials)
	assert.NotEmpty(t, ErrCommandFailed)

	// Test sanitization constants
	assert.NotEmpty(t, SensitiveDataPrefix)
	assert.Equal(t, "]", SensitiveDataSuffix)
}

func TestPullerUsesConstants(t *testing.T) {
	config := &Config{
		GithubOwner:       "test-owner",
		GithubRepo:        "test-repo",
		GithubToken:       "test-token",
		GithubWorkflowID:  "test-workflow.yml",
		RegistryHost:      "registry.test.com",
		RegistryUsername:  "test-user",
		RegistryPassword:  "test-pass",
		RegistryNamespace: "test-ns",
		RegistryArch:      "amd64",
		RegistryOs:        "linux",
	}

	logger := &logrus.Logger{}
	puller := NewPuller(config, logger)

	// Verify that puller uses the constants
	assert.Equal(t, MaxCacheSizeDefault, puller.MaxCacheSize)
	assert.NotNil(t, puller.ImageValidator)
}