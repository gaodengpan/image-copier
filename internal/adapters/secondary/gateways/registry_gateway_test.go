package gateways

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkopeoAdapter_NewSkopeoAdapter(t *testing.T) {
	adapter := NewSkopeoAdapter()
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.validator)
	assert.NotNil(t, adapter.commandRunner)
}

func TestSkopeoAdapter_ImageExists_InvalidImageName(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.ImageExists(context.Background(), "invalid;command", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_ImageExists_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.ImageExists(context.Background(), "nginx:latest", "", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")

	_, err = adapter.ImageExists(context.Background(), "nginx:latest", "user", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidImageName(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), "invalid;command", "tag", "/tmp/test.tar", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidFilePath(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), "nginx:latest", "tag", "/etc/passwd", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), "nginx:latest", "tag", "/tmp/test.tar", "", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestSkopeoAdapter_BuildDestImageID(t *testing.T) {
	adapter := NewSkopeoAdapter()

	tests := []struct {
		name              string
		sourceID          string
		registryHost      string
		registryNamespace string
		expectedResult    string
	}{
		{"with host and namespace", "nginx:latest", "registry.com", "ns", "registry.com/ns/nginx:latest"},
		{"with host only", "nginx:latest", "registry.com", "", "registry.com/nginx:latest"},
		{"with namespace only", "nginx:latest", "", "ns", "nginx:latest"},
		{"neither", "nginx:latest", "", "", "nginx:latest"},
		{"complex image", "myrepo/myimage:v1.0", "registry.com", "ns", "registry.com/ns/myrepo_myimage:v1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.BuildDestImageID(tt.sourceID, tt.registryHost, tt.registryNamespace)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSkopeoAdapter_BuildDestImageID_WithDigest(t *testing.T) {
	adapter := NewSkopeoAdapter()

	result := adapter.BuildDestImageID("nginx@sha256:abc123", "registry.com", "ns")
	assert.Contains(t, result, "@sha256:abc123")
}

func TestSkopeoAdapter_BuildDestImageID_MaxLength(t *testing.T) {
	adapter := NewSkopeoAdapter()

	longName := "myverylongrepositorynamewithalotofcharacters_that exceedsthe limit of fifty characters"
	result := adapter.BuildDestImageID(longName+":v1.0", "registry.com", "ns")

	assert.LessOrEqual(t, len(result), 50+len("registry.com/ns/")+len(":v1.0"))
	assert.True(t, strings.HasPrefix(result, "registry.com/ns/"))
	assert.True(t, strings.HasSuffix(result, ":v1.0"))
}

func TestSkopeoAdapter_CheckImageExists_InvalidImageName(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.CheckImageExists(context.Background(), "invalid;command", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_CheckImageExists_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.CheckImageExists(context.Background(), "nginx:latest", "", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}
