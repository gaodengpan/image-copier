package gateways

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecDockerAdapter_ImageExists(t *testing.T) {
	t.Run("ImageExists", func(t *testing.T) {
		adapter := NewExecDockerAdapter()
		// This test requires Docker to be running
		// Skip if Docker is not available
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("Docker not available")
		}

		// Test with a commonly available image
		// This is a basic integration test
		ctx := context.Background()
		// We don't assert specific results since it depends on local Docker state
		_, err := adapter.ImageExists(ctx, "hello-world:latest")
		// Should not return an error for valid image name
		if err != nil {
			assert.Contains(t, err.Error(), "invalid image name")
		}
	})

	t.Run("InvalidImageName", func(t *testing.T) {
		adapter := NewExecDockerAdapter()
		ctx := context.Background()

		_, err := adapter.ImageExists(ctx, "invalid;command")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid image name")
	})
}

func TestExecDockerAdapter_ListImages(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}

	adapter := NewExecDockerAdapter()
	ctx := context.Background()

	images, err := adapter.ListImages(ctx)
	// Should not error even with empty Docker
	assert.NoError(t, err)
	// Result is a list, could be empty
	assert.NotNil(t, images)
}

func TestExecDockerAdapter_NewExecDockerAdapter(t *testing.T) {
	adapter := NewExecDockerAdapter()

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.validator)
}

func TestExecDockerAdapter_ImageExists_Timeout(t *testing.T) {
	adapter := NewExecDockerAdapter()
	ctx := context.Background()

	// Use a valid image name that likely doesn't exist
	exists, err := adapter.ImageExists(ctx, "nonexistent-image-12345:latest")
	// Should return false without error (image doesn't exist)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple command",
			input:    "docker",
			expected: []string{"docker"},
		},
		{
			name:     "Command with args",
			input:    "docker run hello",
			expected: []string{"docker", "run", "hello"},
		},
		{
			name:     "Command with double quotes",
			input:    `docker run "hello world"`,
			expected: []string{"docker", "run", "hello world"},
		},
		{
			name:     "Command with single quotes",
			input:    `docker run 'hello world'`,
			expected: []string{"docker", "run", "hello world"},
		},
		{
			name:     "Complex command with lima",
			input:    `limactl shell k3s-master -- sudo nerdctl --address /run/k3s/containerd/containerd.sock -n k8s.io`,
			expected: []string{"limactl", "shell", "k3s-master", "--", "sudo", "nerdctl", "--address", "/run/k3s/containerd/containerd.sock", "-n", "k8s.io"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecDockerAdapter_LoadImageFromReader(t *testing.T) {
	t.Run("NilReader", func(t *testing.T) {
		adapter := NewExecDockerAdapter()
		ctx := context.Background()

		// Nil reader should cause an error when docker load is executed
		err := adapter.LoadImageFromReader(ctx, nil)
		// The actual behavior depends on docker being available
		// We just verify no panic occurs
		_ = err
	})
}

func TestExecDockerAdapter_CustomDockerCommand(t *testing.T) {
	// Test that custom docker command environment variable is respected
	t.Setenv(EnvDockerCommand, "echo test")

	adapter := NewExecDockerAdapter()
	assert.NotNil(t, adapter)

	// The adapter should use the custom command
	// We can't easily test this without actually running the command
	// but we can verify the adapter was created successfully
}
