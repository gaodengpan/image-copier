package progress

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"golang.org/x/term"
)

// StageInfo holds the display information for a worker's current stage.
type StageInfo struct {
	Label     string                  // 镜像名称
	StageName string                  // 阶段显示名称，如 "workflow running"
	Percent   float64                 // 子进度百分比 [0, 100]
	StartAt   time.Time               // 镜像开始处理时间
	Stage     value_objects.SyncStage // 当前阶段
}

// FormatStageWithTarget 格式化阶段名称，用于分发阶段显示目标名
func FormatStageWithTarget(stage value_objects.SyncStage, targetName string) string {
	if stage == value_objects.SyncStageDistributing && targetName != "" {
		return "dist → " + targetName
	}
	return stage.String()
}

// mapStageToDisplay 将 SyncStage 映射为显示名称
func mapStageToDisplay(stage value_objects.SyncStage) string {
	switch stage {
	case value_objects.SyncStageChecking:
		return "Checking"
	case value_objects.SyncStageSyncing:
		return "Downloading"
	case value_objects.SyncStageDistributing:
		return "Uploading"
	default:
		return stage.String()
	}
}

// ImageStatus represents the status of an image being processed
type ImageStatus int

const (
	StatusPending ImageStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
	StatusDryRun
	StatusCancelled
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
	case StatusSkipped:
		return "skipped"
	case StatusDryRun:
		return "dry-run"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// ImageProgress tracks the progress of a single image
type ImageProgress struct {
	Image    string
	Status   ImageStatus
	Error    error
	Duration time.Duration
}

// Progress manages the progress display for multiple images
type Progress struct {
	homebrew  *HomebrewProgress
	images    []*ImageProgress
	total     int
	startedAt time.Time
	mu        sync.Mutex
	noOutput  bool
	operation string // 操作名称 (pulling/syncing)
}

// isTerminal checks if stdout is connected to a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// NewProgress creates a new progress tracker with Homebrew-style display.
// workerCount determines how many worker status lines are displayed.
// If noOutput is true, no UI is displayed (useful for JSON output mode).
// operation is the action name shown in the progress bar (e.g., "pulling", "syncing").
func NewProgress(total int, workerCount int, noOutput bool, operation string) *Progress {
	if operation == "" {
		operation = "syncing"
	}
	// Disable progress bar if not a terminal (to avoid repeated line output)
	if !noOutput && !isTerminal() {
		noOutput = true
	}

	p := &Progress{
		images:    make([]*ImageProgress, total),
		total:     total,
		startedAt: time.Now(),
		noOutput:  noOutput,
		operation: operation,
	}

	for i := range total {
		p.images[i] = &ImageProgress{}
	}

	// Create HomebrewProgress with total images
	p.homebrew = NewHomebrewProgress(total, noOutput)
	p.homebrew.Start()

	return p
}

// AddImage adds an image to track
func (p *Progress) AddImage(index int, image string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.images[index] = &ImageProgress{
		Image:  image,
		Status: StatusPending,
	}

	// Also set image name in HomebrewProgress
	if p.homebrew != nil {
		p.homebrew.SetTaskImage(index, image)
	}
}

// UpdateStatus updates the status of an image for summary tracking
func (p *Progress) UpdateStatus(index int, status ImageStatus, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.images) || p.images[index] == nil {
		return
	}

	p.images[index].Status = status
	p.images[index].Error = err

	// Update HomebrewProgress based on status
	if p.homebrew != nil {
		switch status {
		case StatusCompleted:
			p.homebrew.CompleteTask(index)
		case StatusFailed:
			p.homebrew.FailTask(index, err)
		case StatusSkipped:
			p.homebrew.SkipTask(index)
		}
	}
}

// SetDuration records how long a specific image took to process.
func (p *Progress) SetDuration(index int, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.images) || p.images[index] == nil {
		return
	}
	p.images[index].Duration = d
}

// UpdateWorker sets the display text for a specific worker line.
// This is now a no-op since HomebrewProgress shows all tasks.
func (p *Progress) UpdateWorker(workerIdx int, imageName string) {
	// No-op: HomebrewProgress shows all tasks, not workers
}

// UpdateStage sets the stage display info for a worker.
// This is now a no-op since HomebrewProgress uses UpdateSyncStage instead.
func (p *Progress) UpdateStage(workerIdx int, info StageInfo) {
	// No-op: HomebrewProgress uses UpdateSyncStage instead
}

// UpdateSyncStage 更新 sync 命令的工作阶段显示
// workerIdx: 任务索引
// stage: 当前阶段 (checking/sync/dist)
// targetName: 分发目标名称（仅在 dist 阶段使用）
// percent: 未使用（保持接口兼容）
func (p *Progress) UpdateSyncStage(workerIdx int, _ string, stage value_objects.SyncStage, targetName string, _ float64) {
	if p.noOutput || p.homebrew == nil {
		return
	}

	// Map stage to display name
	stageName := mapStageToDisplay(stage)
	if stage == value_objects.SyncStageDistributing && targetName != "" {
		stageName = "Uploading → " + targetName
	}

	p.homebrew.UpdateTask(workerIdx, TaskRunning, stageName, nil)
}

// ClearWorker 清除指定 worker 的显示
// This is now a no-op since HomebrewProgress manages task lines directly.
func (p *Progress) ClearWorker(workerIdx int) {
	// No-op: HomebrewProgress manages task lines directly
}

// IsNoOutput returns true if progress output is disabled
func (p *Progress) IsNoOutput() bool {
	return p.noOutput
}

// GetImages returns a copy of all images with their status.
// This is useful for building result summaries after processing completes.
func (p *Progress) GetImages() []*ImageProgress {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]*ImageProgress, len(p.images))
	copy(result, p.images)
	return result
}

// hashSuffixRe matches a hex digest suffix (20+ hex chars) at the end of
// the image path component, e.g. "controller-10a3e32792f33651396d02b6855a6e36".
var hashSuffixRe = regexp.MustCompile(`-[0-9a-f]{20,}$`)

// stripHash removes hash digest suffixes from image names for display purposes.
// For example "ghcr.io/tektoncd/pipeline/controller-10a3e327...:v1.1.0" becomes
// "ghcr.io/tektoncd/pipeline/controller:v1.1.0". Names without hash suffixes
// are returned unchanged.
func stripHash(image string) string {
	ref := image
	tag := ""
	if i := strings.LastIndex(ref, ":"); i != -1 {
		tag = ref[i:]
		ref = ref[:i]
	}
	ref = hashSuffixRe.ReplaceAllString(ref, "")
	return ref + tag
}

// smartTruncate truncates s to maxLen by keeping both head and tail,
// joining them with "...". The tail receives ~60% of the available space
// so that image names and tags (which appear at the end) remain visible.
func smartTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	const ellipsis = "..."
	const ellipsisLen = 3
	if maxLen <= ellipsisLen {
		return s[:maxLen]
	}
	available := maxLen - ellipsisLen
	tailLen := available * 6 / 10
	headLen := available - tailLen
	return s[:headLen] + ellipsis + s[len(s)-tailLen:]
}

// Increment increments the main progress bar by 1
// This is now a no-op since HomebrewProgress tracks tasks individually.
func (p *Progress) Increment() {
	// No-op: HomebrewProgress tracks tasks individually
}

// SetInitialProgress sets the initial completed count (for already-skipped items)
// This is now a no-op since HomebrewProgress tracks tasks individually.
func (p *Progress) SetInitialProgress(count int) {
	// No-op: HomebrewProgress tracks tasks individually
}

// CompleteSkipped marks all images from index start as completed (for skipped items)
func (p *Progress) CompleteSkipped(start int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := start; i < len(p.images); i++ {
		if p.images[i] != nil {
			p.images[i].Status = StatusSkipped
			if p.homebrew != nil {
				p.homebrew.SkipTask(i)
			}
		}
	}
}

// AbortWorkers aborts all worker bars without printing summary
func (p *Progress) AbortWorkers() {
	if p.noOutput || p.homebrew == nil {
		return
	}
	p.homebrew.Stop()
}

// WaitContainer stops the progress display without printing summary.
// Kept for backward compatibility with existing callers.
func (p *Progress) WaitContainer() {
	if p.noOutput || p.homebrew == nil {
		return
	}
	p.homebrew.Stop()
}

// Wait waits for the display to finish, then prints the summary
func (p *Progress) Wait() {
	// Always stop homebrew to render final state
	if p.homebrew != nil {
		p.homebrew.Stop()
	}
	p.printSummary()
}

// formatDuration formats a duration into a human-readable string like "1m12s" or "5s".
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}

// printSummary prints the final summary after all processing
func (p *Progress) printSummary() {
	// Skip summary output in noOutput mode (e.g., JSON output)
	if p.noOutput {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	totalDuration := time.Since(p.startedAt)

	var succeeded, failed, skipped, dryRun, cancelled int

	for _, img := range p.images {
		if img == nil {
			continue
		}
		switch img.Status {
		case StatusCompleted:
			succeeded++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		case StatusDryRun:
			dryRun++
		case StatusCancelled:
			cancelled++
		}
	}

	totalFailed := failed + cancelled

	fmt.Printf("\n==> Summary\n")
	if totalFailed > 0 {
		fmt.Printf("✓ %d succeeded, %d skipped, %d failed in %s\n",
			succeeded, skipped, totalFailed, formatDuration(totalDuration))
	} else {
		fmt.Printf("✓ %d succeeded, %d skipped in %s\n",
			succeeded, skipped, formatDuration(totalDuration))
	}
}

// LogWriter returns an io.Writer for logging output.
// Returns os.Stdout or io.Discard based on noOutput setting.
func (p *Progress) LogWriter() io.Writer {
	if p.noOutput {
		return io.Discard
	}
	return os.Stdout
}
