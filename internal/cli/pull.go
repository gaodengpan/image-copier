package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	dockeradapter "github.com/gaodengpan/image-copier/internal/adapters/docker"
	registryadapter "github.com/gaodengpan/image-copier/internal/adapters/registry"
	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/use_cases"
	"github.com/gaodengpan/image-copier/pkg/logformat"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// PullCommandOptions defines options for the pull command
type PullCommandOptions struct {
	Arch        string
	OsType      string
	FilePath    string
	WorkerCount int
	Force       bool
	DryRun      bool
	Verbose     bool
}

// NewPullCommandWithConfigProvider creates a new pull command that accepts a ConfigProvider
func NewPullCommandWithConfigProvider(configProvider config.ConfigProvider) *cobra.Command {
	return NewPullCommandWithConfigProviderAndOptions(configProvider, PullCommandOptions{
		WorkerCount: 3,
	})
}

// NewPullCommandWithConfigProviderAndOptions creates a new pull command with a ConfigProvider and specified options
func NewPullCommandWithConfigProviderAndOptions(configProvider config.ConfigProvider, opts PullCommandOptions) *cobra.Command {
	var (
		arch        = opts.Arch
		osType      = opts.OsType
		filePath    = opts.FilePath
		workerCount = opts.WorkerCount
		force       = opts.Force
		dryRun      = opts.DryRun
		verbose     = opts.Verbose
	)

	cmd := &cobra.Command{
		Use:   "pull [IMAGE...]",
		Short: "Pull images through GitHub Actions",
		Long: `Pull one or more images by routing them through GitHub Actions when direct pulling is not possible.

Supports two modes:
1. Command-line mode: Specify images directly as arguments
2. Manifest mode: Use -f flag with YAML manifest for declarative sync with diff`,
		Args: func(cmd *cobra.Command, args []string) error {
			f, _ := cmd.Flags().GetString("file")
			if len(args) == 0 && f == "" {
				return fmt.Errorf("requires at least 1 image argument or --file flag")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configProvider.Load()
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

			baseCfg := CreateCoreConfigFromConfig(cfg, force, dryRun)
			ctx := context.Background()

			// === YAML manifest 模式（-f 参数）===
			if filePath != "" {
				tasks, err := readSyncManifest(filePath, arch, osType)
				if err != nil {
					return err
				}
				if len(tasks) == 0 {
					fmt.Println("No images found in manifest.")
					return nil
				}
				return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctx)
			}

			// === 命令式模式（CLI 参数）===
			// Override the arch and os with CLI flag values if they were specified
			if arch != "" {
				baseCfg.Registry.Arch = arch
			}
			if osType != "" {
				baseCfg.Registry.Os = osType
			}

			// Convert CLI args to syncTask format
			tasks := make([]syncTask, len(args))
			for i, img := range args {
				tasks[i] = syncTask{
					Source: img,
					Arch:   baseCfg.Registry.Arch,
					Os:     baseCfg.Registry.Os,
				}
			}

			return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctx)
		},
	}

	// Flags
	cmd.Flags().StringVar(&arch, "arch", "", "Image architecture (e.g., amd64, arm64)")
	cmd.Flags().StringVar(&osType, "os", "", "Image operating system (e.g., linux)")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML manifest file")
	cmd.Flags().IntVarP(&workerCount, "jobs", "j", 3, "Number of concurrent workers")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-pull even if image exists locally")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed log output above progress bars")

	cmd.SilenceUsage = true

	return cmd
}

// SyncManifest represents the YAML manifest structure.
type SyncManifest struct {
	Images []SyncImage `yaml:"images"`
}

// SyncImage represents a single image entry in the manifest.
type SyncImage struct {
	Source    string   `yaml:"source"`
	Platforms []string `yaml:"platforms"`
}

type syncTask struct {
	Source string
	Arch   string
	Os     string
}

// displayName returns a human-readable label like "nginx:latest (linux/amd64)"
func (t syncTask) displayName() string {
	return fmt.Sprintf("%s (%s/%s)", t.Source, t.Os, t.Arch)
}

func readSyncManifest(path, defaultArch, defaultOs string) ([]syncTask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var manifest SyncManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var tasks []syncTask
	for _, img := range manifest.Images {
		if img.Source == "" {
			continue
		}
		platforms := img.Platforms
		if len(platforms) == 0 {
			platforms = []string{defaultOs + "/" + defaultArch}
		}
		for _, plat := range platforms {
			parts := strings.SplitN(plat, "/", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid platform format %q (expected os/arch)", plat)
			}
			tasks = append(tasks, syncTask{
				Source: img.Source,
				Arch:   parts[1],
				Os:     parts[0],
			})
		}
	}
	return tasks, nil
}

func processSyncTasks(logger *logrus.Logger, baseCfg *config.Config, tasks []syncTask,
	workerCount int, force, dryRun, verbose bool, ctx context.Context) error {

	presenter := NewCLIPresenter()
	presenter.PresentCheckingImageCount(len(tasks))

	// === Phase 1: Diff ===
	type diffResult struct {
		task         syncTask
		remoteExists bool
		localExists  bool
	}
	results := make([]diffResult, len(tasks))

	// Create adapters for local and registry image checks
	dockerClient := dockeradapter.NewExecDockerAdapter()
	registryClient := registryadapter.NewSkopeoAdapter()

	// Concurrent check (bounded by workerCount)
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, task syncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sourceID := task.Source
			destID := registryClient.BuildDestImageID(sourceID, baseCfg.Registry.Host, baseCfg.Registry.Namespace)
			remoteExists, _ := registryClient.CheckImageExists(ctx, destID, baseCfg.Registry.Username, baseCfg.Registry.Password)
			localExists, _ := dockerClient.ImageExists(ctx, task.Source)
			results[idx] = diffResult{task: task, remoteExists: remoteExists, localExists: localExists}
		}(i, t)
	}
	wg.Wait()

	// Partition: synced vs needsSync
	// An image is synced if it exists locally (no need to pull from source)
	var synced, needsSync []syncTask
	for _, r := range results {
		if r.localExists && !force {
			synced = append(synced, r.task)
		} else {
			needsSync = append(needsSync, r.task)
		}
	}

	// Report diff results
	presenter.PresentDiffSummary(len(synced), len(needsSync))

	if dryRun {
		presenter.PresentDryRunResults(synced, needsSync)
		presenter.PresentSummary(&PullSummary{
			Succeeded: 0,
			Skipped:   len(synced),
			DryRun:    len(needsSync),
			Failed:    0,
		}, nil)
		return nil
	}

	if len(needsSync) == 0 {
		fmt.Println("Everything is up to date.")
		return nil
	}

	// === Phase 2: Sync ===
	// Optimize worker count for sync phase
	if workerCount > len(needsSync) {
		workerCount = len(needsSync)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	// Limit worker count by CPU cores to prevent excessive resource usage
	maxWorkersByCPU := runtime.NumCPU()
	if workerCount > maxWorkersByCPU {
		logger.Debugf("Reducing sync worker count from %d to %d based on CPU cores", workerCount, maxWorkersByCPU)
		workerCount = maxWorkersByCPU
	}

	// Progress tracks all images (needsSync + synced) for unified summary display
	totalCount := len(needsSync) + len(synced)
	p := progress.NewProgress(totalCount, workerCount)

	// Add needsSync images to progress
	for i, t := range needsSync {
		p.AddImage(i, t.displayName())
	}

	// Add synced images to progress and mark as skipped
	for i, t := range synced {
		p.AddImage(len(needsSync)+i, t.displayName())
		p.UpdateStatus(len(needsSync)+i, progress.StatusSkipped, nil)
	}

	// Set initial progress to the number of already synced images
	p.SetInitialProgress(len(synced))

	// Create tasks for the worker pool (only needsSync)
	syncTasks := make([]WorkerPoolTask[SyncTask], len(needsSync))
	for i, t := range needsSync {
		syncTasks[i] = WorkerPoolTask[SyncTask]{
			Index: i,
			Item: SyncTask{
				Source: t.Source,
				Arch:   t.Arch,
				Os:     t.Os,
			},
			Config: baseCfg,
		}
	}

	// Create processor
	processor := NewSyncTasksProcessor(logger, force)

	// Execute with inline worker pool (skip progress.Wait() auto-summary)
	startTime := time.Now()
	var failCount int32 = 0

	// Worker pool execution (inline to control summary output)
	if len(syncTasks) > 0 {
		// Route logger output based on verbose flag
		if verbose {
			logger.SetOutput(p.LogWriter())
		} else {
			logger.SetOutput(io.Discard)
		}
		defer logger.SetOutput(os.Stderr)

		sem := make(chan struct{}, workerCount)
		var wg sync.WaitGroup

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func(workerIdx int) {
				defer wg.Done()
				for idx := 0; idx < len(syncTasks); idx++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					sem <- struct{}{}
					p.UpdateStatus(idx, progress.StatusRunning, nil)
					taskStart := time.Now()

					err := processor.Process(ctx, syncTasks[idx], p, workerIdx)
					elapsed := time.Since(taskStart)

					if err != nil {
						if errors.Is(err, use_cases.ErrSkipped) {
							p.UpdateStatus(idx, progress.StatusSkipped, nil)
						} else if errors.Is(err, use_cases.ErrDryRun) {
							p.UpdateStatus(idx, progress.StatusDryRun, nil)
						} else {
							p.UpdateStatus(idx, progress.StatusFailed, err)
							atomic.AddInt32(&failCount, 1)
						}
					} else {
						p.UpdateStatus(idx, progress.StatusCompleted, nil)
					}

					p.SetDuration(idx, elapsed)
					p.UpdateWorker(workerIdx, "")
					p.Increment()
					<-sem
				}
			}(i)
		}
		wg.Wait()
	}

	// Wait for progress bars to finish (without auto-summary)
	p.AbortWorkers()
	p.WaitContainer()

	duration := time.Since(startTime)

	// Build image results for summary
	imageResults := make([]ImageResult, 0, totalCount)
	for _, t := range needsSync {
		imageResults = append(imageResults, ImageResult{
			Image:   t.displayName(),
			Success: true,
		})
	}
	for _, t := range synced {
		imageResults = append(imageResults, ImageResult{
			Image:   t.displayName(),
			Skipped: true,
		})
	}

	// Present summary
	presenter.PresentSummary(&PullSummary{
		Succeeded: len(needsSync) - int(failCount),
		Skipped:   len(synced),
		DryRun:    0,
		Failed:    int(failCount),
		Duration:  duration,
	}, imageResults)

	if failCount > 0 {
		return fmt.Errorf("%d image(s) failed to sync", failCount)
	}

	return nil
}
