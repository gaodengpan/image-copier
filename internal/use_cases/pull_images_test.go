package use_cases

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPullImagesUseCase_NewPullImagesUseCase(t *testing.T) {
	uc := NewPullImagesUseCase(
		"registry.com", "user", "pass", "ns", "amd64", "linux",
		true, true,
		"token", "owner", "repo", "workflow",
	)

	assert.NotNil(t, uc)
	assert.Equal(t, "registry.com", uc.registryHost)
	assert.Equal(t, "user", uc.registryUser)
	assert.Equal(t, "pass", uc.registryPass)
	assert.Equal(t, "ns", uc.registryNS)
	assert.Equal(t, "amd64", uc.registryArch)
	assert.Equal(t, "linux", uc.registryOs)
	assert.Equal(t, true, uc.force)
	assert.Equal(t, true, uc.dryRun)
	assert.Equal(t, "token", uc.githubToken)
	assert.Equal(t, "owner", uc.githubOwner)
	assert.Equal(t, "repo", uc.githubRepo)
	assert.Equal(t, "workflow", uc.githubWorkflowID)
}
