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

	p := &Progress{
		container: container,
		mainBar:   mainBar,
		images:    make([]*ImageProgress, total),
		total:     total,
	}

	// When total is 0, the bar will never reach completion naturally,
	// so abort it immediately to allow container.Wait() to return.
	if total == 0 {
		mainBar.Abort(true)
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
