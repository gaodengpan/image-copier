package use_cases

import (
	"testing"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/stretchr/testify/assert"
)

func TestProgressNotifier_NilSafe(t *testing.T) {
	// 测试 nil callback 不会 panic
	notifier := NewProgressNotifier(nil)

	assert.NotPanics(t, func() {
		notifier.Notify("test-image", value_objects.SyncStageChecking, "", 50)
	})
}

func TestProgressNotifier_NotifyChecking(t *testing.T) {
	tests := []struct {
		name    string
		percent float64
	}{
		{"start", ProgressStart},
		{"halfway", ProgressHalfway},
		{"done", ProgressDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				called          bool
				receivedImageID string
				receivedStage   value_objects.SyncStage
				receivedPercent float64
			)

			callback := func(imageID string, stage value_objects.SyncStage, targetName string, percent float64) {
				called = true
				receivedImageID = imageID
				receivedStage = stage
				receivedPercent = percent
			}

			notifier := NewProgressNotifier(callback)
			notifier.NotifyChecking("test-image", tt.percent)

			assert.True(t, called)
			assert.Equal(t, "test-image", receivedImageID)
			assert.Equal(t, value_objects.SyncStageChecking, receivedStage)
			assert.Equal(t, tt.percent, receivedPercent)
		})
	}
}

func TestProgressNotifier_NotifySyncing(t *testing.T) {
	var receivedStage value_objects.SyncStage

	notifier := NewProgressNotifier(func(imageID string, stage value_objects.SyncStage, targetName string, percent float64) {
		receivedStage = stage
	})

	notifier.NotifySyncing("test-image", ProgressHalfway)
	assert.Equal(t, value_objects.SyncStageSyncing, receivedStage)
}

func TestProgressNotifier_NotifyDistributing(t *testing.T) {
	var (
		receivedTargetName string
		receivedPercent    float64
	)

	notifier := NewProgressNotifier(func(imageID string, stage value_objects.SyncStage, targetName string, percent float64) {
		receivedTargetName = targetName
		receivedPercent = percent
	})

	notifier.NotifyDistributing("test-image", "docker", 66.67)

	assert.Equal(t, "docker", receivedTargetName)
	assert.InDelta(t, 66.67, receivedPercent, 0.01)
}

func TestProgressConstants(t *testing.T) {
	// 验证进度常量值
	assert.Equal(t, 0.0, ProgressStart)
	assert.Equal(t, 50.0, ProgressHalfway)
	assert.Equal(t, 100.0, ProgressDone)
}
