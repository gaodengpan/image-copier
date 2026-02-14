package encryption

import (
	"fmt"
	"os"
	"path/filepath"
)

// FirstRunHandler manages the first-run experience for configurations with encrypted values
type FirstRunHandler struct {
	configPath string
}

// NewFirstRunHandler creates a new FirstRunHandler
func NewFirstRunHandler(configPath string) *FirstRunHandler {
	return &FirstRunHandler{
		configPath: configPath,
	}
}

// CheckFirstRun determines if this is the first time the application is running
// based on whether the configuration file exists
func (frh *FirstRunHandler) CheckFirstRun() (bool, error) {
	if frh.configPath == "" {
		// If no config path is specified, determine from default location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return false, fmt.Errorf("could not determine home directory: %w", err)
		}
		frh.configPath = filepath.Join(homeDir, ".config", "image-copier", "config.yaml")
	}

	// Check if config file exists
	_, err := os.Stat(frh.configPath)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking config file: %w", err)
	}

	return false, nil
}

// CreateSampleConfig creates a sample configuration file with properly formatted fields
// including examples of encrypted values
func (frh *FirstRunHandler) CreateSampleConfig() error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(frh.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Sample configuration with placeholders
	sampleConfig := `# Image Copier Configuration
# For encrypted values, use the format: encrypted:ENCRYPTED_VALUE
# To encrypt a value, use: go run cmd/encrypt/main.go "your-sensitive-value"

github:
  owner: "your-github-username"              # GitHub username or organization
  repo: "image-copier"                       # GitHub repository name
  token: "encrypted:YOUR_ENCRYPTED_TOKEN_HERE"   # GitHub personal access token (encrypted)
  workflow_id: "image-copier-v2.yaml"        # GitHub Actions workflow file name

registry:
  host: "registry.example.com"               # Your container registry host
  username: "encrypted:YOUR_ENCRYPTED_USERNAME_HERE" # Registry username (encrypted)
  password: "encrypted:YOUR_ENCRYPTED_PASSWORD_HERE" # Registry password/token (encrypted)
  namespace: "your-namespace"                # Registry namespace (optional)
  arch: "amd64"                              # Default image architecture
  os: "linux"                                # Default operating system

retry:
  max_attempts: "3"                          # Maximum retry attempts
  initial_interval: "2s"                     # Initial retry interval
  max_interval: "30s"                        # Maximum retry interval

log_level: "info"                            # Log level: debug/info/warn/error
`

	// Write the sample configuration
	file, err := os.Create(frh.configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(sampleConfig)
	if err != nil {
		return fmt.Errorf("failed to write to config file: %w", err)
	}

	return nil
}

// ValidateFirstRunPrerequisites checks if all prerequisites for first run are met
func (frh *FirstRunHandler) ValidateFirstRunPrerequisites() error {
	// Check if encryption key is set
	encryptKey := os.Getenv("ENCRYPT_KEY")
	if encryptKey == "" {
		return fmt.Errorf("ENCRYPT_KEY environment variable is not set - this is required for using encrypted configuration values")
	}

	if len(encryptKey) < 32 {
		return fmt.Errorf("ENCRYPT_KEY is too short: must be at least 32 characters long, got %d", len(encryptKey))
	}

	// Verify that we can create the config directory path
	dir := filepath.Dir(frh.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create configuration directory %s: %w", dir, err)
	}

	return nil
}

// HandleFirstRun executes the complete first run process
func (frh *FirstRunHandler) HandleFirstRun() error {
	isFirstRun, err := frh.CheckFirstRun()
	if err != nil {
		return fmt.Errorf("failed to check first run status: %w", err)
	}

	if !isFirstRun {
		return fmt.Errorf("configuration file already exists at %s, not a first run", frh.configPath)
	}

	fmt.Printf("This appears to be your first time running Image Copier.\n")

	// Validate prerequisites
	if err := frh.ValidateFirstRunPrerequisites(); err != nil {
		return fmt.Errorf("prerequisites not met: %w", err)
	}

	fmt.Printf("Creating sample configuration at: %s\n", frh.configPath)

	// Create sample config
	if err := frh.CreateSampleConfig(); err != nil {
		return fmt.Errorf("failed to create sample configuration: %w", err)
	}

	fmt.Printf("Sample configuration created successfully!\n")
	fmt.Printf("Please edit the configuration file to add your GitHub and registry details.\n")
	fmt.Printf("Remember to encrypt sensitive values like tokens and passwords.\n")

	return nil
}

// InitializeConfigPath determines the proper configuration file path based on
// environment and default conventions
func InitializeConfigPath(userProvidedPath string) string {
	if userProvidedPath != "" {
		return userProvidedPath
	}

	// Default to standard location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return "./config.yaml"
	}

	return filepath.Join(homeDir, ".config", "image-copier", "config.yaml")
}

// FirstRunChecker is a simple function to check if it's the first run
func FirstRunChecker(configPath string) (bool, error) {
	handler := NewFirstRunHandler(configPath)
	return handler.CheckFirstRun()
}

// SetupFirstRunIfNeeded handles first run setup if needed
func SetupFirstRunIfNeeded(userProvidedPath string) error {
	configPath := InitializeConfigPath(userProvidedPath)
	handler := NewFirstRunHandler(configPath)

	isFirstRun, err := handler.CheckFirstRun()
	if err != nil {
		return fmt.Errorf("error checking first run status: %w", err)
	}

	if isFirstRun {
		return handler.HandleFirstRun()
	}

	return nil
}
