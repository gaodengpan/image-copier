package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/gaodengpan/image-copier/internal/infrastructure/encryption"
)

// WriteConfigFile writes the configuration to a file
func WriteConfigFile(data *ConfigData, path string) error {
	// Encrypt sensitive fields before writing to file
	encryptedData, err := encryptConfigData(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt config data: %w", err)
	}

	privateRegsContent := ""
	for _, reg := range encryptedData.PrivateRegistries {
		privateRegsContent += fmt.Sprintf(`  - name: %q
    host: %q
    username: %q
    password: %q
`, reg.Name, reg.Host, reg.Username, reg.Password)
	}

	// Write config file directly as YAML
	content := fmt.Sprintf(`# GitHub Configuration
github:
  # GitHub repository owner (user or organization)
  owner: %q
  # GitHub repository name
  repo: %q
  # GitHub personal access token with workflow permissions
  token: %q
  # Workflow filename (usually ends with .yaml)
  workflow_id: %q

# Registry Configuration
registry:
  # Domestic registry host (e.g., registry.cn-hangzhou.aliyuncs.com)
  host: %q
  # Registry username
  username: %q
  # Registry password or access token
  password: %q
  # Optional namespace for organizing images
  namespace: %q
  # Architecture for multi-platform images (default: amd64)
  arch: %q
  # Operating system for multi-platform images (default: linux)
  os: %q

# Private Registries Configuration
private_registries:
%s# Retry Configuration
retry:
  # Maximum number of retry attempts
  max_attempts: "3"
  # Initial retry interval (e.g., 1s, 500ms)
  initial_interval: "1s"
  # Maximum retry interval (e.g., 30s, 1m)
  max_interval: "30s"

# Logging Configuration
log_level: "info"
`,
		encryptedData.GitHubOwner,
		encryptedData.GitHubRepo,
		encryptedData.GitHubToken,
		encryptedData.GitHubWorkflowID,
		encryptedData.RegistryHost,
		encryptedData.RegistryUsername,
		encryptedData.RegistryPassword,
		encryptedData.RegistryNamespace,
		encryptedData.RegistryArch,
		encryptedData.RegistryOs,
		privateRegsContent,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

// encryptConfigData encrypts sensitive fields in ConfigData
func encryptConfigData(data *ConfigData) (*ConfigData, error) {
	// Attempt to create config encryptor
	encryptor, err := encryption.NewConfigEncryptor()
	if err != nil {
		// If encryption key is not available, return the original data unchanged
		// This allows the configuration to still be written even without encryption
		return data, nil
	}

	// Create a copy of the config data to encrypt
	encryptedData := &ConfigData{
		GitHubOwner:      data.GitHubOwner,
		GitHubRepo:       data.GitHubRepo,
		GitHubToken:      data.GitHubToken,
		GitHubWorkflowID: data.GitHubWorkflowID,
		RegistryHost:     data.RegistryHost,
		RegistryUsername: data.RegistryUsername,
		RegistryPassword: data.RegistryPassword,
		RegistryNamespace: data.RegistryNamespace,
		RegistryArch:     data.RegistryArch,
		RegistryOs:       data.RegistryOs,
		PrivateRegistries: data.PrivateRegistries,
	}

	// Encrypt sensitive fields if they're not already encrypted
	if data.GitHubToken != "" && !strings.HasPrefix(data.GitHubToken, "encrypted:") {
		encryptedToken, err := encryptor.EncryptValue(data.GitHubToken)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt GitHub token: %w", err)
		}
		encryptedData.GitHubToken = encryptedToken
	}

	if data.RegistryUsername != "" && !strings.HasPrefix(data.RegistryUsername, "encrypted:") {
		encryptedUsername, err := encryptor.EncryptValue(data.RegistryUsername)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt registry username: %w", err)
		}
		encryptedData.RegistryUsername = encryptedUsername
	}

	if data.RegistryPassword != "" && !strings.HasPrefix(data.RegistryPassword, "encrypted:") {
		encryptedPassword, err := encryptor.EncryptValue(data.RegistryPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt registry password: %w", err)
		}
		encryptedData.RegistryPassword = encryptedPassword
	}

	// Encrypt private registry credentials
	for i, reg := range data.PrivateRegistries {
		if reg.Username != "" && !strings.HasPrefix(reg.Username, "encrypted:") {
			encryptedUsername, err := encryptor.EncryptValue(reg.Username)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt private registry username: %w", err)
			}
			encryptedData.PrivateRegistries[i].Username = encryptedUsername
		}

		if reg.Password != "" && !strings.HasPrefix(reg.Password, "encrypted:") {
			encryptedPassword, err := encryptor.EncryptValue(reg.Password)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt private registry password: %w", err)
			}
			encryptedData.PrivateRegistries[i].Password = encryptedPassword
		}
	}

	return encryptedData, nil
}