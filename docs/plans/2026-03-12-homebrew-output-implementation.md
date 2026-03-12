# Homebrew 风格输出实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 sync 命令的输出改为 Homebrew 风格的简洁 spinner 动画显示。

**Architecture:** 创建新的 HomebrewProgress 组件，使用纯标准库实现 Braille spinner 动画和多行列表显示，移除 mpb 依赖。

**Tech Stack:** Go 标准库（sync, io, time）+ ANSI 转义序列

---

## Task 1: 创建 HomebrewProgress 核心结构

**Files:**
- Create: `pkg/progress/homebrew.go`

**Step 1: 写测试**

创建 `pkg/progress/homebrew_test.go`:

```go
package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHomebrewProgress(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		noOutput  bool
		wantNoOut bool
	}{
		{"with output", 5, false, false},
		{"no output mode", 5, true, true},
		{"zero total", 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewHomebrewProgress(tt.total, tt.noOutput)
			assert.NotNil(t, p)
			assert.Equal(t, tt.wantNoOut, p.IsNoOutput())
		})
	}
}

func TestTaskStatus_String(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskPending, "pending"},
		{TaskRunning, "running"},
		{TaskCompleted, "completed"},
		{TaskFailed, "failed"},
		{TaskSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}
```

**Step 2: 运行测试确认失败**

```bash
go test -v ./pkg/progress -run TestNewHomebrewProgress
go test -v ./pkg/progress -run TestTaskStatus
```
Expected: FAIL - undefined types

**Step 3: 实现核心结构**

创建 `pkg/progress/homebrew.go`:

```go
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Braille spinner frames (same as Homebrew)
var brailleSpinner = []rune{'⣾', '⣷', '⣯', '⣟', '⡿', '⢿', '⣻', '⣽'}

// TaskStatus represents the status of a task
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
	TaskSkipped
)

func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskRunning:
		return "running"
	case TaskCompleted:
		return "completed"
	case TaskFailed:
		return "failed"
	case TaskSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// TaskLine represents a single task line in the display
type TaskLine struct {
	ImageName string
	Stage     string      // "Checking", "Downloading", "Uploading"
	Status    TaskStatus
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

// HomebrewProgress implements Homebrew-style progress display
type HomebrewProgress struct {
	mu         sync.Mutex
	tasks      []*TaskLine
	spinnerIdx int
	interval   time.Duration
	output     io.Writer
	isTerminal bool
	noOutput   bool
	startTime  time.Time

	// For animation control
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewHomebrewProgress creates a new Homebrew-style progress display
func NewHomebrewProgress(total int, noOutput bool) *HomebrewProgress {
	isTerm := term.IsTerminal(int(os.Stdout.Fd()))

	// Disable animation in non-TTY or noOutput mode
	if noOutput || !isTerm {
		return &HomebrewProgress{
			tasks:     make([]*TaskLine, total),
			noOutput:  true,
			startTime: time.Now(),
		}
	}

	p := &HomebrewProgress{
		tasks:      make([]*TaskLine, total),
		interval:   100 * time.Millisecond,
		output:     os.Stdout,
		isTerminal: true,
		noOutput:   false,
		startTime:  time.Now(),
		stopChan:   make(chan struct{}),
	}

	for i := range total {
		p.tasks[i] = &TaskLine{Status: TaskPending}
	}

	return p
}

// IsNoOutput returns true if output is disabled
func (p *HomebrewProgress) IsNoOutput() bool {
	return p.noOutput
}
```

**Step 4: 运行测试确认通过**

```bash
go test -v ./pkg/progress -run "TestNewHomebrewProgress|TestTaskStatus"
```
Expected: PASS

**Step 5: 提交**

```bash
git add pkg/progress/homebrew.go pkg/progress/homebrew_test.go
git commit -m "feat(progress): add HomebrewProgress core structure"
```

---

## Task 2: 实现任务管理方法

**Files:**
- Modify: `pkg/progress/homebrew.go`
- Modify: `pkg/progress/homebrew_test.go`

**Step 1: 写测试**

添加到 `pkg/progress/homebrew_test.go`:

```go
func TestHomebrewProgress_AddTask(t *testing.T) {
	p := NewHomebrewProgress(3, true)

	idx := p.AddTask("nginx:latest")
	assert.Equal(t, 0, idx)

	idx = p.AddTask("redis:alpine")
	assert.Equal(t, 1, idx)

	task := p.GetTask(0)
	assert.NotNil(t, task)
	assert.Equal(t, "nginx:latest", task.ImageName)
	assert.Equal(t, TaskPending, task.Status)
}

func TestHomebrewProgress_UpdateStage(t *testing.T) {
	p := NewHomebrewProgress(1, true)
	p.AddTask("nginx:latest")

	p.UpdateStage(0, "Downloading")

	task := p.GetTask(0)
	assert.Equal(t, "Downloading", task.Stage)
	assert.Equal(t, TaskRunning, task.Status)
}

func TestHomebrewProgress_CompleteTask(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status TaskStatus
	}{
		{"success", nil, TaskCompleted},
		{"failure", fmt.Errorf("timeout"), TaskFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewHomebrewProgress(1, true)
			p.AddTask("nginx:latest")
			p.UpdateStage(0, "Downloading")

			p.CompleteTask(0, tt.err)

			task := p.GetTask(0)
			assert.Equal(t, tt.status, task.Status)
			assert.Equal(t, tt.err, task.Error)
			assert.False(t, task.EndTime.IsZero())
		})
	}
}
```

**Step 2: 运行测试确认失败**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_AddTask|TestHomebrewProgress_UpdateStage|TestHomebrewProgress_CompleteTask"
```
Expected: FAIL - methods not defined

**Step 3: 实现任务管理方法**

添加到 `pkg/progress/homebrew.go`:

```go
// AddTask adds a new task and returns its index
func (p *HomebrewProgress) AddTask(imageName string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, task := range p.tasks {
		if task == nil {
			p.tasks[i] = &TaskLine{
				ImageName: imageName,
				Status:    TaskPending,
				StartTime: time.Now(),
			}
			return i
		}
		if task.Status == TaskPending && task.ImageName == "" {
			task.ImageName = imageName
			task.StartTime = time.Now()
			return i
		}
	}

	// Extend if needed
	idx := len(p.tasks)
	p.tasks = append(p.tasks, &TaskLine{
		ImageName: imageName,
		Status:    TaskPending,
		StartTime: time.Now(),
	})
	return idx
}

// UpdateStage updates the stage of a task
func (p *HomebrewProgress) UpdateStage(index int, stage string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) && p.tasks[index] != nil {
		p.tasks[index].Stage = stage
		p.tasks[index].Status = TaskRunning
	}
}

// CompleteTask marks a task as completed or failed
func (p *HomebrewProgress) CompleteTask(index int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) && p.tasks[index] != nil {
		p.tasks[index].EndTime = time.Now()
		p.tasks[index].Error = err
		if err != nil {
			p.tasks[index].Status = TaskFailed
		} else {
			p.tasks[index].Status = TaskCompleted
		}
	}
}

// GetTask returns the task at the given index (for testing)
func (p *HomebrewProgress) GetTask(index int) *TaskLine {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) {
		return p.tasks[index]
	}
	return nil
}
```

**Step 4: 运行测试确认通过**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_"
```
Expected: PASS

**Step 5: 提交**

```bash
git add pkg/progress/homebrew.go pkg/progress/homebrew_test.go
git commit -m "feat(progress): add task management methods to HomebrewProgress"
```

---

## Task 3: 实现渲染引擎

**Files:**
- Modify: `pkg/progress/homebrew.go`
- Modify: `pkg/progress/homebrew_test.go`

**Step 1: 写测试**

添加到 `pkg/progress/homebrew_test.go`:

```go
func TestHomebrewProgress_RenderLine(t *testing.T) {
	tests := []struct {
		name     string
		task     TaskLine
		expected string
	}{
		{
			name: "running task",
			task: TaskLine{
				ImageName: "nginx:latest",
				Stage:     "Downloading",
				Status:    TaskRunning,
			},
			expected: "⣾ Downloading nginx:latest...",
		},
		{
			name: "completed task",
			task: TaskLine{
				ImageName: "nginx:latest",
				Status:    TaskCompleted,
				StartTime: time.Now().Add(-5 * time.Second),
				EndTime:   time.Now(),
			},
			expected: "✓ nginx:latest (5s)",
		},
		{
			name: "failed task",
			task: TaskLine{
				ImageName: "nginx:latest",
				Status:    TaskFailed,
				Error:     fmt.Errorf("timeout"),
			},
			expected: "✗ nginx:latest: timeout",
		},
		{
			name: "skipped task",
			task: TaskLine{
				ImageName: "nginx:latest",
				Status:    TaskSkipped,
			},
			expected: "◦ nginx:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewHomebrewProgress(1, true)
			line := p.renderLine(&tt.task, 0)
			assert.Contains(t, line, tt.expected[:3]) // Check symbol
		})
	}
}

func TestHomebrewProgress_FormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{500 * time.Millisecond, "<1s"},
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m30s"},
		{2 * time.Minute, "2m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDurationHomebrew(tt.d)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

**Step 2: 运行测试确认失败**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_RenderLine|TestHomebrewProgress_FormatDuration"
```
Expected: FAIL - methods not defined

**Step 3: 实现渲染方法**

添加到 `pkg/progress/homebrew.go`:

```go
// formatDurationHomebrew formats duration like Homebrew
func formatDurationHomebrew(d time.Duration) string {
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

// renderLine renders a single task line
func (p *HomebrewProgress) renderLine(task *TaskLine, spinnerIdx int) string {
	if task == nil || task.ImageName == "" {
		return ""
	}

	switch task.Status {
	case TaskRunning:
		spinner := brailleSpinner[spinnerIdx%len(brailleSpinner)]
		return fmt.Sprintf("%c %s %s...", spinner, task.Stage, task.ImageName)
	case TaskCompleted:
		dur := formatDurationHomebrew(time.Since(task.StartTime))
		return fmt.Sprintf("✓ %s (%s)", task.ImageName, dur)
	case TaskFailed:
		dur := formatDurationHomebrew(time.Since(task.StartTime))
		if task.Error != nil {
			return fmt.Sprintf("✗ %s (%s): %v", task.ImageName, dur, task.Error)
		}
		return fmt.Sprintf("✗ %s (%s)", task.ImageName, dur)
	case TaskSkipped:
		return fmt.Sprintf("◦ %s", task.ImageName)
	default:
		return ""
	}
}

// Start begins the render loop
func (p *HomebrewProgress) Start() {
	if p.noOutput {
		return
	}

	p.wg.Add(1)
	go p.renderLoop()
}

// Stop stops the render loop
func (p *HomebrewProgress) Stop() {
	if p.noOutput {
		return
	}

	close(p.stopChan)
	p.wg.Wait()
}

// renderLoop is the main animation loop
func (p *HomebrewProgress) renderLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			p.renderFinal()
			return
		case <-ticker.C:
			p.render()
			p.spinnerIdx++
		}
	}
}

// render draws all task lines
func (p *HomebrewProgress) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Move cursor up for each line
	if len(p.tasks) > 0 {
		fmt.Fprintf(p.output, "\033[%dA", len(p.tasks))
	}

	for _, task := range p.tasks {
		line := p.renderLine(task, p.spinnerIdx)
		if line == "" {
			line = " " // Empty line placeholder
		}
		fmt.Fprintf(p.output, "\033[K%s\n", line) // Clear line and print
	}
}

// renderFinal prints the final state
func (p *HomebrewProgress) renderFinal() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear and move up
	for range p.tasks {
		fmt.Fprintf(p.output, "\033[K\033[A")
	}

	// Print final state
	for _, task := range p.tasks {
		line := p.renderLine(task, p.spinnerIdx)
		if line != "" {
			fmt.Fprintf(p.output, "\033[K%s\n", line)
		}
	}
}
```

**Step 4: 运行测试确认通过**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_RenderLine|TestHomebrewProgress_FormatDuration"
```
Expected: PASS

**Step 5: 提交**

```bash
git add pkg/progress/homebrew.go pkg/progress/homebrew_test.go
git commit -m "feat(progress): add render engine for HomebrewProgress"
```

---

## Task 4: 实现摘要输出

**Files:**
- Modify: `pkg/progress/homebrew.go`
- Modify: `pkg/progress/homebrew_test.go`

**Step 1: 写测试**

添加到 `pkg/progress/homebrew_test.go`:

```go
func TestHomebrewProgress_Summary(t *testing.T) {
	p := NewHomebrewProgress(3, true)
	p.AddTask("nginx:latest")
	p.AddTask("redis:alpine")
	p.AddTask("python:3.11")

	p.CompleteTask(0, nil)
	p.CompleteTask(1, fmt.Errorf("timeout"))
	p.MarkSkipped(2)

	summary := p.GetSummary()
	assert.Equal(t, 1, summary.Completed)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Skipped)
}
```

**Step 2: 运行测试确认失败**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_Summary"
```
Expected: FAIL - method not defined

**Step 3: 实现摘要方法**

添加到 `pkg/progress/homebrew.go`:

```go
// Summary holds summary statistics
type Summary struct {
	Total     int
	Completed int
	Failed    int
	Skipped   int
	Duration  time.Duration
}

// MarkSkipped marks a task as skipped
func (p *HomebrewProgress) MarkSkipped(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= 0 && index < len(p.tasks) && p.tasks[index] != nil {
		p.tasks[index].Status = TaskSkipped
		p.tasks[index].EndTime = time.Now()
	}
}

// GetSummary returns summary statistics
func (p *HomebrewProgress) GetSummary() Summary {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := Summary{
		Total:    len(p.tasks),
		Duration: time.Since(p.startTime),
	}

	for _, task := range p.tasks {
		if task == nil {
			continue
		}
		switch task.Status {
		case TaskCompleted:
			s.Completed++
		case TaskFailed:
			s.Failed++
		case TaskSkipped:
			s.Skipped++
		}
	}

	return s
}

// PrintSummary prints the final summary
func (p *HomebrewProgress) PrintSummary() {
	s := p.GetSummary()

	fmt.Fprintln(p.output, "\n==> Summary")
	if s.Failed > 0 {
		fmt.Fprintf(p.output, "✓ %d succeeded, %d skipped, %d failed in %s\n",
			s.Completed, s.Skipped, s.Failed, formatDurationHomebrew(s.Duration))
	} else {
		fmt.Fprintf(p.output, "✓ %d succeeded, %d skipped in %s\n",
			s.Completed, s.Skipped, formatDurationHomebrew(s.Duration))
	}
}
```

**Step 4: 运行测试确认通过**

```bash
go test -v ./pkg/progress -run "TestHomebrewProgress_Summary"
```
Expected: PASS

**Step 5: 提交**

```bash
git add pkg/progress/homebrew.go pkg/progress/homebrew_test.go
git commit -m "feat(progress): add summary output for HomebrewProgress"
```

---

## Task 5: 集成到 Progress 接口

**Files:**
- Modify: `pkg/progress/progress.go`

**Step 1: 运行现有测试确保基线**

```bash
go test -v ./pkg/progress
```
Expected: PASS (现有测试)

**Step 2: 重构 Progress 使用 HomebrewProgress**

修改 `pkg/progress/progress.go`，替换 mpb 实现为 HomebrewProgress：

```go
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
	Label     string
	StageName string
	Percent   float64
	StartAt   time.Time
	Stage     value_objects.SyncStage
}

// FormatStageWithTarget formats stage name for distribution
func FormatStageWithTarget(stage value_objects.SyncStage, targetName string) string {
	if stage == value_objects.SyncStageDistributing && targetName != "" {
		return "Uploading → " + targetName
	}
	return simplifyStage(stage)
}

// simplifyStage converts internal stage to Homebrew-style
func simplifyStage(stage value_objects.SyncStage) string {
	switch stage {
	case value_objects.SyncStageChecking:
		return "Checking"
	case value_objects.SyncStageSyncing:
		return "Downloading"
	case value_objects.SyncStageDistributing:
		return "Uploading"
	default:
		return "Processing"
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

// Progress manages the progress display using Homebrew-style output
type Progress struct {
	homebrew   *HomebrewProgress
	images     []*ImageProgress
	imageMap   map[string]int
	total      int
	startedAt  time.Time
	mu         sync.Mutex
	noOutput   bool
	operation  string
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// NewProgress creates a new progress tracker with Homebrew-style display.
func NewProgress(total int, workerCount int, noOutput bool, operation string) *Progress {
	if operation == "" {
		operation = "syncing"
	}
	if !noOutput && !isTerminal() {
		noOutput = true
	}

	p := &Progress{
		homebrew:  NewHomebrewProgress(total, noOutput),
		images:    make([]*ImageProgress, total),
		imageMap:  make(map[string]int),
		total:     total,
		startedAt: time.Now(),
		noOutput:  noOutput,
		operation: operation,
	}

	for i := range total {
		p.images[i] = &ImageProgress{}
	}

	p.homebrew.Start()
	return p
}

// AddImage adds an image to track
func (p *Progress) AddImage(index int, image string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.imageMap[image] = index
	if p.images[index] == nil {
		p.images[index] = &ImageProgress{}
	}
	p.images[index].Image = image
	p.images[index].Status = StatusPending

	p.homebrew.AddTask(image)
}

// UpdateStatus updates the status of an image
func (p *Progress) UpdateStatus(index int, status ImageStatus, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.images) || p.images[index] == nil {
		return
	}

	p.images[index].Status = status
	p.images[index].Error = err

	switch status {
	case StatusCompleted:
		p.homebrew.CompleteTask(index, nil)
	case StatusFailed:
		p.homebrew.CompleteTask(index, err)
	case StatusSkipped:
		p.homebrew.MarkSkipped(index)
	}
}

// SetDuration records how long a specific image took
func (p *Progress) SetDuration(index int, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.images) || p.images[index] == nil {
		return
	}
	p.images[index].Duration = d
}

// UpdateWorker is deprecated - kept for compatibility
func (p *Progress) UpdateWorker(workerIdx int, imageName string) {
	// No-op in Homebrew style
}

// UpdateStage is deprecated - use UpdateSyncStage
func (p *Progress) UpdateStage(workerIdx int, info StageInfo) {
	// No-op
}

// UpdateSyncStage updates the stage display
func (p *Progress) UpdateSyncStage(workerIdx int, imageName string, stage value_objects.SyncStage, targetName string, percent float64) {
	if p.noOutput {
		return
	}

	stageName := FormatStageWithTarget(stage, targetName)
	if idx, ok := p.imageMap[imageName]; ok {
		p.homebrew.UpdateStage(idx, stageName)
	}
}

// ClearWorker is deprecated
func (p *Progress) ClearWorker(workerIdx int) {
	// No-op
}

// IsNoOutput returns true if progress output is disabled
func (p *Progress) IsNoOutput() bool {
	return p.noOutput
}

// GetImages returns a copy of all images with their status
func (p *Progress) GetImages() []*ImageProgress {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]*ImageProgress, len(p.images))
	copy(result, p.images)
	return result
}

// Increment increments the completed count
func (p *Progress) Increment() {
	// No explicit progress bar in Homebrew style
}

// SetInitialProgress sets initial completed count (for skipped items)
func (p *Progress) SetInitialProgress(count int) {
	// No explicit progress bar
}

// CompleteSkipped marks images from start as completed
func (p *Progress) CompleteSkipped(start int) {
	for i := start; i < len(p.images); i++ {
		if p.images[i] != nil && p.images[i].Image != "" {
			p.homebrew.MarkSkipped(i)
		}
	}
}

// AbortWorkers stops the display
func (p *Progress) AbortWorkers() {
	p.homebrew.Stop()
}

// WaitContainer waits for render to complete
func (p *Progress) WaitContainer() {
	p.homebrew.Stop()
}

// Wait stops the display and prints summary
func (p *Progress) Wait() {
	p.homebrew.Stop()
	p.printSummary()
}

// LogWriter returns an io.Writer for logs
func (p *Progress) LogWriter() io.Writer {
	if p.noOutput {
		return io.Discard
	}
	return os.Stdout
}

// printSummary prints the final summary
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

	fmt.Fprintf(os.Stdout, "\n==> Summary\n")
	totalFailed := failed + cancelled
	if totalFailed > 0 {
		fmt.Fprintf(os.Stdout, "✓ %d succeeded, %d skipped, %d failed in %s\n",
			succeeded, skipped, totalFailed, formatDurationHomebrew(totalDuration))
	} else {
		fmt.Fprintf(os.Stdout, "✓ %d succeeded, %d skipped in %s\n",
			succeeded, skipped, formatDurationHomebrew(totalDuration))
	}
}

// Helper functions kept for compatibility
var hashSuffixRe = regexp.MustCompile(`-[0-9a-f]{20,}$`)

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
```

**Step 3: 运行所有测试**

```bash
go test -v ./pkg/progress
```
Expected: PASS

**Step 4: 提交**

```bash
git add pkg/progress/progress.go
git commit -m "refactor(progress): replace mpb with Homebrew-style display"
```

---

## Task 6: 移除 mpb 依赖

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: 运行 go mod tidy**

```bash
go mod tidy
```

**Step 2: 验证依赖已移除**

```bash
grep -c "mpb" go.mod || echo "mpb removed"
```
Expected: 0 (mpb removed)

**Step 3: 运行完整测试套件**

```bash
make test
```
Expected: PASS

**Step 4: 提交**

```bash
git add go.mod go.sum
git commit -m "chore: remove mpb dependency"
```

---

## Task 7: 验证和清理

**Files:**
- All modified files

**Step 1: 构建验证**

```bash
make build
```
Expected: Success

**Step 2: 运行完整测试**

```bash
make test
```
Expected: PASS

**Step 3: 运行代码检查**

```bash
make vet
make fmt
```
Expected: No errors

**Step 4: 手动测试（需要用户执行）**

```bash
# 测试单个镜像
./image-copier sync nginx:latest

# 测试多个镜像并发
./image-copier sync nginx:latest redis:alpine python:3.11

# 测试 JSON 输出
./image-copier sync nginx:latest --output json
```

**Step 5: 最终提交**

```bash
git add -A
git commit -m "feat: complete Homebrew-style output implementation"
```

---

## 验证清单

- [ ] HomebrewProgress 单元测试通过
- [ ] Progress 集成测试通过
- [ ] 完整测试套件通过 (`make test`)
- [ ] 构建成功 (`make build`)
- [ ] 代码检查通过 (`make vet`)
- [ ] 手动测试输出格式正确
- [ ] JSON 模式不受影响
- [ ] 非 TTY 环境正常工作