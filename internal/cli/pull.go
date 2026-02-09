package cli

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/example/image-copier/internal/config"
	"github.com/example/image-copier/internal/core"
)

func NewPullCommand() *cobra.Command {
	var (
		arch      string
		osType    string
		multiArch bool
	)

	cmd := &cobra.Command{
		Use:   "pull IMAGE_ID",
		Short: "Pull a single image through GitHub Actions",
		Long:  `Pull a single image by routing it through GitHub Actions when direct pulling is not possible`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Convert log level string to logrus.Level
			level, err := logrus.ParseLevel(cfg.LogLevel)
			if err != nil {
				level = logrus.InfoLevel
			}

			logger := logrus.New()
			logger.SetLevel(level)

			// Use CLI flags if provided, otherwise use config
			if arch == "" {
				arch = cfg.Registry.Arch
			}
			if osType == "" {
				osType = cfg.Registry.Os
			}

			if multiArch {
				logger.Info("Multi-arch sync mode is enabled")
				// TODO: Implement multi-arch sync logic
				// For now, we'll log a warning and proceed with the requested arch
				logger.Warnf("Multi-arch sync not yet fully implemented, using arch: %s, os: %s", arch, osType)
			}

			pullerCfg := &core.Config{
				GithubOwner:       cfg.Github.Owner,
				GithubRepo:        cfg.Github.Repo,
				GithubToken:       cfg.Github.Token,
				GithubWorkflowID:  cfg.Github.WorkflowID,
				RegistryHost:      cfg.Registry.Host,
				RegistryUsername:  cfg.Registry.Username,
				RegistryPassword:  cfg.Registry.Password,
				RegistryNamespace: cfg.Registry.Namespace,
				RegistryArch:      arch,
				RegistryOs:        osType,
			}

			puller := core.NewPuller(pullerCfg, logger)

			ctx := context.Background()
			if err := puller.PullSingle(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to pull image: %w", err)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&arch, "arch", "", "Image architecture (e.g., amd64, arm64)")
	cmd.Flags().StringVar(&osType, "os", "", "Image operating system (e.g., linux)")
	cmd.Flags().BoolVar(&multiArch, "multi-arch", false, "Sync all available architectures")

	return cmd
}
