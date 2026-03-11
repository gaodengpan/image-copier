package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gaodengpan/image-copier/internal/adapters"
	"github.com/gaodengpan/image-copier/internal/adapters/secondary/gateways"
	use_cases "github.com/gaodengpan/image-copier/internal/application/usecases"
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/gaodengpan/image-copier/pkg/logformat"
	"github.com/gaodengpan/image-copier/pkg/progress"
)

// SyncCommandOptions defines options for the sync command
type SyncCommandOptions struct {
	Arch           string
	OsType         string
	FilePath       string
	WorkerCount    int
	Force          bool
	DryRun         bool
	Verbose        bool
	Output         string
	Timeout        time.Duration
	Targets        []string
	SkipSync       bool
	SkipDistribute bool
}

// NewSyncCommand creates a new sync command
func NewSyncCommand(configProvider config.ConfigProvider) *cobra.Command {
	return NewSyncCommandWithOptions(configProvider, SyncCommandOptions{
		WorkerCount: 3,
	})
}

// NewSyncCommandWithOptions creates a new sync command with specified options
func NewSyncCommandWithOptions(configProvider config.ConfigProvider, opts SyncCommandOptions) *cobra.Command {
	var (
		arch           = opts.Arch
		osType         = opts.OsType
		filePath       = opts.FilePath
		workerCount    = opts.WorkerCount
		force          = opts.Force
		dryRun         = opts.DryRun
		verbose        = opts.Verbose
		output         = opts.Output
		timeout        = opts.Timeout
		targets        = opts.Targets
		skipSync       = opts.SkipSync
		skipDistribute = opts.SkipDistribute
	)

	cmd := &cobra.Command{
		Use:   "sync [IMAGE...]",
		Short: "Sync images through GitHub Actions and distribute to targets",
		Long: `Two-phase image synchronization:

Phase 1 (Sync): Sync images from foreign sources to staging registry via GitHub Actions
Phase 2 (Distribute): Distribute images from staging registry to local Docker and/or private registries

Supports two input modes:
1. Command-line mode: Specify images directly as arguments
2. Manifest mode: Use -f flag with YAML manifest file`,
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

			// Setup logger
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

			// Setup context with timeout
			var ctx context.Context
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			// Calculate worker count and build images list
			jobsSpecified := cmd.Flags().Changed("jobs")
			imageCount := len(args)
			var manifestTasks []syncTask

			if filePath != "" {
				tasks, err := readSyncManifest(filePath, arch, osType)
				if err != nil {
					return err
				}
				manifestTasks = tasks
				imageCount = len(tasks)
			}
			workerCount = calculateAdaptiveWorkerCount(jobsSpecified, workerCount, imageCount, runtime.NumCPU())

			// Build images list
			images := args
			if manifestTasks != nil {
				images = make([]string, len(manifestTasks))
				for i, t := range manifestTasks {
					images[i] = t.Source
				}
			}

			// Execute sync command
			return executeSyncCommand(ctx, logger, cfg, images, input.SyncCommandInput{
				Images:         images,
				ManifestFile:   filePath,
				Arch:           arch,
				Os:             osType,
				Force:          force,
				DryRun:         dryRun,
				WorkerCount:    workerCount,
				Timeout:        timeout,
				Targets:        targets,
				SkipSync:       skipSync,
				SkipDistribute: skipDistribute,
			}, output, verbose)
		},
	}

	// Flags
	cmd.Flags().StringVar(&arch, "arch", "", "Image architecture (e.g., amd64, arm64)")
	cmd.Flags().StringVar(&osType, "os", "", "Image operating system (e.g., linux)")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML manifest file")
	cmd.Flags().IntVarP(&workerCount, "jobs", "j", 3, "Number of concurrent workers")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-sync even if image exists")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed log output")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text or json")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Overall timeout (e.g., 5m, 1h)")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "Distribution targets (can be specified multiple times)")
	cmd.Flags().BoolVar(&skipSync, "skip-sync", false, "Skip sync phase (only distribute)")
	cmd.Flags().BoolVar(&skipDistribute, "skip-distribute", false, "Skip distribute phase (only sync)")

	cmd.SilenceUsage = true

	return cmd
}

// executeSyncCommand executes the sync command
func executeSyncCommand(
	ctx context.Context,
	logger *logrus.Logger,
	cfg *config.Config,
	images []string,
	syncInput input.SyncCommandInput,
	outputFormat string,
	verbose bool,
) error {
	// Set log level based on verbose flag
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		// In non-verbose mode, suppress info logs to avoid interfering with progress bar
		logger.SetLevel(logrus.WarnLevel)
	}

	if len(images) == 0 {
		fmt.Println("No images to sync.")
		return nil
	}

	// Create presenter
	var presenter SyncPresenter
	if outputFormat == "json" {
		presenter = NewSyncJSONPresenter()
	} else {
		presenter = NewSyncCLIPresenter()
	}

	// Present start
	presenter.PresentSyncStart(len(images))

	// Create progress manager
	prog := presenter.PresentProgress(len(images), syncInput.WorkerCount)

	// Redirect logger output to progress bar's LogWriter for proper integration
	// This ensures logs appear above the progress bar without breaking the animation
	if !prog.IsNoOutput() {
		logger.SetOutput(prog.LogWriter())
	}

	// Build image map for progress tracking
	imageMap := make(map[string]int)
	for i, img := range images {
		imageMap[img] = i
		prog.AddImage(i, img)
	}

	// Create progress callback
	syncInput.ProgressCallback = func(imageID string, stage value_objects.SyncStage, targetName string, percent float64) {
		if idx, ok := imageMap[imageID]; ok {
			prog.UpdateSyncStage(idx%syncInput.WorkerCount, imageID, stage, targetName, percent)
		}
	}

	// Create task complete callback for incrementing main progress bar
	syncInput.TaskComplete = func(imageID string, err error) {
		if idx, ok := imageMap[imageID]; ok {
			if err != nil {
				prog.UpdateStatus(idx, progress.StatusFailed, err)
			} else {
				prog.UpdateStatus(idx, progress.StatusCompleted, nil)
			}
			prog.Increment()
		}
	}

	// Create factory and strategies
	factory := adapters.NewAdapterFactory(logger)

	registryClient := factory.CreateRegistryClient()

	dockerStrategy := gateways.NewDockerSyncStrategy(
		factory.CreateDockerClient(),
		registryClient,
		factory.CreateFileSystem(),
	)

	registryStrategy := gateways.NewRegistrySyncStrategy(
		registryClient,
	)

	// Create distributor
	distributor := gateways.NewMultiTargetDistributor(
		dockerStrategy,
		registryStrategy,
		logger,
	)

	// Create config adapter
	configAdapter := gateways.NewConfigAdapter(cfg, logger)

	// Create use case
	syncUseCase := use_cases.NewSyncCommandUseCase(
		registryClient,
		factory.CreateGitHubClient(cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Token, cfg.Github.WorkflowID),
		logger,
		factory.CreateImageIDService(),
		configAdapter,
		configAdapter,
		distributor,
	)

	// Execute
	result, err := syncUseCase.Execute(ctx, syncInput)
	if err != nil {
		prog.AbortWorkers()
		presenter.PresentError(err)
		return err
	}

	// Wait for progress bar to complete
	prog.Wait()

	// Present phase results for JSON mode
	if outputFormat == "json" {
		if result.SyncPhase != nil {
			presenter.PresentSyncPhaseResult(result.SyncPhase)
		}
		if result.DistributePhase != nil {
			presenter.PresentDistributePhaseResult(result.DistributePhase)
		}
	}

	// Build summary with nil-safe access
	summary := &SyncSummary{
		TotalImages: len(images),
		Duration:    result.Duration,
	}
	if result.SyncPhase != nil {
		summary.SyncSuccess = len(result.SyncPhase.AlreadyExisted) + len(result.SyncPhase.NewlySynced)
		summary.SyncFailed = len(result.SyncPhase.Failed)
	}
	if result.DistributePhase != nil {
		summary.DistSuccess = result.DistributePhase.SuccessCount
		summary.DistSkipped = result.DistributePhase.SkippedCount
		summary.DistFailed = result.DistributePhase.FailedCount
	}
	presenter.PresentSummary(summary)

	// Return error if any failures
	syncFailed := 0
	distFailed := 0
	if result.SyncPhase != nil {
		syncFailed = len(result.SyncPhase.Failed)
	}
	if result.DistributePhase != nil {
		distFailed = result.DistributePhase.FailedCount
	}
	if syncFailed > 0 || distFailed > 0 {
		if outputFormat != "json" {
			return fmt.Errorf("sync completed with %d sync failures and %d distribution failures",
				syncFailed, distFailed)
		}
	}

	return nil
}

// SyncPresenter defines the interface for presenting sync results
type SyncPresenter interface {
	PresentSyncStart(count int)
	PresentProgress(total int, workerCount int) *progress.Progress
	PresentSyncPhaseResult(result *input.SyncPhaseResult)
	PresentDistributePhaseResult(result *input.DistributePhaseResult)
	PresentSummary(summary *SyncSummary)
	PresentError(err error)
}

// SyncSummary represents the summary of a sync operation
type SyncSummary struct {
	TotalImages int
	SyncSuccess int
	SyncFailed  int
	DistSuccess int
	DistSkipped int
	DistFailed  int
	Duration    time.Duration
}
