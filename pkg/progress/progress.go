package progress

import (
	"fmt"
	"strings"
	"sync"
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
	Image  string
	Status ImageStatus
	Error  error
}

// Progress manages the progress display for multiple images
type Progress struct {
	images      []*ImageProgress
	current     int
	total       int
	mu          sync.Mutex
	width       int
	showDetails bool
}

// NewProgress creates a new progress tracker
func NewProgress(total int, showDetails bool) *Progress {
	return &Progress{
		images:      make([]*ImageProgress, total),
		total:       total,
		width:       50, // progress bar width
		showDetails: showDetails,
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
func (p *Progress) UpdateStatus(index int, status ImageStatus, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < len(p.images) && p.images[index] != nil {
		p.images[index].Status = status
		p.images[index].Error = err
	}
}

// IncrementProgress increments the completed count and displays progress
func (p *Progress) IncrementProgress() {
	p.mu.Lock()
	p.current++
	p.mu.Unlock()
	p.Display()
}

// Display shows the progress bar and image statuses
func (p *Progress) Display() {
	p.mu.Lock()
	defer p.mu.Unlock()

	completed := p.current
	percentage := float64(completed) / float64(p.total) * 100
	barWidth := float64(p.width) * percentage / 100
	bar := strings.Repeat("=", int(barWidth)) + strings.Repeat(" ", p.width-int(barWidth))

	// Clear current line and write progress bar
	fmt.Printf("\r[%d/%d] [%s] %.1f%%", completed, p.total, bar, percentage)

	if p.showDetails || completed == p.total {
		// Show detailed image statuses
		fmt.Println() // Move to next line
		p.displayImageStatuses()
	}
}

// displayImageStatuses shows the status of all images
func (p *Progress) displayImageStatuses() {
	if !p.showDetails {
		return
	}

	for _, img := range p.images {
		if img == nil {
			continue
		}
		icon := p.getStatusIcon(img.Status)
		fmt.Printf("  %s %s %s", icon, img.Image, img.Status.String())
		if img.Error != nil {
			fmt.Printf(": %v", img.Error)
		}
		fmt.Println()
	}
}

// getStatusIcon returns an icon for the status
func (p *Progress) getStatusIcon(status ImageStatus) string {
	switch status {
	case StatusPending:
		return "○"
	case StatusRunning:
		return "◐"
	case StatusCompleted:
		return "●"
	case StatusFailed:
		return "✗"
	default:
		return "?"
	}
}

// StartWatching starts watching the progress and displaying updates
func (p *Progress) StartWatching(done <-chan struct{}) {
	ticker := make(chan struct{})
	close(ticker)

	for {
		select {
		case <-ticker:
			p.Display()
		case <-done:
			p.Display()
			return
		}
	}
}

// BatchProgress manages batch processing with worker pool
type BatchProgress struct {
	progress *Progress
}

// NewBatchProgress creates a new batch progress manager
func NewBatchProgress(images []string, showDetails bool) *BatchProgress {
	total := len(images)
	p := NewProgress(total, showDetails)

	for i, img := range images {
		p.AddImage(i, img)
	}

	return &BatchProgress{progress: p}
}

// Process processes the images with a worker pool
func (bp *BatchProgress) Process(workerCount int, processFunc func(int, string) error) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(bp.progress.images))
	done := make(chan struct{})
	defer close(done)

	// Start progress display
	go bp.progress.StartWatching(done)

	// Create worker pool
	jobs := make(chan int, len(bp.progress.images))
	for i := 0; i < len(bp.progress.images); i++ {
		jobs <- i
	}
	close(jobs)

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				img := bp.progress.images[idx]
				if img == nil {
					continue
				}

				bp.progress.UpdateStatus(idx, StatusRunning, nil)

				err := processFunc(idx, img.Image)
				if err != nil {
					bp.progress.UpdateStatus(idx, StatusFailed, err)
					errChan <- err
				} else {
					bp.progress.UpdateStatus(idx, StatusCompleted, nil)
				}

				bp.progress.IncrementProgress()
			}
		}()
	}

	wg.Wait()

	// Collect errors
	var errors []error
	close(errChan)
	for err := range errChan {
		errors = append(errors, err)
	}

	// Display summary
	bp.displaySummary(len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("%d images failed", len(errors))
	}

	return nil
}

// displaySummary shows the final summary
func (bp *BatchProgress) displaySummary(failedCount int) {
	successCount := bp.progress.total - failedCount
	fmt.Printf("\n\nCompleted: %d/%d images processed successfully", successCount, bp.progress.total)
	if failedCount > 0 {
		fmt.Printf(" (%d failed)", failedCount)
	}
	fmt.Println()
}

// GetTerminalWidth returns the terminal width
func GetTerminalWidth() int {
	// Try to get terminal width using TIOCGWINSZ
	// This is a simplified version - a real implementation would use syscall
	// For now, return a reasonable default
	return 80
}

// SetWidth sets the progress bar width
func (p *Progress) SetWidth(width int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width = width
}