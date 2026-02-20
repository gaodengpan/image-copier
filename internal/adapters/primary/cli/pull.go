package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gaodengpan/image-copier/internal/adapters"
	"github.com/gaodengpan/image-copier/internal/application/usecases"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
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
	Output      string
	Timeout     time.Duration
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
		output      = opts.Output
		timeout     = opts.Timeout
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
			ctxNoTimeout := context.Background()

			var ctx context.Context
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			jobsSpecified := cmd.Flags().Changed("jobs")

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
				workerCount = calculateAdaptiveWorkerCount(jobsSpecified, workerCount, len(tasks), runtime.NumCPU())
				return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctxNoTimeout, ctx, output)
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

			workerCount = calculateAdaptiveWorkerCount(jobsSpecified, workerCount, len(tasks), runtime.NumCPU())
			return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctxNoTimeout, ctx, output)
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
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text or json")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Overall timeout for batch sync (e.g., 5m, 1h). Default: no timeout")

	cmd.SilenceUsage = true

	return cmd
}

func processSyncTasks(logger *logrus.Logger, baseCfg *config.Config, tasks []syncTask,
	workerCount int, force, dryRun, verbose bool, ctxDiff, ctxSync context.Context, outputFormat string) error {

	var presenter PullPresenter
	if outputFormat == "json" {
		presenter = NewJSONPresenter()
	} else {
		presenter = NewCLIPresenter()
	}
	presenter.PresentCheckingImageCount(len(tasks))

	factory := adapters.NewAdapterFactory(logger)

	// Convert CLI syncTask to use case SyncTask
	useCaseTasks := make([]use_cases.SyncTask, len(tasks))
	for i, t := range tasks {
		useCaseTasks[i] = use_cases.SyncTask{
			Source: t.Source,
			Arch:   t.Arch,
			Os:     t.Os,
		}
	}

	// Create use case for diff phase
	cfg := *baseCfg
	cfg.Force = force
	cfg.DryRun = dryRun
	diffUseCase := use_cases.NewSyncImagesUseCase(
		factory.CreateDockerClient(),
		factory.CreateRegistryClient(),
		factory.CreateGitHubClient(baseCfg.Github.Owner, baseCfg.Github.Repo, baseCfg.Github.Token, baseCfg.Github.WorkflowID),
		factory.CreateFileSystem(),
		factory.CreateHTTPClient(),
		logger,
		factory.CreateSystemClient(),
		factory.CreateImageIDService(),
		use_cases.SyncImagesConfig{
			Config: &cfg,
		},
	)

	// Execute diff phase through use case
	useCaseSynced, useCaseNeedsSync, err := diffUseCase.Diff(ctxDiff, useCaseTasks, workerCount, force)
	if err != nil {
		return fmt.Errorf("diff phase failed: %w", err)
	}

	// Convert use case results to CLI syncTask
	synced := make([]syncTask, len(useCaseSynced))
	for i, t := range useCaseSynced {
		synced[i] = syncTask{Source: t.Source, Arch: t.Arch, Os: t.Os}
	}
	needsSync := make([]syncTask, len(useCaseNeedsSync))
	for i, t := range useCaseNeedsSync {
		needsSync[i] = syncTask{Source: t.Source, Arch: t.Arch, Os: t.Os}
	}

	// Report diff results
	presenter.PresentDiffSummary(len(synced), len(needsSync))

	if dryRun {
		presenter.PresentDryRunResults(synced, needsSync)
		imageResults := make([]ImageResult, 0, len(synced)+len(needsSync))
		for _, t := range synced {
			imageResults = append(imageResults, ImageResult{
				Image:   t.Source,
				Arch:    t.Arch,
				Os:      t.Os,
				Skipped: true,
			})
		}
		for _, t := range needsSync {
			imageResults = append(imageResults, ImageResult{
				Image:  t.Source,
				Arch:   t.Arch,
				Os:     t.Os,
				DryRun: true,
			})
		}
		presenter.PresentSummary(&PullSummary{
			Succeeded: 0,
			Skipped:   len(synced),
			DryRun:    len(needsSync),
			Failed:    0,
		}, imageResults)
		return nil
	}

	if len(needsSync) == 0 {
		imageResults := make([]ImageResult, len(synced))
		for i, t := range synced {
			imageResults[i] = ImageResult{
				Image:   t.Source,
				Arch:    t.Arch,
				Os:      t.Os,
				Skipped: true,
			}
		}
		presenter.PresentSummary(&PullSummary{
			Succeeded: 0,
			Skipped:   len(synced),
			DryRun:    0,
			Failed:    0,
		}, imageResults)
		return nil
	}

	// === Phase 2: Sync ===

	// Progress tracks all images (needsSync + synced) for unified summary display
	totalCount := len(needsSync) + len(synced)
	noOutput := outputFormat == "json"
	p := progress.NewProgress(totalCount, workerCount, noOutput)

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
	processor := NewSyncTasksProcessor(
		logger,
		force,
		factory.CreateDockerClient(),
		factory.CreateRegistryClient(),
		factory.CreateGitHubClient(baseCfg.Github.Owner, baseCfg.Github.Repo, baseCfg.Github.Token, baseCfg.Github.WorkflowID),
		factory.CreateFileSystem(),
		factory.CreateHTTPClient(),
		factory.CreateSystemClient(),
		factory.CreateImageIDService(),
	)

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
		var taskIdx int64 = 0

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func(workerIdx int) {
				defer wg.Done()
				for {
					idx := int(atomic.AddInt64(&taskIdx, 1) - 1)
					if idx >= len(syncTasks) {
						break
					}

					// 1. Acquire semaphore
					sem <- struct{}{}
					// 2. defer release semaphore (always runs)
					defer func() {
						<-sem
					}()

					// 3. Check context cancellation BEFORE processing
					select {
					case <-ctxSync.Done():
						if ctxSync.Err() == context.DeadlineExceeded {
							p.UpdateStatus(idx, progress.StatusCancelled, fmt.Errorf("sync cancelled: timeout exceeded"))
							atomic.AddInt32(&failCount, 1)
						}
						// defer will release semaphore, return without processing
						return
					default:
					}

					// 4. Register defer for increment BEFORE processing (runs after normal completion, or on any return)
					defer p.Increment()

					// 5. Mark as running
					p.UpdateStatus(idx, progress.StatusRunning, nil)

					taskStart := time.Now()
					err := processor.Process(ctxSync, syncTasks[idx], p, workerIdx)
					elapsed := time.Since(taskStart)

					// Check context after processing - this catches timeout/cancellation
					// even when the process was killed (signal: killed)
					if ctxSync.Err() != nil {
						if ctxSync.Err() == context.DeadlineExceeded {
							p.UpdateStatus(idx, progress.StatusCancelled, fmt.Errorf("sync cancelled: timeout exceeded"))
							atomic.AddInt32(&failCount, 1)
						} else if ctxSync.Err() == context.Canceled {
							p.UpdateStatus(idx, progress.StatusCancelled, ctxSync.Err())
							atomic.AddInt32(&failCount, 1)
						}
					} else if err != nil {
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
				}
			}(i)
		}
		wg.Wait()
	}

	// Wait for progress bars to finish (without auto-summary)
	p.AbortWorkers()
	p.WaitContainer()

	duration := time.Since(startTime)

	// Build image results from actual progress status
	images := p.GetImages()
	imageResults := make([]ImageResult, 0, totalCount)

	// needsSync images are at indices 0 to len(needsSync)-1
	for i := 0; i < len(needsSync); i++ {
		if i >= len(images) || images[i] == nil {
			continue
		}
		img := images[i]
		t := needsSync[i]
		switch img.Status {
		case progress.StatusCompleted:
			imageResults = append(imageResults, ImageResult{
				Image:   t.Source,
				Arch:    t.Arch,
				Os:      t.Os,
				Success: true,
			})
		case progress.StatusSkipped:
			imageResults = append(imageResults, ImageResult{
				Image:   t.Source,
				Arch:    t.Arch,
				Os:      t.Os,
				Skipped: true,
			})
		case progress.StatusDryRun:
			imageResults = append(imageResults, ImageResult{
				Image:  t.Source,
				Arch:   t.Arch,
				Os:     t.Os,
				DryRun: true,
			})
		case progress.StatusFailed:
			errMsg := ""
			if img.Error != nil {
				errMsg = img.Error.Error()
			}
			imageResults = append(imageResults, ImageResult{
				Image:  t.Source,
				Arch:   t.Arch,
				Os:     t.Os,
				Failed: true,
				Error:  errMsg,
			})
		case progress.StatusCancelled:
			errMsg := ""
			if img.Error != nil {
				errMsg = img.Error.Error()
			}
			imageResults = append(imageResults, ImageResult{
				Image:     t.Source,
				Arch:      t.Arch,
				Os:        t.Os,
				Cancelled: true,
				Error:     errMsg,
			})
		default:
			imageResults = append(imageResults, ImageResult{
				Image:   t.Source,
				Arch:    t.Arch,
				Os:      t.Os,
				Success: true,
			})
		}
	}

	// synced images are at indices len(needsSync) onwards
	for i := len(needsSync); i < len(synced)+len(needsSync); i++ {
		if i >= len(images) || images[i] == nil {
			continue
		}
		t := synced[i-len(needsSync)]
		imageResults = append(imageResults, ImageResult{
			Image:   t.Source,
			Arch:    t.Arch,
			Os:      t.Os,
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
		errMsg := ""
		if ctxSync.Err() == context.DeadlineExceeded {
			errMsg = " (timeout exceeded)"
		}
		// In JSON mode, the JSON output has already been printed above.
		// Return nil to avoid printing error to stderr, which would pollute JSON output.
		if outputFormat == "json" {
			return nil
		}
		return fmt.Errorf("%d image(s) failed to sync%s", failCount, errMsg)
	}

	return nil
}
