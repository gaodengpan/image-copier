package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdapterError(t *testing.T) {
	innerErr := errors.New("inner error")
	err := NewAdapterError("docker", "pull", "failed to pull image", innerErr)

	assert.Equal(t, "docker", err.Adapter)
	assert.Equal(t, "pull", err.Operation)
	assert.Equal(t, "failed to pull image", err.Message)
	assert.Equal(t, innerErr, err.Cause)

	expected := "docker adapter error during pull: failed to pull image (cause: inner error)"
	assert.Equal(t, expected, err.Error())
	assert.True(t, errors.Is(err, innerErr))
}

func TestAdapterErrorWithoutCause(t *testing.T) {
	err := NewAdapterError("registry", "list", "no images found", nil)

	expected := "registry adapter error during list: no images found"
	assert.Equal(t, expected, err.Error())
}

func TestDockerError(t *testing.T) {
	err := NewDockerError("build", "build failed", assert.AnError)

	assert.Equal(t, "docker", err.Adapter)
	assert.Contains(t, err.Error(), "docker")
	assert.Contains(t, err.Error(), "build")
}

func TestRegistryError(t *testing.T) {
	err := NewRegistryError("copy", "copy failed", assert.AnError)

	assert.Equal(t, "registry", err.Adapter)
	assert.Contains(t, err.Error(), "registry")
}

func TestGitHubError(t *testing.T) {
	err := NewGitHubError("workflow", "workflow failed", 404, assert.AnError)

	assert.Equal(t, "github", err.Adapter)
	assert.Equal(t, 404, err.StatusCode)
	assert.Contains(t, err.Error(), "github")
	assert.Contains(t, err.Error(), "404")
}

func TestGitHubErrorWithoutCause(t *testing.T) {
	err := NewGitHubError("workflow", "not found", 404, nil)

	expected := "github adapter error during workflow: not found (status: 404)"
	assert.Equal(t, expected, err.Error())
}

func TestFileSystemError(t *testing.T) {
	err := NewFileSystemError("read", "file not found", assert.AnError)

	assert.Equal(t, "filesystem", err.Adapter)
	assert.Contains(t, err.Error(), "filesystem")
}
