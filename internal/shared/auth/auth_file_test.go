package auth

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAuthFile(t *testing.T) {
	tests := []struct {
		name      string
		registry  string
		username  string
		password  string
		wantError bool
	}{
		{
			name:     "valid credentials",
			registry: "docker.io",
			username: "testuser",
			password: "testpass",
		},
		{
			name:     "empty registry",
			registry: "",
			username: "testuser",
			password: "testpass",
		},
		{
			name:     "ghcr.io registry",
			registry: "ghcr.io",
			username: "token",
			password: "ghp_xxx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, err := CreateAuthFile(tt.registry, tt.username, tt.password)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() {
				_ = os.Remove(filePath)
			}()

			// Verify file exists
			_, err = os.Stat(filePath)
			assert.NoError(t, err)

			// Verify file permissions are restrictive
			info, err := os.Stat(filePath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

			// Verify file contents
			data, err := os.ReadFile(filePath)
			require.NoError(t, err)

			var config DockerConfig
			err = json.Unmarshal(data, &config)
			require.NoError(t, err)

			// Verify auth entry exists for the registry
			authEntry, exists := config.Auths[tt.registry]
			assert.True(t, exists)

			// Verify the auth is base64 encoded username:password
			expectedAuth := base64.StdEncoding.EncodeToString([]byte(tt.username + ":" + tt.password))
			assert.Equal(t, expectedAuth, authEntry.Auth)
		})
	}
}

func TestCreateAuthFile_FileCleanup(t *testing.T) {
	filePath, err := CreateAuthFile("test.io", "user", "pass")
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Clean up
	err = os.Remove(filePath)
	assert.NoError(t, err)

	// Verify file is removed
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}
