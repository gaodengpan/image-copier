package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/stretchr/testify/assert"
)

func TestNewDistributeTask(t *testing.T) {
	targets := []string{"docker", "my-registry"}
	task := NewDistributeTask("nginx:latest", "nginx:latest", "amd64", "linux", targets)

	assert.Equal(t, "nginx:latest", task.SourceImageID)
	assert.Equal(t, "nginx:latest", task.OriginalSource)
	assert.Equal(t, "amd64", task.Arch)
	assert.Equal(t, "linux", task.Os)
	assert.Equal(t, targets, task.Targets)
	assert.Equal(t, value_objects.TaskStatusPending, task.Status)
	assert.NotNil(t, task.Results)
	assert.Empty(t, task.Results)
}

func TestDistributeTask_Start(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		err := task.Start()

		assert.NoError(t, err)
		assert.Equal(t, value_objects.TaskStatusSyncing, task.Status)
		assert.NotNil(t, task.StartedAt)
	})

	t.Run("AlreadyStarted", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		_ = task.Start()

		err := task.Start()
		assert.Error(t, err)
		assert.Equal(t, ErrDistributeTaskAlreadyStarted, err)
	})
}

func TestDistributeTask_Complete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		_ = task.Start()

		err := task.Complete()
		assert.NoError(t, err)
		assert.Equal(t, value_objects.TaskStatusCompleted, task.Status)
		assert.NotNil(t, task.CompletedAt)
	})

	t.Run("NotDistributing", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})

		err := task.Complete()
		assert.Error(t, err)
		assert.Equal(t, ErrDistributeTaskNotDistributing, err)
	})
}

func TestDistributeTask_Fail(t *testing.T) {
	task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
	_ = task.Start()

	testErr := errors.New("test error")
	task.Fail(testErr)

	assert.Equal(t, value_objects.TaskStatusFailed, task.Status)
	assert.Equal(t, testErr, task.Error)
	assert.NotNil(t, task.CompletedAt)
}

func TestDistributeTask_AddResult(t *testing.T) {
	task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})

	task.AddResult(TargetResult{TargetName: "docker", Success: true})
	task.AddResult(TargetResult{TargetName: "registry", Success: false, Error: errors.New("failed")})

	assert.Len(t, task.Results, 2)
	assert.True(t, task.Results[0].Success)
	assert.False(t, task.Results[1].Success)
}

func TestDistributeTask_HasErrors(t *testing.T) {
	t.Run("NoErrors", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		task.AddResult(TargetResult{TargetName: "docker", Success: true})
		assert.False(t, task.HasErrors())
	})

	t.Run("WithResultError", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		task.AddResult(TargetResult{TargetName: "docker", Error: errors.New("failed")})
		assert.True(t, task.HasErrors())
	})

	t.Run("WithTaskError", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		task.Error = errors.New("task error")
		assert.True(t, task.HasErrors())
	})
}

func TestDistributeTask_Counts(t *testing.T) {
	task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker", "reg1", "reg2"})
	task.AddResult(TargetResult{TargetName: "docker", Success: true})
	task.AddResult(TargetResult{TargetName: "reg1", Skipped: true})
	task.AddResult(TargetResult{TargetName: "reg2", Error: errors.New("failed")})

	success, skipped, failed := task.Counts()
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, skipped)
	assert.Equal(t, 1, failed)

	assert.Equal(t, 1, task.SuccessCount())
	assert.Equal(t, 1, task.SkippedCount())
	assert.Equal(t, 1, task.FailedCount())
}

func TestDistributeTask_DisplayName(t *testing.T) {
	tests := []struct {
		name string
		task *DistributeTask
		want string
	}{
		{
			name: "WithArchAndOs",
			task: NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"}),
			want: "nginx (linux/amd64)",
		},
		{
			name: "WithoutArchAndOs",
			task: NewDistributeTask("nginx", "nginx", "", "", []string{"docker"}),
			want: "nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.task.DisplayName())
		})
	}
}

func TestDistributeTask_Duration(t *testing.T) {
	t.Run("NoTimesReturnsZero", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		assert.Equal(t, time.Duration(0), task.Duration())
	})

	t.Run("WithTimesReturnsDuration", func(t *testing.T) {
		task := NewDistributeTask("nginx", "nginx", "amd64", "linux", []string{"docker"})
		start := time.Now()
		end := start.Add(5 * time.Second)
		task.StartedAt = &start
		task.CompletedAt = &end

		duration := task.Duration()
		assert.InDelta(t, 5*time.Second, duration, float64(time.Millisecond))
	})
}
