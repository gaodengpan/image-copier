package progress

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
		{StatusSkipped, "skipped"},
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
	p := NewProgress(10, 3)

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
	if len(p.workerBars) != 3 {
		t.Errorf("Expected 3 worker bars, got %d", len(p.workerBars))
	}
	if len(p.workerTexts) != 3 {
		t.Errorf("Expected 3 worker texts, got %d", len(p.workerTexts))
	}

	// Clean up
	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_AddImage tests adding images to track
func TestProgress_AddImage(t *testing.T) {
	p := NewProgress(3, 1)

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
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_UpdateStatus tests updating image status
func TestProgress_UpdateStatus(t *testing.T) {
	p := NewProgress(2, 1)
	p.AddImage(0, "image1")
	p.AddImage(1, "image2")

	p.UpdateStatus(0, StatusRunning, nil)
	if p.images[0].Status != StatusRunning {
		t.Errorf("Expected status to be running, got %v", p.images[0].Status)
	}

	p.UpdateStatus(0, StatusCompleted, nil)
	if p.images[0].Status != StatusCompleted {
		t.Errorf("Expected status to be completed, got %v", p.images[0].Status)
	}

	testErr := errors.New("test error")
	p.UpdateStatus(1, StatusFailed, testErr)
	if p.images[1].Status != StatusFailed {
		t.Errorf("Expected status to be failed, got %v", p.images[1].Status)
	}
	if p.images[1].Error == nil || p.images[1].Error.Error() != "test error" {
		t.Errorf("Expected error to be set, got %v", p.images[1].Error)
	}

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_UpdateStatus_OutOfBounds tests bounds checking
func TestProgress_UpdateStatus_OutOfBounds(t *testing.T) {
	p := NewProgress(2, 1)
	p.AddImage(0, "image1")

	// Should not panic on out-of-bounds
	p.UpdateStatus(5, StatusRunning, nil)
	// Should not panic on nil image
	p.UpdateStatus(1, StatusRunning, nil)

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_UpdateWorker tests worker text updates
func TestProgress_UpdateWorker(t *testing.T) {
	p := NewProgress(3, 2)

	// Set worker text
	p.UpdateWorker(0, "nginx:latest")
	p.UpdateWorker(1, "redis:alpine")

	info0 := p.workerTexts[0].Load().(StageInfo)
	if info0.Label != "nginx:latest" {
		t.Errorf("Expected worker 0 label to be nginx:latest, got %s", info0.Label)
	}
	info1 := p.workerTexts[1].Load().(StageInfo)
	if info1.Label != "redis:alpine" {
		t.Errorf("Expected worker 1 label to be redis:alpine, got %s", info1.Label)
	}

	// Clear worker text
	p.UpdateWorker(0, "")
	info0 = p.workerTexts[0].Load().(StageInfo)
	if info0.Label != "" {
		t.Errorf("Expected worker 0 label to be empty, got %s", info0.Label)
	}

	// Out of bounds should not panic
	p.UpdateWorker(5, "test")

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_Increment tests progress increment
func TestProgress_Increment(t *testing.T) {
	p := NewProgress(5, 1)

	p.Increment()
	p.Increment()

	current := p.mainBar.Current()
	if current != 2 {
		t.Errorf("Expected mainBar current to be 2, got %d", current)
	}

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_ConcurrentUpdates tests concurrent status updates
func TestProgress_ConcurrentUpdates(t *testing.T) {
	const n = 50
	const workers = 4
	p := NewProgress(n, workers)

	for i := 0; i < n; i++ {
		p.AddImage(i, "image")
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			workerIdx := idx % workers
			p.UpdateWorker(workerIdx, "image")
			p.UpdateStatus(idx, StatusRunning, nil)
			p.UpdateStatus(idx, StatusCompleted, nil)
			p.UpdateWorker(workerIdx, "")
			p.Increment()
		}(i)
	}

	wg.Wait()

	current := p.mainBar.Current()
	if current != int64(n) {
		t.Errorf("Expected all %d images to be processed, got %d", n, current)
	}

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_FullWorkflow tests the complete lifecycle
func TestProgress_FullWorkflow(t *testing.T) {
	images := []string{"nginx:latest", "redis:alpine", "postgres:15"}
	workerCount := 2
	p := NewProgress(len(images), workerCount)

	for i, img := range images {
		p.AddImage(i, img)
	}

	var failCount atomic.Int32
	var wg sync.WaitGroup

	jobs := make(chan int, len(images))
	for i := range images {
		jobs <- i
	}
	close(jobs)

	for i := 0; i < workerCount; i++ {
		workerIdx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				p.UpdateWorker(workerIdx, images[idx])
				p.UpdateStatus(idx, StatusRunning, nil)

				if idx == 1 {
					p.UpdateStatus(idx, StatusFailed, errors.New("connection refused"))
					failCount.Add(1)
				} else {
					p.UpdateStatus(idx, StatusCompleted, nil)
				}

				p.UpdateWorker(workerIdx, "")
				p.Increment()
			}
		}()
	}

	wg.Wait()
	p.Wait()

	if failCount.Load() != 1 {
		t.Errorf("Expected 1 failure, got %d", failCount.Load())
	}
}

// TestProgress_ZeroImages tests progress with zero images
func TestProgress_ZeroImages(t *testing.T) {
	p := NewProgress(0, 0)

	if p.total != 0 {
		t.Errorf("Expected total to be 0, got %d", p.total)
	}

	p.Wait()
}

// TestProgress_LogWriter tests that LogWriter returns a valid io.Writer
func TestProgress_LogWriter(t *testing.T) {
	p := NewProgress(1, 1)

	w := p.LogWriter()
	if w == nil {
		t.Fatal("Expected non-nil io.Writer from LogWriter()")
	}

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// BenchmarkProgress_Increment benchmarks increment operations
func BenchmarkProgress_Increment(b *testing.B) {
	p := NewProgress(b.N, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Increment()
	}
	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// BenchmarkProgress_UpdateStatus benchmarks status updates
func BenchmarkProgress_UpdateStatus(b *testing.B) {
	const size = 1000
	p := NewProgress(size, 4)
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
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_UpdateStage tests StageInfo storage and retrieval
func TestProgress_UpdateStage(t *testing.T) {
	p := NewProgress(3, 2)

	now := time.Now()
	info := StageInfo{
		Label:     "nginx:latest",
		StageName: "workflow running",
		Percent:   45.5,
		StartAt:   now,
	}
	p.UpdateStage(0, info)

	got := p.workerTexts[0].Load().(StageInfo)
	if got.Label != "nginx:latest" {
		t.Errorf("Label = %q, want %q", got.Label, "nginx:latest")
	}
	if got.StageName != "workflow running" {
		t.Errorf("StageName = %q, want %q", got.StageName, "workflow running")
	}
	if got.Percent != 45.5 {
		t.Errorf("Percent = %f, want 45.5", got.Percent)
	}
	if !got.StartAt.Equal(now) {
		t.Errorf("StartAt mismatch")
	}

	// Out of bounds should not panic
	p.UpdateStage(5, info)

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestProgress_UpdateStage_Idle tests that empty Label renders as idle
func TestProgress_UpdateStage_Idle(t *testing.T) {
	p := NewProgress(1, 1)

	p.UpdateStage(0, StageInfo{})

	got := p.workerTexts[0].Load().(StageInfo)
	if got.Label != "" {
		t.Errorf("Expected empty Label for idle, got %q", got.Label)
	}

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()
}

// TestStageInfo_Rendering tests the formatted rendering of stage info
func TestStageInfo_Rendering(t *testing.T) {
	now := time.Now().Add(-32 * time.Second)
	info := StageInfo{
		Label:     "nginx:latest",
		StageName: "workflow running",
		Percent:   45,
		StartAt:   now,
	}

	elapsed := time.Since(info.StartAt).Truncate(time.Second)
	rendered := fmt.Sprintf("  ◐ %-50s [%3.0f%%] %s (%s)",
		smartTruncate(stripHash(info.Label), 50), info.Percent, info.StageName, elapsed)

	if len(rendered) == 0 {
		t.Error("Expected non-empty rendered string")
	}
	// Verify it contains the key parts
	if !containsSubstring(rendered, "nginx:latest") {
		t.Errorf("Rendered string missing image name: %s", rendered)
	}
	if !containsSubstring(rendered, "45%") {
		t.Errorf("Rendered string missing percentage: %s", rendered)
	}
	if !containsSubstring(rendered, "workflow running") {
		t.Errorf("Rendered string missing stage name: %s", rendered)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestFormatDuration tests the duration formatting helper
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "<1s"},
		{500 * time.Millisecond, "<1s"},
		{1 * time.Second, "1s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{61 * time.Second, "1m1s"},
		{72 * time.Second, "1m12s"},
		{154 * time.Second, "2m34s"},
		{300 * time.Second, "5m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestProgress_SetDuration tests recording duration for an image
func TestProgress_SetDuration(t *testing.T) {
	p := NewProgress(2, 1)
	p.AddImage(0, "nginx:latest")
	p.AddImage(1, "redis:alpine")

	p.SetDuration(0, 72*time.Second)
	p.SetDuration(1, 5*time.Second)

	if p.images[0].Duration != 72*time.Second {
		t.Errorf("Expected duration 72s, got %v", p.images[0].Duration)
	}
	if p.images[1].Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", p.images[1].Duration)
	}

	// Out of bounds should not panic
	p.SetDuration(5, time.Second)
	// Nil image should not panic
	p2 := NewProgress(2, 1)
	p2.SetDuration(0, time.Second)

	p.mainBar.Abort(true)
	for _, bar := range p.workerBars {
		bar.Abort(true)
	}
	p.container.Wait()

	p2.mainBar.Abort(true)
	for _, bar := range p2.workerBars {
		bar.Abort(true)
	}
	p2.container.Wait()
}

// TestTruncate tests the truncate helper function
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this-is-a-long-string", 10, "this-is..."},
		{"abc", 3, "abc"},
		{"abcd", 4, "abcd"},
		{"abcde", 4, "a..."},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.input, tt.maxLen), func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

// TestSmartTruncate tests the smartTruncate helper function
func TestSmartTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		// Short names — no truncation needed
		{"nginx:latest", 50, "nginx:latest"},
		{"ghcr.io/tektoncd/pipeline/cmd/git-clone:v1.1.0", 50, "ghcr.io/tektoncd/pipeline/cmd/git-clone:v1.1.0"},
		{"ghcr.io/some-org/some-very-long-image-name:v1.2.3", 50, "ghcr.io/some-org/some-very-long-image-name:v1.2.3"},
		// Exactly at maxLen
		{"docker.io/library/nginx:latest", 30, "docker.io/library/nginx:latest"},
		// Truncation cases with maxLen=50
		{"ghcr.io/nginx-gateway-fabric/nginx-gateway-fabric:2.0.1", 50, "ghcr.io/nginx-gatew...c/nginx-gateway-fabric:2.0.1"},
		{"registry.example.com/org/team/my-service:v2.0.0-rc.1", 50, "registry.example.co.../team/my-service:v2.0.0-rc.1"},
		// Truncation cases with maxLen=30
		{"ghcr.io/some-org/some-very-long-image-name:v1.2.3", 30, "ghcr.io/som...mage-name:v1.2.3"},
		{"registry.example.com/org/team/my-service:v2.0.0-rc.1", 30, "registry.ex...vice:v2.0.0-rc.1"},
		// Edge cases
		{"", 50, ""},
		{"abcdefgh", 3, "abc"},
		{"abcdefghij", 4, "a..."},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.input, tt.maxLen), func(t *testing.T) {
			got := smartTruncate(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("smartTruncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
			if len(got) > tt.maxLen {
				t.Errorf("smartTruncate(%q, %d) length = %d, exceeds maxLen", tt.input, tt.maxLen, len(got))
			}
		})
	}
}

// TestStripHash tests hash digest suffix removal from image names
func TestStripHash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// With hash suffix — should be stripped
		{"ghcr.io/tektoncd/pipeline/controller-10a3e32792f33651396d02b6855a6e36:v1.1.0",
			"ghcr.io/tektoncd/pipeline/controller:v1.1.0"},
		{"ghcr.io/tektoncd/pipeline/events-a9042f7efb0cbade2a868a1ee5ddd52c:v1.1.0",
			"ghcr.io/tektoncd/pipeline/events:v1.1.0"},
		{"ghcr.io/tektoncd/dashboard/dashboard-9623576a202fe86c8b7d1bc489905f86:v0.55.0",
			"ghcr.io/tektoncd/dashboard/dashboard:v0.55.0"},
		{"ghcr.io/tektoncd/triggers/interceptors-3176d6a3f314c3655b30bfd36e421dd5:v0.32.0",
			"ghcr.io/tektoncd/triggers/interceptors:v0.32.0"},
		// Without hash — unchanged
		{"nginx:latest", "nginx:latest"},
		{"ghcr.io/nginx/nginx-gateway-fabric:2.0.1", "ghcr.io/nginx/nginx-gateway-fabric:2.0.1"},
		{"ghcr.io/tektoncd-catalog/git-clone:v1.1.0", "ghcr.io/tektoncd-catalog/git-clone:v1.1.0"},
		// No tag
		{"ghcr.io/tektoncd/pipeline/controller-10a3e32792f33651396d02b6855a6e36",
			"ghcr.io/tektoncd/pipeline/controller"},
		// Short hex suffix (< 20 chars) — NOT stripped
		{"myimage-abc123:v1", "myimage-abc123:v1"},
		// Empty string
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripHash(tt.input)
			if got != tt.expected {
				t.Errorf("stripHash(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
