package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/logformat"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// stageWeights defines the cumulative percentage weight for each pull stage.
var stageWeights = [6]float64{5, 15, 20, 80, 95, 100}

var stageNames = [6]string{
	"checking local",
	"checking registry",
	"triggering workflow",
	"workflow running",
	"downloading",
	"loading",
}

// asymptotic computes progress that slows as it approaches the ceiling.
// Formula: base + range * (1 - 1/(1 + k*polls))
func asymptotic(base, rangeSize float64, polls int) float64 {
	const k = 0.05
	return base + rangeSize*(1-1/(1+k*float64(polls)))
}

func NewPullCommand() *cobra.Command {
	var (
		arch        string
		osType      string
		multiArch   bool
		filePath    string
		workerCount int
		force       bool
		verbose     bool
	)

	cmd := &cobra.Command{
		Use:   "pull [IMAGE...]",
		Short: "Pull images through GitHub Actions",
		Long:  `Pull one or more images by routing them through GitHub Actions when direct pulling is not possible`,
		Args: func(cmd *cobra.Command, args []string) error {
			f, _ := cmd.Flags().GetString("file")
			if len(args) == 0 && f == "" {
				return fmt.Errorf("requires at least 1 image argument or --file flag")
			}
			return nil
		},
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
			logger.SetFormatter(&logformat.CLIFormatter{})

			// Use CLI flags if provided, otherwise use config
			if arch == "" {
				arch = cfg.Registry.Arch
			}
			if osType == "" {
				osType = cfg.Registry.Os
			}

			if multiArch {
				logger.Info("Multi-arch sync mode is enabled")
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
				Force:             force,
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

			ctx := context.Background()

			// Unified progress bar display for all modes
			return processImagesWithProgress(logger, pullerCfg, images, workerCount, verbose, ctx)
		},
	}

	// Flags
	cmd.Flags().StringVar(&arch, "arch", "", "Image architecture (e.g., amd64, arm64)")
	cmd.Flags().StringVar(&osType, "os", "", "Image operating system (e.g., linux)")
	cmd.Flags().BoolVar(&multiArch, "multi-arch", false, "Sync all available architectures")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing image list (one per line)")
	cmd.Flags().IntVarP(&workerCount, "jobs", "j", 3, "Number of concurrent workers")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-pull even if image exists locally")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed log output above progress bars")

	cmd.SilenceUsage = true

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

// processImagesWithProgress processes images with a progress bar and worker pool.
func processImagesWithProgress(logger *logrus.Logger, pullerCfg *core.Config, images []string, workerCount int, verbose bool, ctx context.Context) error {
	// Validate worker count
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(images) {
		workerCount = len(images)
	}

	// Create progress manager with pre-allocated worker bars
	p := progress.NewProgress(len(images), workerCount)
	for i, img := range images {
		p.AddImage(i, img)
	}

	// Route logger output based on verbose flag
	if verbose {
		logger.SetOutput(p.LogWriter())
	} else {
		logger.SetOutput(io.Discard)
	}
	defer logger.SetOutput(os.Stderr)

	// Create worker pool
	jobs := make(chan int, len(images))
	for i := range images {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var failCount atomic.Int32

	// Start workers — each worker owns a fixed worker bar
	for i := 0; i < workerCount; i++ {
		workerIdx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				p.UpdateStatus(idx, progress.StatusRunning, nil)
				startTime := time.Now()

				puller := core.NewPuller(pullerCfg, logger)
				puller.StageCallback = func(stage core.PullStage, polls int) {
					var pct float64
					stageIdx := int(stage)

					if stage == core.StageWaitWorkflow && polls > 0 {
						base := stageWeights[2]
						ceiling := stageWeights[3]
						pct = asymptotic(base, ceiling-base, polls)
					} else if stageIdx > 0 {
						pct = stageWeights[stageIdx-1]
					}

					p.UpdateStage(workerIdx, progress.StageInfo{
						Label:     images[idx],
						StageName: stageNames[stageIdx],
						Percent:   pct,
						StartAt:   startTime,
					})
				}

				err := puller.PullSingle(ctx, images[idx])
				elapsed := time.Since(startTime)

				if err != nil {
					if errors.Is(err, core.ErrSkipped) {
						p.UpdateStatus(idx, progress.StatusSkipped, nil)
					} else {
						p.UpdateStatus(idx, progress.StatusFailed, err)
						failCount.Add(1)
					}
				} else {
					p.UpdateStatus(idx, progress.StatusCompleted, nil)
				}
				p.SetDuration(idx, elapsed)

				p.UpdateWorker(workerIdx, "")
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
