package progress

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
)

// StageInfo holds the display information for a worker's current stage.
type StageInfo struct {
	Label     string    // 镜像名称
	StageName string    // 阶段显示名称，如 "workflow running"
	Percent   float64   // 子进度百分比 [0, 100]
	StartAt   time.Time // 镜像开始处理时间
}

// SyncStage 定义 sync 命令的阶段
type SyncStage int

const (
	SyncStageChecking     SyncStage = iota // 检查镜像存在性
	SyncStageSyncing                       // 同步到中转仓库
	SyncStageDistributing                  // 分发到目标
)

// String 返回阶段的显示名称
func (s SyncStage) String() string {
	switch s {
	case SyncStageChecking:
		return "checking"
	case SyncStageSyncing:
		return "sync"
	case SyncStageDistributing:
		return "dist"
	default:
		return "unknown"
	}
}

// FormatStageWithTarget 格式化阶段名称，用于分发阶段显示目标名
func FormatStageWithTarget(stage SyncStage, targetName string) string {
	if stage == SyncStageDistributing && targetName != "" {
		return "dist → " + targetName
	}
	return stage.String()
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
	container   *mpb.Progress
	mainBar     *mpb.Bar
	workerBars  []*mpb.Bar
	workerTexts []*atomic.Value
	images      []*ImageProgress
	total       int
	startedAt   time.Time
	mu          sync.Mutex
	noOutput    bool
	operation   string // 操作名称 (pulling/syncing)
}

// isTerminal checks if stdout is connected to a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// NewProgress creates a new progress tracker with an mpb container.
// workerCount determines how many worker status lines are displayed below the main bar.
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
	if noOutput {
		p := &Progress{
			images:    make([]*ImageProgress, total),
			total:     total,
			startedAt: time.Now(),
			noOutput:  true,
			operation: operation,
		}
		for i := range total {
			p.images[i] = &ImageProgress{}
		}
		return p
	}

	container := mpb.New(
		mpb.WithWidth(64),
	)

	op := operation // capture for closure
	mainBar := container.AddBar(int64(total),
		mpb.PrependDecorators(
			decor.Any(func(s decor.Statistics) string {
				return fmt.Sprintf("[%d/%d]", s.Current, total)
			}, decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
			decor.OnComplete(
				decor.Name(op, decor.WCSyncSpace),
				"done",
			),
		),
	)

	workerTexts := make([]*atomic.Value, workerCount)
	workerBars := make([]*mpb.Bar, workerCount)

	for i := range workerCount {
		workerTexts[i] = &atomic.Value{}
		workerTexts[i].Store(StageInfo{})

		idx := i
		workerBars[i] = container.New(0,
			mpb.NopStyle(),
			mpb.PrependDecorators(
				decor.Any(func(s decor.Statistics) string {
					raw := workerTexts[idx].Load()
					info, ok := raw.(StageInfo)
					if !ok || info.Label == "" {
						return ""
					}
					if info.StageName == "" {
						return "  ◐ " + smartTruncate(stripHash(info.Label), 50)
					}
					elapsed := time.Since(info.StartAt).Truncate(time.Second)
					return fmt.Sprintf("  ◐ %-50s [%3.0f%%] %s (%s)",
						smartTruncate(stripHash(info.Label), 50), info.Percent, info.StageName, elapsed)
				}),
			),
		)
	}

	p := &Progress{
		container:   container,
		mainBar:     mainBar,
		workerBars:  workerBars,
		workerTexts: workerTexts,
		images:      make([]*ImageProgress, total),
		total:       total,
		startedAt:   time.Now(),
		noOutput:    false,
		operation:   op,
	}

	if total == 0 {
		mainBar.Abort(true)
		for _, bar := range workerBars {
			bar.Abort(true)
		}
	}

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
// Pass an empty string to clear the worker line (when idle between images).
func (p *Progress) UpdateWorker(workerIdx int, imageName string) {
	if p.noOutput || workerIdx >= len(p.workerTexts) {
		return
	}
	if imageName == "" {
		p.workerTexts[workerIdx].Store(StageInfo{})
	} else {
		p.workerTexts[workerIdx].Store(StageInfo{Label: imageName, StartAt: time.Now()})
	}
}

// UpdateStage sets the stage display info for a worker.
func (p *Progress) UpdateStage(workerIdx int, info StageInfo) {
	if p.noOutput || workerIdx >= len(p.workerTexts) {
		return
	}
	p.workerTexts[workerIdx].Store(info)
}

// UpdateSyncStage 更新 sync 命令的工作阶段显示
// stage: 当前阶段 (checking/sync/dist)
// targetName: 分发目标名称（仅在 dist 阶段使用）
// percent: 阶段内进度百分比 [0, 100]
func (p *Progress) UpdateSyncStage(workerIdx int, imageName string, stage SyncStage, targetName string, percent float64) {
	if p.noOutput || workerIdx >= len(p.workerTexts) {
		return
	}
	stageName := FormatStageWithTarget(stage, targetName)
	p.workerTexts[workerIdx].Store(StageInfo{
		Label:     imageName,
		StageName: stageName,
		Percent:   percent,
		StartAt:   time.Now(),
	})
}

// ClearWorker 清除指定 worker 的显示
func (p *Progress) ClearWorker(workerIdx int) {
	if p.noOutput || workerIdx >= len(p.workerTexts) {
		return
	}
	p.workerTexts[workerIdx].Store(StageInfo{})
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
func (p *Progress) Increment() {
	if p.noOutput {
		return
	}
	p.mainBar.Increment()
}

// SetInitialProgress sets the initial completed count (for already-skipped items)
func (p *Progress) SetInitialProgress(count int) {
	if p.noOutput {
		return
	}
	p.mainBar.SetCurrent(int64(count))
}

// CompleteSkipped marks all images from index start as completed (for skipped items)
func (p *Progress) CompleteSkipped(start int) {
	if p.noOutput {
		return
	}
	for i := start; i < len(p.images); i++ {
		if p.images[i] != nil {
			p.mainBar.Increment()
		}
	}
}

// AbortWorkers aborts all worker bars without printing summary
func (p *Progress) AbortWorkers() {
	if p.noOutput {
		return
	}
	// Abort mainBar as well to prevent deadlock
	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
}

// WaitContainer waits for the mpb render loop to finish
func (p *Progress) WaitContainer() {
	if p.noOutput {
		return
	}
	p.container.Wait()
}

// Wait waits for the mpb render loop to finish, then prints the summary
func (p *Progress) Wait() {
	if p.noOutput {
		p.printSummary()
		return
	}
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
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

	if dryRun > 0 {
		fmt.Printf("\nSummary: %d succeeded, %d skipped, %d dry-run, %d failed | Total: %s\n",
			succeeded, skipped, dryRun, totalFailed, formatDuration(totalDuration))
	} else {
		fmt.Printf("\nSummary: %d succeeded, %d skipped, %d failed | Total: %s\n",
			succeeded, skipped, totalFailed, formatDuration(totalDuration))
	}

	for _, img := range p.images {
		if img == nil {
			continue
		}
		dur := formatDuration(img.Duration)
		switch img.Status {
		case StatusCompleted:
			fmt.Printf("  ✓ %s (%s)\n", img.Image, dur)
		case StatusSkipped:
			fmt.Printf("  ◦ %s (%s)\n", img.Image, dur)
		case StatusDryRun:
			fmt.Printf("  ~ %s (%s)\n", img.Image, dur)
		case StatusFailed:
			msg := fmt.Sprintf("  ✗ %s (%s)", img.Image, dur)
			if img.Error != nil {
				msg += fmt.Sprintf(": %v", img.Error)
			}
			fmt.Println(msg)
		case StatusCancelled:
			msg := fmt.Sprintf("  ⊘ %s (%s)", img.Image, dur)
			if img.Error != nil {
				msg += fmt.Sprintf(": %v", img.Error)
			}
			fmt.Println(msg)
		}
	}
}

// LogWriter returns an io.Writer that outputs text above the progress bars.
// This leverages mpb's built-in io.Writer support on the Progress container.
func (p *Progress) LogWriter() io.Writer {
	if p.noOutput {
		return io.Discard
	}
	return p.container
}
