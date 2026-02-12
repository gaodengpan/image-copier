package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gaodengpan/image-copier/internal/cli"
	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "image-copier",
		Short:   "A tool to copy images from foreign registries to domestic ones",
		Version: version.Version,
		Long: `Image-copier is a CLI tool that helps pull images from foreign registries
through GitHub Actions when direct pulling is not possible due to network restrictions.`,
	}
	rootCmd.SetVersionTemplate(fmt.Sprintf("image-copier %s (commit: %s, built: %s)\n",
		version.Version, version.Commit, version.Date))

	// Use default config provider for standard execution
	configProvider := config.DefaultConfigProvider()

	rootCmd.AddCommand(cli.NewPullCommandWithConfigProvider(configProvider))
	rootCmd.AddCommand(cli.NewConfigCommandWithConfigProvider(configProvider))

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
