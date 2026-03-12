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
	Stage     string // "Checking", "Downloading", "Uploading"
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

// currentSpinner returns the current spinner frame and advances it (not thread-safe)
func (p *HomebrewProgress) currentSpinner() string {
	frame := brailleSpinner[p.spinnerIdx]
	p.spinnerIdx = (p.spinnerIdx + 1) % len(brailleSpinner)
	return string(frame)
}

// Spinner returns the current spinner frame and advances it (thread-safe)
func (p *HomebrewProgress) Spinner() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentSpinner()
}

// Start begins the progress display animation
func (p *HomebrewProgress) Start() {
	if p.noOutput || p.stopChan == nil {
		return
	}

	p.wg.Add(1)
	go p.animate()
}

// Stop stops the progress display animation
func (p *HomebrewProgress) Stop() {
	if p.noOutput || p.stopChan == nil {
		return
	}

	close(p.stopChan)
	p.wg.Wait()
}

// animate runs the animation loop
func (p *HomebrewProgress) animate() {
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
		}
	}
}

// renderFinal draws the final state when animation stops
func (p *HomebrewProgress) renderFinal() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear previous lines
	for range p.tasks {
		fmt.Fprint(p.output, "\033[1A\033[K")
	}

	// Render each task line with final state
	for _, task := range p.tasks {
		if task == nil {
			continue
		}

		var line string
		switch task.Status {
		case TaskPending:
			if task.ImageName != "" {
				line = fmt.Sprintf("  ◦ %s (pending)", task.ImageName)
			}
		case TaskRunning:
			// Task still running at stop time - show current state
			elapsed := time.Since(task.StartTime).Truncate(time.Second)
			if task.Stage != "" {
				line = fmt.Sprintf("  %s %s %s (%s)", p.currentSpinner(), task.ImageName, task.Stage, elapsed)
			} else {
				line = fmt.Sprintf("  %s %s (%s)", p.currentSpinner(), task.ImageName, elapsed)
			}
		case TaskCompleted:
			duration := task.EndTime.Sub(task.StartTime).Truncate(time.Second)
			line = fmt.Sprintf("  ✓ %s (%s)", task.ImageName, formatDurationHomebrew(duration))
		case TaskFailed:
			duration := task.EndTime.Sub(task.StartTime).Truncate(time.Second)
			line = fmt.Sprintf("  ✗ %s (%s)", task.ImageName, formatDurationHomebrew(duration))
			if task.Error != nil {
				line += fmt.Sprintf(": %v", task.Error)
			}
		case TaskSkipped:
			line = fmt.Sprintf("  ◦ %s", task.ImageName)
		}

		if line != "" {
			fmt.Fprintln(p.output, line)
		}
	}
}

// render draws the current state to the terminal
func (p *HomebrewProgress) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear previous lines
	for range p.tasks {
		fmt.Fprint(p.output, "\033[1A\033[K")
	}

	// Render each task line
	for _, task := range p.tasks {
		if task == nil {
			continue
		}

		var line string
		switch task.Status {
		case TaskPending:
			if task.ImageName != "" {
				line = fmt.Sprintf("  %s %s...", p.currentSpinner(), task.ImageName)
			}
		case TaskRunning:
			elapsed := time.Since(task.StartTime).Truncate(time.Second)
			if task.Stage != "" {
				line = fmt.Sprintf("  %s %s %s (%s)", p.currentSpinner(), task.ImageName, task.Stage, elapsed)
			} else {
				line = fmt.Sprintf("  %s %s (%s)", p.currentSpinner(), task.ImageName, elapsed)
			}
		case TaskCompleted:
			duration := task.EndTime.Sub(task.StartTime).Truncate(time.Second)
			line = fmt.Sprintf("  ✓ %s (%s)", task.ImageName, formatDurationHomebrew(duration))
		case TaskFailed:
			duration := task.EndTime.Sub(task.StartTime).Truncate(time.Second)
			line = fmt.Sprintf("  ✗ %s (%s)", task.ImageName, formatDurationHomebrew(duration))
			if task.Error != nil {
				line += fmt.Sprintf(": %v", task.Error)
			}
		case TaskSkipped:
			line = fmt.Sprintf("  ◦ %s", task.ImageName)
		}

		if line != "" {
			fmt.Fprintln(p.output, line)
		}
	}
}

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

// UpdateTask updates a task's status and stage
func (p *HomebrewProgress) UpdateTask(index int, status TaskStatus, stage string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.tasks) || p.tasks[index] == nil {
		return
	}

	task := p.tasks[index]
	task.Status = status
	task.Stage = stage
	task.Error = err

	if status == TaskRunning && task.StartTime.IsZero() {
		task.StartTime = time.Now()
	}
	if status == TaskCompleted || status == TaskFailed || status == TaskSkipped {
		task.EndTime = time.Now()
	}
}

// CompleteTask marks a task as completed
func (p *HomebrewProgress) CompleteTask(index int) {
	p.UpdateTask(index, TaskCompleted, "", nil)
}

// FailTask marks a task as failed
func (p *HomebrewProgress) FailTask(index int, err error) {
	p.UpdateTask(index, TaskFailed, "", err)
}

// SkipTask marks a task as skipped
func (p *HomebrewProgress) SkipTask(index int) {
	p.UpdateTask(index, TaskSkipped, "", nil)
}

// SetTaskImage sets the image name for a task
func (p *HomebrewProgress) SetTaskImage(index int, imageName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index >= len(p.tasks) || p.tasks[index] == nil {
		return
	}
	p.tasks[index].ImageName = imageName
}
