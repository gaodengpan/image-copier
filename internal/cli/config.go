package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gaodengpan/image-copier/internal/config"
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
	var (
		skipExisting bool
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create configuration file interactively",
		Long:  `Create a configuration file through an interactive wizard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filename := config.ConfigFilePath()

			// Check if file exists
			if _, err := os.Stat(filename); err == nil && !force {
				return fmt.Errorf("config file %s already exists. Use --force to overwrite", filename)
			}

			// Run wizard
			ctx := context.Background()
			data, err := RunWizard(ctx, skipExisting)
			if err != nil {
				return fmt.Errorf("wizard failed: %w", err)
			}

			// Ensure config directory exists
			if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Write config file
			if err := WriteConfigFile(data, filename); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("\nConfiguration file created: %s\n", filename)
			fmt.Println("You can now use image-copier!")
			fmt.Println("\nIf you need to make changes later, you can:")
			fmt.Println("  - Edit the config file directly")
			fmt.Println("  - Run 'image-copier config init --force' to reconfigure")
			fmt.Println("  - Use environment variables to override settings")

			return nil
		},
	}

	cmd.Flags().BoolVar(&skipExisting, "skip-existing", true, "Skip already configured values")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file")

	return cmd
}
