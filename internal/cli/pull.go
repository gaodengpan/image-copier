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
		filePath    string
		workerCount int
		force       bool
		dryRun      bool
		verbose     bool
	)

	cmd := &cobra.Command{
		Use:   "pull [IMAGE...]",
		Short: "Pull images through GitHub Actions",
		Long:  `Pull one or more images by routing them through GitHub Actions when direct pulling is not possible.

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
				baseCfg := &core.Config{
					GithubOwner:      cfg.Github.Owner,
					GithubRepo:       cfg.Github.Repo,
					GithubToken:      cfg.Github.Token,
					GithubWorkflowID: cfg.Github.WorkflowID,
					RegistryHost:     cfg.Registry.Host,
					RegistryUsername:  cfg.Registry.Username,
					RegistryPassword:  cfg.Registry.Password,
					RegistryNamespace: cfg.Registry.Namespace,
					RetryConfig:      cfg.ParseRetryConfig(),
				}
				ctx := context.Background()
				return processSyncTasks(logger, baseCfg, tasks, workerCount, force, dryRun, verbose, ctx)
			}

			// === 命令式模式（CLI 参数）===
			pullerCfg := &core.Config{
				GithubOwner:      cfg.Github.Owner,
				GithubRepo:       cfg.Github.Repo,
				GithubToken:      cfg.Github.Token,
				GithubWorkflowID: cfg.Github.WorkflowID,
				RegistryHost:     cfg.Registry.Host,
				RegistryUsername:  cfg.Registry.Username,
				RegistryPassword:  cfg.Registry.Password,
				RegistryNamespace: cfg.Registry.Namespace,
				RegistryArch:     arch,
				RegistryOs:       osType,
				Force:            force,
				RetryConfig:      cfg.ParseRetryConfig(),
				DryRun:           dryRun,
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
					} else if errors.Is(err, core.ErrDryRun) {
						p.UpdateStatus(idx, progress.StatusDryRun, nil)
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
		task   syncTask
		exists bool
	}
	results := make([]diffResult, len(tasks))

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
			destID := core.BuildDestImageID(baseCfg.RegistryHost, baseCfg.RegistryNamespace, sourceID)
			exists, _ := core.CheckImageExists(destID, baseCfg.RegistryUsername, baseCfg.RegistryPassword)
			results[idx] = diffResult{task: task, exists: exists}
		}(i, t)
	}
	wg.Wait()

	// Partition: synced vs needsSync
	var synced, needsSync []syncTask
	for _, r := range results {
		if r.exists && !force {
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
		for _, t := range needsSync {
			fmt.Printf("  → %s (will sync)\n", t.displayName())
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

	if verbose {
		logger.SetOutput(p.LogWriter())
	} else {
		logger.SetOutput(io.Discard)
	}
	defer logger.SetOutput(os.Stderr)

	jobs := make(chan int, len(needsSync))
	for i := range needsSync {
		jobs <- i
	}
	close(jobs)

	var syncWg sync.WaitGroup
	var failCount atomic.Int32

	for i := 0; i < workerCount; i++ {
		workerIdx := i
		syncWg.Add(1)
		go func() {
			defer syncWg.Done()
			for idx := range jobs {
				task := needsSync[idx]
				p.UpdateStatus(idx, progress.StatusRunning, nil)
				startTime := time.Now()

				// Create per-task Config with specific arch/os
				taskCfg := *baseCfg
				taskCfg.RegistryArch = task.Arch
				taskCfg.RegistryOs = task.Os
				taskCfg.Force = true // diff already confirmed sync needed

				puller := core.NewPuller(&taskCfg, logger)
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
						Label:     task.displayName(),
						StageName: stageNames[stageIdx],
						Percent:   pct,
						StartAt:   startTime,
					})
				}

				err := puller.PullSingle(ctx, task.Source)
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

	syncWg.Wait()
	p.Wait()

	if failCount.Load() > 0 {
		return fmt.Errorf("%d image(s) failed to sync", failCount.Load())
	}
	return nil
}
