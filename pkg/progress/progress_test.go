package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		{StatusDryRun, "dry-run"},
		{StatusCancelled, "cancelled"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}
