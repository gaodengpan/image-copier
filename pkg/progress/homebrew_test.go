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
		// Note: In non-TTY environment (like tests), noOutput is always true
		// because term.IsTerminal returns false
		{"with output (non-TTY)", 5, false, true},
		{"no output mode", 5, true, true},
		{"zero total (non-TTY)", 0, false, true},
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
