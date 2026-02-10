# Progress Bar Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the custom progress bar in `pkg/progress/progress.go` with `vbauerster/mpb/v8` to fix cursor flickering in the batch command.

**Architecture:** Rewrite `Progress` struct to wrap an `mpb.Progress` container and `mpb.Bar`. Each running image gets a temporary spinner bar that appears on `StatusRunning` and is removed on completion/failure. The `BatchProgress` wrapper and `-v` flag are eliminated.

**Tech Stack:** Go 1.23, `vbauerster/mpb/v8`, `vbauerster/mpb/v8/decor`

---

### Task 1: Add mpb dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Install mpb/v8**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go get github.com/vbauerster/mpb/v8`
Expected: go.mod updated with `github.com/vbauerster/mpb/v8` in require block

**Step 2: Verify dependency is available**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go mod tidy`
Expected: Clean exit, no errors

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add vbauerster/mpb/v8 for progress bar rendering"
```

---

### Task 2: Rewrite `pkg/progress/progress.go` with mpb

**Files:**
- Rewrite: `pkg/progress/progress.go`

**Step 1: Write the new progress.go**

Replace the entire file content with:

```go
package progress

import (
	"fmt"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// ImageStatus represents the status of an image being processed
type ImageStatus int

const (
	StatusPending ImageStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
)

func (s ImageStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "syncing..."
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ImageProgress tracks the progress of a single image
type ImageProgress struct {
	Image   string
	Status  ImageStatus
	Error   error
	spinner *mpb.Bar
}

// Progress manages the progress display for multiple images
type Progress struct {
	container *mpb.Progress
	mainBar   *mpb.Bar
	images    []*ImageProgress
	total     int
	mu        sync.Mutex
}

// NewProgress creates a new progress tracker with an mpb container
func NewProgress(total int) *Progress {
	container := mpb.New(
		mpb.WithWidth(64),
	)

	mainBar := container.AddBar(int64(total),
		mpb.PrependDecorators(
			decor.Any(func(s decor.Statistics) string {
				return fmt.Sprintf("[%d/%d]", s.Current, total)
			}, decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
			decor.OnComplete(
				decor.Name("pulling", decor.WCSyncSpace),
				"done",
			),
		),
	)

	return &Progress{
		container: container,
		mainBar:   mainBar,
		images:    make([]*ImageProgress, total),
		total:     total,
	}
}

// AddImage adds an image to track
func (p *Progress) AddImage(index int, image string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.images[index] = &ImageProgress{
		Image:  image,
		Status: StatusPending,
	}
}

// UpdateStatus updates the status of an image
// When status changes to Running, a spinner bar is created.
// When status changes to Completed or Failed, the spinner bar is removed.
func (p *Progress) UpdateStatus(index int, status ImageStatus, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.images) || p.images[index] == nil {
		return
	}

	img := p.images[index]
	img.Status = status
	img.Error = err

	switch status {
	case StatusRunning:
		if img.spinner == nil {
			img.spinner = p.container.AddSpinner(0,
				mpb.PrependDecorators(
					decor.Name("  ◐ "+img.Image, decor.WCSyncSpace),
				),
				mpb.BarFillerOnComplete(""),
			)
		}
	case StatusCompleted, StatusFailed:
		if img.spinner != nil {
			img.spinner.Abort(true) // drop=true removes the bar from display
			img.spinner = nil
		}
	}
}

// Increment increments the main progress bar by 1
func (p *Progress) Increment() {
	p.mainBar.Increment()
}

// Wait waits for the mpb render loop to finish, then prints the summary
func (p *Progress) Wait() {
	p.container.Wait()
	p.printSummary()
}

// printSummary prints the final summary after all processing
func (p *Progress) printSummary() {
	p.mu.Lock()
	defer p.mu.Unlock()

	var succeeded, failed int
	var failures []string

	for _, img := range p.images {
		if img == nil {
			continue
		}
		switch img.Status {
		case StatusCompleted:
			succeeded++
		case StatusFailed:
			failed++
			msg := fmt.Sprintf("  ✗ %s", img.Image)
			if img.Error != nil {
				msg += fmt.Sprintf(": %v", img.Error)
			}
			failures = append(failures, msg)
		}
	}

	fmt.Printf("\nSummary: %d succeeded, %d failed\n", succeeded, failed)
	for _, f := range failures {
		fmt.Println(f)
	}
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go build ./pkg/progress/`
Expected: Clean exit, no errors

**Step 3: Commit**

```bash
git add pkg/progress/progress.go
git commit -m "refactor: rewrite progress bar using mpb/v8

Replace custom \r-based progress bar with mpb container.
Main bar shows overall progress, spinner bars show active images.
Remove BatchProgress wrapper, Display(), SetWidth(), StartWatching()."
```

---

### Task 3: Update `internal/cli/batch.go` to use new Progress API

**Files:**
- Modify: `internal/cli/batch.go`

**Step 1: Update batch.go**

The changes are:
1. Remove `showDetails` variable and its flag registration
2. Remove `showDetails` parameter from `processImagesWithProgress`
3. Rewrite `processImagesWithProgress` to use new `Progress` API directly (no `BatchProgress`)

Replace the full file with:

```go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gaodengpan/image-copier/internal/config"
	"github.com/gaodengpan/image-copier/internal/core"
	"github.com/gaodengpan/image-copier/pkg/progress"
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
```

**Step 2: Verify it compiles**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go build ./internal/cli/`
Expected: Clean exit, no errors

**Step 3: Commit**

```bash
git add internal/cli/batch.go
git commit -m "refactor: update batch command for new progress API

Remove -v/--verbose flag (unified display mode).
Use Progress directly instead of BatchProgress wrapper.
Worker pool logic now lives in batch.go for clarity."
```

---

### Task 4: Rewrite tests for new API

**Files:**
- Rewrite: `pkg/progress/progress_test.go`

**Step 1: Write new test file**

The key changes:
- `NewProgress` now takes 1 param (no `showDetails`)
- No `BatchProgress`, `Display()`, `SetWidth()`, `StartWatching()`
- Test `Increment()` instead of `IncrementProgress()`
- Test spinner bar creation/removal in `UpdateStatus`
- Keep: `ImageStatus.String()` tests, `AddImage` tests, concurrent safety tests, benchmarks

```go
package progress

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestImageStatus_String tests the ImageStatus String method
func TestImageStatus_String(t *testing.T) {
	tests := []struct {
		status   ImageStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "syncing..."},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{ImageStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("ImageStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNewProgress tests creating a new progress tracker
func TestNewProgress(t *testing.T) {
	p := NewProgress(10)

	if p == nil {
		t.Fatal("Expected non-nil progress tracker")
	}
	if p.total != 10 {
		t.Errorf("Expected total to be 10, got %d", p.total)
	}
	if p.container == nil {
		t.Error("Expected non-nil mpb container")
	}
	if p.mainBar == nil {
		t.Error("Expected non-nil main bar")
	}
	if len(p.images) != 10 {
		t.Errorf("Expected images slice length 10, got %d", len(p.images))
	}

	// Clean up mpb container
	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_AddImage tests adding images to track
func TestProgress_AddImage(t *testing.T) {
	p := NewProgress(3)

	p.AddImage(0, "nginx:latest")
	p.AddImage(1, "redis:alpine")
	p.AddImage(2, "postgres:15")

	if p.images[0] == nil || p.images[0].Image != "nginx:latest" {
		t.Error("Failed to add image at index 0")
	}
	if p.images[1] == nil || p.images[1].Image != "redis:alpine" {
		t.Error("Failed to add image at index 1")
	}
	if p.images[2] == nil || p.images[2].Image != "postgres:15" {
		t.Error("Failed to add image at index 2")
	}
	if p.images[0].Status != StatusPending {
		t.Errorf("Expected initial status to be pending, got %v", p.images[0].Status)
	}

	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_UpdateStatus tests updating image status
func TestProgress_UpdateStatus(t *testing.T) {
	p := NewProgress(2)
	p.AddImage(0, "image1")
	p.AddImage(1, "image2")

	// Test Running creates spinner
	p.UpdateStatus(0, StatusRunning, nil)
	if p.images[0].Status != StatusRunning {
		t.Errorf("Expected status to be running, got %v", p.images[0].Status)
	}
	if p.images[0].spinner == nil {
		t.Error("Expected spinner to be created on StatusRunning")
	}

	// Test Completed removes spinner
	p.UpdateStatus(0, StatusCompleted, nil)
	if p.images[0].Status != StatusCompleted {
		t.Errorf("Expected status to be completed, got %v", p.images[0].Status)
	}
	if p.images[0].spinner != nil {
		t.Error("Expected spinner to be removed on StatusCompleted")
	}

	// Test Failed with error
	testErr := errors.New("test error")
	p.UpdateStatus(1, StatusRunning, nil)
	p.UpdateStatus(1, StatusFailed, testErr)
	if p.images[1].Status != StatusFailed {
		t.Errorf("Expected status to be failed, got %v", p.images[1].Status)
	}
	if p.images[1].Error == nil || p.images[1].Error.Error() != "test error" {
		t.Errorf("Expected error to be set, got %v", p.images[1].Error)
	}
	if p.images[1].spinner != nil {
		t.Error("Expected spinner to be removed on StatusFailed")
	}

	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_UpdateStatus_OutOfBounds tests bounds checking
func TestProgress_UpdateStatus_OutOfBounds(t *testing.T) {
	p := NewProgress(2)
	p.AddImage(0, "image1")

	// Should not panic on out-of-bounds
	p.UpdateStatus(5, StatusRunning, nil)
	// Should not panic on nil image
	p.UpdateStatus(1, StatusRunning, nil)

	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_Increment tests progress increment
func TestProgress_Increment(t *testing.T) {
	p := NewProgress(5)

	// Increment and check mainBar current
	p.Increment()
	p.Increment()

	current := p.mainBar.Current()
	if current != 2 {
		t.Errorf("Expected mainBar current to be 2, got %d", current)
	}

	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_ConcurrentUpdates tests concurrent status updates
func TestProgress_ConcurrentUpdates(t *testing.T) {
	const n = 50
	p := NewProgress(n)

	for i := 0; i < n; i++ {
		p.AddImage(i, "image")
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p.UpdateStatus(idx, StatusRunning, nil)
			p.UpdateStatus(idx, StatusCompleted, nil)
			p.Increment()
		}(i)
	}

	wg.Wait()

	current := p.mainBar.Current()
	if current != int64(n) {
		t.Errorf("Expected all %d images to be processed, got %d", n, current)
	}

	p.mainBar.Abort(true)
	p.container.Wait()
}

// TestProgress_FullWorkflow tests the complete lifecycle
func TestProgress_FullWorkflow(t *testing.T) {
	images := []string{"nginx:latest", "redis:alpine", "postgres:15"}
	p := NewProgress(len(images))

	for i, img := range images {
		p.AddImage(i, img)
	}

	// Simulate worker processing
	var failCount atomic.Int32
	var wg sync.WaitGroup

	for i := range images {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p.UpdateStatus(idx, StatusRunning, nil)

			// Simulate: index 1 fails
			if idx == 1 {
				p.UpdateStatus(idx, StatusFailed, errors.New("connection refused"))
				failCount.Add(1)
			} else {
				p.UpdateStatus(idx, StatusCompleted, nil)
			}
			p.Increment()
		}(i)
	}

	wg.Wait()
	p.Wait()

	if failCount.Load() != 1 {
		t.Errorf("Expected 1 failure, got %d", failCount.Load())
	}
}

// TestProgress_ZeroImages tests progress with zero images
func TestProgress_ZeroImages(t *testing.T) {
	p := NewProgress(0)

	if p.total != 0 {
		t.Errorf("Expected total to be 0, got %d", p.total)
	}

	// Should complete without issues
	p.Wait()
}

// BenchmarkProgress_Increment benchmarks increment operations
func BenchmarkProgress_Increment(b *testing.B) {
	p := NewProgress(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Increment()
	}
	p.mainBar.Abort(true)
	p.container.Wait()
}

// BenchmarkProgress_UpdateStatus benchmarks status updates
func BenchmarkProgress_UpdateStatus(b *testing.B) {
	const size = 1000
	p := NewProgress(size)
	for i := 0; i < size; i++ {
		p.AddImage(i, "image")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % size
		p.UpdateStatus(idx, StatusRunning, nil)
		p.UpdateStatus(idx, StatusCompleted, nil)
	}

	p.mainBar.Abort(true)
	p.container.Wait()
}
```

**Step 2: Run the tests**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go test ./pkg/progress/ -v -count=1`
Expected: All tests pass

**Step 3: Run the benchmarks**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go test ./pkg/progress/ -bench=. -benchmem -count=1`
Expected: Benchmarks complete without errors

**Step 4: Commit**

```bash
git add pkg/progress/progress_test.go
git commit -m "test: rewrite progress tests for mpb-based API

Update all tests for new Progress API (no BatchProgress, no showDetails).
Add tests for spinner bar creation/removal lifecycle.
Keep concurrent safety tests and benchmarks."
```

---

### Task 5: Full build and integration verification

**Step 1: Run all project tests**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go test ./... -count=1`
Expected: All tests pass

**Step 2: Build the binary**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && go build -o image-copier .`
Expected: Binary compiles successfully

**Step 3: Verify batch command help (no -v flag)**

Run: `cd /Users/diejia/Documents/privy/sourceCode/image-copier && ./image-copier batch --help`
Expected: Help output shows `-f/--file` and `-j/--jobs` flags but NOT `-v/--verbose`

**Step 4: Clean up binary**

Run: `rm /Users/diejia/Documents/privy/sourceCode/image-copier/image-copier`

**Step 5: Commit (if any go.sum changes from tidy)**

```bash
go mod tidy
git add go.mod go.sum
git commit -m "chore: tidy go modules"
```
