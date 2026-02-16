package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	dockeradapter "github.com/gaodengpan/image-copier/internal/adapters/docker"
	registryadapter "github.com/gaodengpan/image-copier/internal/adapters/registry"
	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/core"
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
				baseCfg := CreateCoreConfigFromConfig(cfg, force, dryRun)
				ctx := context.Background()
				return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctx)
			}

			// === 命令式模式（CLI 参数）===
			pullerCfg := CreateCoreConfigFromConfig(cfg, force, dryRun)
			// Override the arch and os with CLI flag values if they were specified
			if arch != "" {
				pullerCfg.RegistryArch = arch
			}
			if osType != "" {
				pullerCfg.RegistryOs = osType
			}

			images := args
			ctx := context.Background()

			// Unified progress bar display for all modes
			return processImagesWithProgress(logger, pullerCfg, images, workerCount, verbose, ctx)
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

// processImagesWithProgress processes images with a progress bar and worker pool.
func processImagesWithProgress(logger *logrus.Logger, pullerCfg *core.Config, images []string, workerCount int, verbose bool, ctx context.Context) error {
	// Validate and optimize worker count
	if workerCount < 1 {
		workerCount = 1
	}

	// Adjust worker count based on number of images and CPU cores
	maxFromImages := len(images)
	if maxFromImages == 0 {
		maxFromImages = 1
	}

	if workerCount > maxFromImages {
		workerCount = maxFromImages
	}

	// Limit worker count by CPU cores to prevent excessive resource usage
	maxWorkersByCPU := runtime.NumCPU()
	if workerCount > maxWorkersByCPU {
		logger.Debugf("Reducing worker count from %d to %d based on CPU cores", workerCount, maxWorkersByCPU)
		workerCount = maxWorkersByCPU
	}

	// Create progress manager with pre-allocated worker bars
	p := progress.NewProgress(len(images), workerCount)
	for i, img := range images {
		p.AddImage(i, img)
	}

	// Create tasks
	tasks := make([]WorkerPoolTask[CLIImage], len(images))
	for i, img := range images {
		tasks[i] = WorkerPoolTask[CLIImage]{
			Index:  i,
			Item:   CLIImage{ImageID: img},
			Config: pullerCfg,
		}
	}

	// Create processor
	processor := NewCLIImagesProcessor(logger)

	// Execute with generic worker pool
	failCount, err := GenericWorkerPool(logger, tasks, processor, p, workerCount, verbose, ctx)
	if err != nil {
		return fmt.Errorf("%d image(s) failed", failCount)
	}
	return nil
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

func processSyncTasks(logger *logrus.Logger, baseCfg *core.Config, tasks []syncTask,
	workerCount int, force, dryRun, verbose bool, ctx context.Context) error {

	// === Phase 1: Diff ===
	fmt.Printf("Checking %d image(s) against destination registry...\n", len(tasks))

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

			sourceID := core.NormalizeSourceID(task.Source)
			destID := registryClient.BuildDestImageID(sourceID, baseCfg.RegistryHost, baseCfg.RegistryNamespace)
			remoteExists, _ := registryClient.CheckImageExists(ctx, destID, baseCfg.RegistryUsername, baseCfg.RegistryPassword)
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
	fmt.Printf("\n  ✓ %d already synced\n  → %d to sync\n\n", len(synced), len(needsSync))
	if dryRun {
		for _, t := range synced {
			fmt.Printf("  ✓ %s (synced)\n", t.displayName())
		}
		for _, r := range results {
			if r.localExists && !force {
				continue
			}
			if r.localExists && !r.remoteExists {
				fmt.Printf("  → %s (in local, will sync to registry)\n", r.task.displayName())
			} else {
				fmt.Printf("  → %s (will sync)\n", r.task.displayName())
			}
		}
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

	p := progress.NewProgress(len(needsSync), workerCount)
	for i, t := range needsSync {
		p.AddImage(i, t.displayName())
	}

	// Create tasks for the worker pool
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

	// Execute with generic worker pool
	failCount, err := GenericWorkerPool(logger, syncTasks, processor, p, workerCount, verbose, ctx)
	if err != nil {
		return fmt.Errorf("%d image(s) failed to sync", failCount)
	}
	return nil
}
