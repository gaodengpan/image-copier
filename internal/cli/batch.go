package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/example/image-copier/internal/config"
	"github.com/example/image-copier/internal/core"
	"github.com/example/image-copier/pkg/progress"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewBatchCommand() *cobra.Command {
	var (
		filePath    string
		workerCount int
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Pull multiple images through GitHub Actions",
		Long:  `Pull multiple images either from command line arguments or from a file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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

			// Process images with progress bar
			return processImagesWithProgress(logger, pullerCfg, images, workerCount, ctx)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing image list (one per line)")
	cmd.Flags().IntVarP(&workerCount, "jobs", "j", 3, "Number of concurrent workers")

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

// processImagesWithProgress processes images with a progress bar and worker pool
func processImagesWithProgress(logger *logrus.Logger, pullerCfg *core.Config, images []string, workerCount int, ctx context.Context) error {
	// Validate worker count
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(images) {
		workerCount = len(images)
	}

	// Create progress manager
	p := progress.NewProgress(len(images))
	for i, img := range images {
		p.AddImage(i, img)
	}

	// Create worker pool
	jobs := make(chan int, len(images))
	for i := range images {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var failCount atomic.Int32

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				p.UpdateStatus(idx, progress.StatusRunning, nil)

				puller := core.NewPuller(pullerCfg, logger)
				err := puller.PullSingle(ctx, images[idx])

				if err != nil {
					p.UpdateStatus(idx, progress.StatusFailed, err)
					failCount.Add(1)
				} else {
					p.UpdateStatus(idx, progress.StatusCompleted, nil)
				}

				p.Increment()
			}
		}()
	}

	wg.Wait()
	p.Wait()

	if failCount.Load() > 0 {
		return fmt.Errorf("%d image(s) failed", failCount.Load())
	}
	return nil
}
