package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/example/image-copier/internal/config"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View and manage image-copier configuration`,
	}

	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigInitCommand())

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  `Display the current configuration being used`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.GetConfigPath()
			if path != "" {
				fmt.Printf("Using config file: %s\n\n", path)
			} else {
				fmt.Println("No config file found, using environment variables only")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Printf("GitHub Owner: %s\n", cfg.Github.Owner)
			fmt.Printf("GitHub Repo: %s\n", cfg.Github.Repo)
			fmt.Printf("GitHub Workflow ID: %s\n", cfg.Github.WorkflowID)
			fmt.Printf("Registry Host: %s\n", cfg.Registry.Host)
			fmt.Printf("Registry Username: %s\n", cfg.Registry.Username)
			fmt.Printf("Registry Namespace: %s\n", cfg.Registry.Namespace)
			fmt.Printf("Registry Arch: %s\n", cfg.Registry.Arch)
			fmt.Printf("Registry OS: %s\n", cfg.Registry.Os)
			fmt.Printf("Log Level: %s\n", cfg.LogLevel)

			return nil
		},
	}

	return cmd
}

func newConfigInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a sample configuration file",
		Long:  `Create a sample configuration file in the current directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := `# GitHub Configuration
github:
  # GitHub repository owner (user or organization)
  owner: ""
  # GitHub repository name
  repo: ""
  # GitHub personal access token with workflow permissions
  token: ""
  # Workflow filename (usually ends with .yaml)
  workflow_id: "image-copier-v2.yaml"

# Registry Configuration
registry:
  # Domestic registry host (e.g., registry.cn-hangzhou.aliyuncs.com)
  host: ""
  # Registry username
  username: ""
  # Registry password or access token
  password: ""
  # Optional namespace for organizing images
  namespace: ""
  # Architecture for multi-platform images (default: amd64)
  arch: "amd64"
  # Operating system for multi-platform images (default: linux)
  os: "linux"

# Logging Configuration
log_level: "info"
`

			filename := "config.yaml"
			if _, err := os.Stat(filename); err == nil {
				return fmt.Errorf("config file %s already exists", filename)
			}

			if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("Sample configuration file created: %s\n", filename)
			fmt.Println("Please edit the file with your actual configuration values.")

			return nil
		},
	}

	return cmd
}
