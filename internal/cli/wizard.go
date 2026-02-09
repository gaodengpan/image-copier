package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/example/image-copier/internal/config"
)

// ConfigData holds the configuration from the wizard
type ConfigData struct {
	GitHubOwner      string
	GitHubRepo       string
	GitHubToken      string
	GitHubWorkflowID string

	RegistryHost      string
	RegistryUsername  string
	RegistryPassword  string
	RegistryNamespace string
	RegistryArch      string
	RegistryOs        string
}

// RunWizard runs the interactive configuration wizard
func RunWizard(ctx context.Context, skipExisting bool) (*ConfigData, error) {
	reader := bufio.NewReader(os.Stdin)

	// Try to load existing config to provide defaults
	var existing *config.Config
	if skipExisting {
		if cfg, err := config.Load(); err == nil {
			existing = cfg
		}
	}

	data := &ConfigData{
		GitHubWorkflowID: "image-copier-v2.yaml",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	// If existing config, use those as defaults
	if existing != nil {
		data.GitHubOwner = existing.Github.Owner
		data.GitHubRepo = existing.Github.Repo
		data.GitHubToken = maskToken(existing.Github.Token)
		data.GitHubWorkflowID = existing.Github.WorkflowID

		data.RegistryHost = existing.Registry.Host
		data.RegistryUsername = existing.Registry.Username
		data.RegistryPassword = maskToken(existing.Registry.Password)
		data.RegistryNamespace = existing.Registry.Namespace
		data.RegistryArch = existing.Registry.Arch
		data.RegistryOs = existing.Registry.Os

		fmt.Println("\nDetected existing configuration. Press Enter to keep current value.")
	}

	fmt.Println("\n=== Image Copier Configuration Wizard ===\n")

	// GitHub Configuration
	fmt.Println("--- GitHub Configuration ---")

	data.GitHubOwner = promptString(reader, "GitHub repository owner (user or organization)",
		data.GitHubOwner, "")
	data.GitHubRepo = promptString(reader, "GitHub repository name",
		data.GitHubRepo, "")

	data.GitHubToken = promptString(reader, "GitHub personal access token with workflow permissions",
		data.GitHubToken, "(empty to skip validation)")

	// Validate GitHub token if provided
	if data.GitHubToken != "" && !strings.Contains(data.GitHubToken, "*") {
		fmt.Print("Validating GitHub token... ")
		if err := validateGitHubToken(ctx, data.GitHubOwner, data.GitHubRepo, data.GitHubToken); err != nil {
			fmt.Printf("Warning: %v\n", err)
			continueWarn := promptYesNo(reader, "Continue anyway?", false)
			if !continueWarn {
				return nil, fmt.Errorf("configuration cancelled")
			}
		} else {
			fmt.Println("OK")
		}
	}

	data.GitHubWorkflowID = promptString(reader, "Workflow filename",
		data.GitHubWorkflowID, "")

	// Registry Configuration
	fmt.Println("\n--- Registry Configuration ---")

	data.RegistryHost = promptString(reader, "Domestic registry host (e.g., registry.cn-hangzhou.aliyuncs.com)",
		data.RegistryHost, "")
	data.RegistryUsername = promptString(reader, "Registry username",
		data.RegistryUsername, "")
	data.RegistryPassword = promptString(reader, "Registry password or access token",
		data.RegistryPassword, "(leave masked to keep existing)")

	data.RegistryNamespace = promptString(reader,
		"Optional namespace for organizing images (leave empty to skip)",
		data.RegistryNamespace, "")

	data.RegistryArch = promptString(reader, "Default architecture",
		data.RegistryArch, "")
	data.RegistryOs = promptString(reader, "Default operating system",
		data.RegistryOs, "")

	return data, nil
}

// WriteConfigFile writes the configuration to a file
func WriteConfigFile(data *ConfigData, path string) error {
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

# Retry Configuration
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
		data.GitHubOwner,
		data.GitHubRepo,
		data.GitHubToken,
		data.GitHubWorkflowID,
		data.RegistryHost,
		data.RegistryUsername,
		data.RegistryPassword,
		data.RegistryNamespace,
		data.RegistryArch,
		data.RegistryOs,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

// promptString prompts for a string value
func promptString(reader *bufio.Reader, prompt, defaultValue, placeholder string) string {
	displayPrompt := prompt
	if defaultValue != "" {
		if placeholder == "" {
			displayPrompt += fmt.Sprintf(" [%s]", defaultValue)
		}
	} else if placeholder != "" {
		displayPrompt += fmt.Sprintf(" %s", placeholder)
	}
	displayPrompt += ": "

	fmt.Print(displayPrompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" || input == placeholder {
		return defaultValue
	}
	return input
}

// promptYesNo prompts for a yes/no answer
func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	displayPrompt := prompt
	if defaultYes {
		displayPrompt += " [Y/n]: "
	} else {
		displayPrompt += " [y/N]: "
	}

	for {
		fmt.Print(displayPrompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return defaultYes
		}
		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}
		fmt.Println("Please enter 'y' or 'n'")
	}
}

// maskToken masks a token with asterisks, leaving only first/last characters visible
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// validateGitHubToken validates a GitHub token by making a simple API call
func validateGitHubToken(ctx context.Context, owner, repo, token string) error {
	if owner == "" || repo == "" {
		return fmt.Errorf("owner and repo are required for token validation")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found")
	}

	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}