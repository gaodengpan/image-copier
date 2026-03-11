package gateways

import (
	"context"
	"strings"
	"testing"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
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

	_, err := adapter.ImageExists(context.Background(), output.RegistryAuthOptions{
		ImageID:  "invalid;command",
		Username: "user",
		Password: "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_ImageExists_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.ImageExists(context.Background(), output.RegistryAuthOptions{
		ImageID:  "nginx:latest",
		Username: "",
		Password: "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")

	_, err = adapter.ImageExists(context.Background(), output.RegistryAuthOptions{
		ImageID:  "nginx:latest",
		Username: "user",
		Password: "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidImageName(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), output.RegistrySaveOptions{
		ImageID:    "invalid;command",
		ImageTag:   "tag",
		OutputPath: "/tmp/test.tar",
		Username:   "user",
		Password:   "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidFilePath(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), output.RegistrySaveOptions{
		ImageID:    "nginx:latest",
		ImageTag:   "tag",
		OutputPath: "/etc/passwd",
		Username:   "user",
		Password:   "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path")
}

func TestSkopeoAdapter_SaveImageToFile_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToFile(context.Background(), output.RegistrySaveOptions{
		ImageID:    "nginx:latest",
		ImageTag:   "tag",
		OutputPath: "/tmp/test.tar",
		Username:   "",
		Password:   "pass",
	})
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
			result := adapter.BuildDestImageID(output.BuildDestOptions{
				SourceID:          tt.sourceID,
				RegistryHost:      tt.registryHost,
				RegistryNamespace: tt.registryNamespace,
			})
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSkopeoAdapter_BuildDestImageID_WithDigest(t *testing.T) {
	adapter := NewSkopeoAdapter()

	result := adapter.BuildDestImageID(output.BuildDestOptions{
		SourceID:          "nginx@sha256:abc123",
		RegistryHost:      "registry.com",
		RegistryNamespace: "ns",
	})
	assert.Contains(t, result, "@sha256:abc123")
}

func TestSkopeoAdapter_BuildDestImageID_MaxLength(t *testing.T) {
	adapter := NewSkopeoAdapter()

	longName := "myverylongrepositorynamewithalotofcharacters_that exceedsthe limit of fifty characters"
	result := adapter.BuildDestImageID(output.BuildDestOptions{
		SourceID:          longName + ":v1.0",
		RegistryHost:      "registry.com",
		RegistryNamespace: "ns",
	})

	assert.LessOrEqual(t, len(result), 50+len("registry.com/ns/")+len(":v1.0"))
	assert.True(t, strings.HasPrefix(result, "registry.com/ns/"))
	assert.True(t, strings.HasSuffix(result, ":v1.0"))
}

func TestSkopeoAdapter_CheckImageExists_InvalidImageName(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.CheckImageExists(context.Background(), output.RegistryAuthOptions{
		ImageID:  "invalid;command",
		Username: "user",
		Password: "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image name")
}

func TestSkopeoAdapter_CheckImageExists_InvalidCredentials(t *testing.T) {
	adapter := NewSkopeoAdapter()

	_, err := adapter.CheckImageExists(context.Background(), output.RegistryAuthOptions{
		ImageID:  "nginx:latest",
		Username: "",
		Password: "pass",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestSkopeoAdapter_SaveImageToWriter_NilWriter(t *testing.T) {
	adapter := NewSkopeoAdapter()

	err := adapter.SaveImageToWriter(context.Background(), output.RegistrySaveOptions{
		ImageID:  "nginx:latest",
		ImageTag: "tag",
		Username: "user",
		Password: "pass",
		Writer:   nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "writer is required")
}

func TestRegistryAuthOptions_String_MasksPassword(t *testing.T) {
	opts := output.RegistryAuthOptions{
		ImageID:  "nginx:latest",
		Username: "testuser",
		Password: "secret-password",
	}

	result := opts.String()
	assert.Contains(t, result, "nginx:latest")
	assert.Contains(t, result, "testuser")
	assert.Contains(t, result, "Password:***")
	assert.NotContains(t, result, "secret-password")
}

func TestRegistrySaveOptions_String_MasksPassword(t *testing.T) {
	opts := output.RegistrySaveOptions{
		ImageID:    "nginx:latest",
		ImageTag:   "v1.0",
		Username:   "testuser",
		Password:   "secret-password",
		OutputPath: "/tmp/test.tar",
	}

	result := opts.String()
	assert.Contains(t, result, "nginx:latest")
	assert.Contains(t, result, "v1.0")
	assert.Contains(t, result, "testuser")
	assert.Contains(t, result, "Password:***")
	assert.Contains(t, result, "/tmp/test.tar")
	assert.NotContains(t, result, "secret-password")
}

func TestBuildDestOptions_String(t *testing.T) {
	opts := output.BuildDestOptions{
		SourceID:          "nginx:latest",
		RegistryHost:      "registry.com",
		RegistryNamespace: "ns",
	}

	result := opts.String()
	assert.Contains(t, result, "nginx:latest")
	assert.Contains(t, result, "registry.com")
	assert.Contains(t, result, "ns")
}
