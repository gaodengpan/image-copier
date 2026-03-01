package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestImageStatusString(t *testing.T) {
	tests := []struct {
		status   ImageStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "syncing..."},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
		{StatusDryRun, "dry-run"},
		{StatusCancelled, "cancelled"},
		{ImageStatus(100), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestNewProgress_NoOutput(t *testing.T) {
	p := NewProgress(5, 3, true)
	assert.NotNil(t, p)
	assert.Equal(t, 5, p.total)
	assert.True(t, p.noOutput)
	assert.Len(t, p.images, 5)
}

func TestNewProgress_ZeroTotal(t *testing.T) {
	p := NewProgress(0, 3, false)
	assert.NotNil(t, p)
	assert.Equal(t, 0, p.total)
}

func TestProgress_AddImage(t *testing.T) {
	p := NewProgress(3, 2, true)

	p.AddImage(0, "nginx:latest")
	p.AddImage(1, "redis:alpine")
	p.AddImage(2, "python:3.9")

	images := p.GetImages()
	assert.Equal(t, "nginx:latest", images[0].Image)
	assert.Equal(t, "redis:alpine", images[1].Image)
	assert.Equal(t, "python:3.9", images[2].Image)
}

func TestProgress_AddImage_OutOfBounds(t *testing.T) {
	p := NewProgress(2, 2, true)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic: %v", r)
		}
	}()

	p.AddImage(5, "nginx:latest")
}

func TestProgress_UpdateStatus(t *testing.T) {
	p := NewProgress(3, 2, true)

	p.AddImage(0, "nginx:latest")
	p.UpdateStatus(0, StatusCompleted, nil)
	p.UpdateStatus(1, StatusFailed, assert.AnError)
	p.UpdateStatus(2, StatusSkipped, nil)

	images := p.GetImages()
	assert.Equal(t, StatusCompleted, images[0].Status)
	assert.Equal(t, StatusFailed, images[1].Status)
	assert.Equal(t, assert.AnError, images[1].Error)
	assert.Equal(t, StatusSkipped, images[2].Status)
}

func TestProgress_UpdateStatus_OutOfBounds(t *testing.T) {
	p := NewProgress(2, 2, true)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic: %v", r)
		}
	}()

	p.UpdateStatus(5, StatusCompleted, nil)
}

func TestProgress_UpdateStatus_NilImage(t *testing.T) {
	p := NewProgress(2, 2, true)

	p.UpdateStatus(0, StatusCompleted, nil)

	images := p.GetImages()
	assert.NotNil(t, images[0])
	assert.Equal(t, StatusCompleted, images[0].Status)
}

func TestProgress_SetDuration(t *testing.T) {
	p := NewProgress(3, 2, true)

	p.AddImage(0, "nginx:latest")
	p.SetDuration(0, 5*time.Second)

	images := p.GetImages()
	assert.Equal(t, 5*time.Second, images[0].Duration)
}

func TestProgress_SetDuration_OutOfBounds(t *testing.T) {
	p := NewProgress(2, 2, true)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic: %v", r)
		}
	}()

	p.SetDuration(5, 5*time.Second)
}

func TestProgress_UpdateWorker_NoOutput(t *testing.T) {
	p := NewProgress(3, 2, true)

	p.UpdateWorker(0, "nginx:latest")

	images := p.GetImages()
	assert.Equal(t, "", images[0].Image)
}

func TestProgress_UpdateStage_NoOutput(t *testing.T) {
	p := NewProgress(3, 2, true)

	info := StageInfo{
		Label:     "nginx:latest",
		StageName: "pulling",
		Percent:   50.0,
		StartAt:   time.Now(),
	}
	p.UpdateStage(0, info)
}

func TestProgress_GetImages(t *testing.T) {
	p := NewProgress(3, 2, true)

	p.AddImage(0, "nginx:latest")
	p.AddImage(1, "redis:alpine")

	images := p.GetImages()
	assert.Len(t, images, 3)
	assert.Equal(t, "nginx:latest", images[0].Image)
	assert.Equal(t, "redis:alpine", images[1].Image)
}

func TestProgress_Increment(t *testing.T) {
	p := NewProgress(3, 2, true)
	p.Increment()
}

func TestProgress_SetInitialProgress(t *testing.T) {
	p := NewProgress(3, 2, true)
	p.SetInitialProgress(2)
}

func TestProgress_CompleteSkipped(t *testing.T) {
	p := NewProgress(3, 2, true)
	p.CompleteSkipped(1)
}

func TestProgress_AbortWorkers(t *testing.T) {
	p := NewProgress(3, 2, true)
	p.AbortWorkers()
}

func TestProgress_WaitContainer(t *testing.T) {
	p := NewProgress(3, 2, true)
	p.WaitContainer()
}

func TestStripHash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"nginx:latest", "nginx:latest"},
		{"ghcr.io/tektoncd/pipeline/controller-10a3e32792f33651396d02b6855a6e36:v1.1.0", "ghcr.io/tektoncd/pipeline/controller:v1.1.0"},
		{"my-registry.io/app@sha256:abc1234567890def1234567890abcdef1234567890abcdef1234567890abcdef", "my-registry.io/app@sha256:abc1234567890def1234567890abcdef1234567890abcdef1234567890abcdef"},
		{"no-hash", "no-hash"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripHash(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSmartTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly", 7, "exactly"},
		{"toolong", 5, "t...g"},
		{"averylongstring", 15, "averylongstring"},
		{"abc", 2, "ab"},
		{"", 5, ""},
		{"x", 1, "x"},
		{"xx", 1, "x"},
		{"abcdefghi", 10, "abcdefghi"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := smartTruncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			if len(tt.input) > tt.maxLen {
				assert.LessOrEqual(t, len(result), tt.maxLen)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "<1s"},
		{500 * time.Millisecond, "<1s"},
		{1 * time.Second, "1s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{1*time.Minute + 30*time.Second, "1m30s"},
		{5 * time.Minute, "5m"},
		{5*time.Minute + 45*time.Second, "5m45s"},
		{10 * time.Minute, "10m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogWriter(t *testing.T) {
	p := NewProgress(3, 2, true)
	writer := p.LogWriter()
	assert.NotNil(t, writer)
}

func TestProgress_UpdateWorker_IndexOutOfBounds(t *testing.T) {
	p := NewProgress(3, 2, false)
	p.UpdateWorker(10, "nginx:latest")
}

func TestProgress_UpdateStage_IndexOutOfBounds(t *testing.T) {
	p := NewProgress(3, 2, false)
	info := StageInfo{
		Label:     "nginx:latest",
		StageName: "pulling",
		Percent:   50.0,
		StartAt:   time.Now(),
	}
	p.UpdateStage(10, info)
}
