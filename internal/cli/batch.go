package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/example/image-copier/internal/config"
	"github.com/example/image-copier/internal/core"
)

func NewBatchCommand() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Pull multiple images through GitHub Actions",
		Long:  `Pull multiple images either from command line arguments or from a file`,
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

			pullerCfg := &core.Config{
				GithubOwner:       cfg.Github.Owner,
				GithubRepo:        cfg.Github.Repo,
				GithubToken:       cfg.Github.Token,
				GithubWorkflowID:  cfg.Github.WorkflowID,
				RegistryHost:      cfg.Registry.Host,
				RegistryUsername:  cfg.Registry.Username,
				RegistryPassword:  cfg.Registry.Password,
				RegistryNamespace: cfg.Registry.Namespace,
				RegistryArch:      cfg.Registry.Arch,
				RegistryOs:        cfg.Registry.Os,
			}

			images := args

			// If file path is provided, read images from file
			if filePath != "" {
				fileImages, err := readImagesFromFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read images from file: %w", err)
				}
				images = append(images, fileImages...)
			}

			if len(images) == 0 {
				return fmt.Errorf("no images provided")
			}

			// Process images concurrently
			return processImagesConcurrently(logger, pullerCfg, images)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing image list (one per line)")

	return cmd
}

func readImagesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var images []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		if line != "" && line[0] != '#' {
			images = append(images, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return images, nil
}

func processImagesConcurrently(logger *logrus.Logger, pullerCfg *core.Config, images []string) error {
	type result struct {
		image string
		err   error
	}

	results := make(chan result, len(images))
	var wg sync.WaitGroup

	// Start workers
	for _, image := range images {
		wg.Add(1)
		go func(img string) {
			defer wg.Done()

			puller := core.NewPuller(pullerCfg, logger)
			ctx := context.Background()

			err := puller.PullSingle(ctx, img)
			results <- result{image: img, err: err}
		}(image)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var errors []error
	successCount := 0

	for res := range results {
		if res.err != nil {
			logger.Errorf("Failed to process image %s: %v", res.image, res.err)
			errors = append(errors, res.err)
		} else {
			logger.Infof("Successfully processed image %s", res.image)
			successCount++
		}
	}

	logger.Infof("Processed %d images successfully, %d failed", successCount, len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("failed to process %d images", len(errors))
	}

	return nil
}
