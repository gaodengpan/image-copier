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
