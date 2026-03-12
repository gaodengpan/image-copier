// Package auth provides authentication utilities for container registries.
package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// DockerConfig represents Docker config.json format for authentication.
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth represents a single auth entry.
type DockerAuth struct {
	Auth string `json:"auth"`
}

// CreateAuthFile creates a temporary authentication file for skopeo/container tools.
// The file is created with restrictive permissions (0600) for security.
// Caller is responsible for deleting the file when done.
//
// Parameters:
//   - registry: the registry host (e.g., "docker.io", "ghcr.io")
//   - username: registry username
//   - password: registry password
//
// Returns the path to the temporary auth file.
func CreateAuthFile(registry, username, password string) (string, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	config := DockerConfig{
		Auths: map[string]DockerAuth{
			registry: {Auth: auth},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "auth-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp auth file: %w", err)
	}

	// Set restrictive permissions before writing credentials
	// 0600 = only owner can read/write
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set auth file permissions: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write auth config: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close auth file: %w", err)
	}

	return tmpFile.Name(), nil
}
