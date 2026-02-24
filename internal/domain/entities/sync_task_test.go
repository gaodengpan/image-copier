package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status SyncStatus
		want   string
	}{
		{"pending", SyncStatusPending, "pending"},
		{"syncing", SyncStatusSyncing, "syncing"},
		{"completed", SyncStatusCompleted, "completed"},
		{"failed", SyncStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}

func TestSyncStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status SyncStatus
		want   bool
	}{
		{"pending", SyncStatusPending, false},
		{"syncing", SyncStatusSyncing, false},
		{"completed", SyncStatusCompleted, true},
		{"failed", SyncStatusFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsTerminal())
		})
	}
}

func TestNewSyncTask(t *testing.T) {
	task := NewSyncTask("1", "nginx:latest", "amd64", "linux")

	assert.Equal(t, "1", task.ID)
	assert.Equal(t, "nginx:latest", task.Source)
	assert.Equal(t, "amd64", task.Arch)
	assert.Equal(t, "linux", task.Os)
	assert.Equal(t, SyncStatusPending, task.Status)
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
}

func TestSyncTask_Start(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		err := task.Start()

		assert.NoError(t, err)
		assert.Equal(t, SyncStatusSyncing, task.Status)
		assert.NotNil(t, task.StartedAt)
	})

	t.Run("AlreadyStarted", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		_ = task.Start()

		err := task.Start()
		assert.Error(t, err)
		assert.Equal(t, ErrTaskAlreadyStarted, err)
	})

	t.Run("FromCompleted", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		task.Status = SyncStatusCompleted

		err := task.Start()
		assert.Error(t, err)
	})

	t.Run("FromFailed", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		task.Status = SyncStatusFailed

		err := task.Start()
		assert.Error(t, err)
	})
}

func TestSyncTask_Complete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		_ = task.Start()

		err := task.Complete()
		assert.NoError(t, err)
		assert.Equal(t, SyncStatusCompleted, task.Status)
		assert.NotNil(t, task.CompletedAt)
	})

	t.Run("NotSyncing", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")

		err := task.Complete()
		assert.Error(t, err)
		assert.Equal(t, ErrTaskNotSyncing, err)
	})

	t.Run("FromFailed", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		task.Status = SyncStatusFailed

		err := task.Complete()
		assert.Error(t, err)
	})
}

func TestSyncTask_Fail(t *testing.T) {
	task := NewSyncTask("1", "nginx", "amd64", "linux")
	_ = task.Start()

	testErr := errors.New("test error")
	err := task.Fail(testErr)

	assert.NoError(t, err)
	assert.Equal(t, SyncStatusFailed, task.Status)
	assert.Equal(t, testErr, task.Error)
	assert.NotNil(t, task.CompletedAt)
}

func TestSyncTask_DisplayName(t *testing.T) {
	tests := []struct {
		name string
		task *SyncTask
		want string
	}{
		{
			name: "WithArchAndOs",
			task: NewSyncTask("1", "nginx", "amd64", "linux"),
			want: "nginx (linux/amd64)",
		},
		{
			name: "WithoutArchAndOs",
			task: NewSyncTask("1", "nginx", "", ""),
			want: "nginx",
		},
		{
			name: "WithImageTag",
			task: NewSyncTask("1", "nginx:latest", "amd64", "linux"),
			want: "nginx:latest (linux/amd64)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.task.DisplayName())
		})
	}
}

func TestSyncTask_Duration(t *testing.T) {
	t.Run("NoTimesReturnsZero", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		assert.Equal(t, time.Duration(0), task.Duration())
	})

	t.Run("WithTimesReturnsDuration", func(t *testing.T) {
		task := NewSyncTask("1", "nginx", "amd64", "linux")
		start := time.Now()
		end := start.Add(5 * time.Second)
		task.StartedAt = &start
		task.CompletedAt = &end

		duration := task.Duration()
		assert.InDelta(t, 5*time.Second, duration, float64(time.Millisecond))
	})
}
