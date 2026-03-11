package concurrency

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelForEach(t *testing.T) {
	t.Run("processes all items", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5}
		var processedCount int32

		results := ParallelForEach(context.Background(), items, 2, func(ctx context.Context, item int) (string, error) {
			atomic.AddInt32(&processedCount, 1)
			return "result", nil
		})

		collected := CollectResults(results)
		assert.Len(t, collected, 5)
		assert.Equal(t, int32(5), processedCount)
	})

	t.Run("respects worker count", func(t *testing.T) {
		items := make([]int, 10)
		for i := range items {
			items[i] = i
		}

		var maxConcurrent int32
		var currentConcurrent int32

		results := ParallelForEach(context.Background(), items, 3, func(ctx context.Context, item int) (string, error) {
			cur := atomic.AddInt32(&currentConcurrent, 1)
			defer atomic.AddInt32(&currentConcurrent, -1)

			// Track max concurrent
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if cur <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, cur) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			return "result", nil
		})

		CollectResults(results)
		assert.LessOrEqual(t, maxConcurrent, int32(3))
	})

	t.Run("handles errors", func(t *testing.T) {
		items := []int{1, 2, 3}
		expectedErr := errors.New("test error")

		results := ParallelForEach(context.Background(), items, 2, func(ctx context.Context, item int) (string, error) {
			if item == 2 {
				return "", expectedErr
			}
			return "ok", nil
		})

		collected := CollectResults(results)
		assert.Len(t, collected, 3)

		errorCount := 0
		for _, r := range collected {
			if r.Error != nil {
				errorCount++
				assert.ErrorIs(t, r.Error, expectedErr)
			}
		}
		assert.Equal(t, 1, errorCount)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		items := []int{1, 2, 3, 4, 5}

		var processedCount int32

		// Cancel after first item
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		results := ParallelForEach(ctx, items, 1, func(ctx context.Context, item int) (string, error) {
			atomic.AddInt32(&processedCount, 1)
			time.Sleep(30 * time.Millisecond)
			return "result", nil
		})

		CollectResults(results)
		assert.Less(t, processedCount, int32(5))
	})

	t.Run("handles empty input", func(t *testing.T) {
		items := []int{}

		results := ParallelForEach(context.Background(), items, 2, func(ctx context.Context, item int) (string, error) {
			return "result", nil
		})

		collected := CollectResults(results)
		assert.Empty(t, collected)
	})
}

func TestParallelForEachSimple(t *testing.T) {
	t.Run("processes all items", func(t *testing.T) {
		items := []string{"a", "b", "c"}
		var processedCount int32

		errs := ParallelForEachSimple(context.Background(), items, 2, func(ctx context.Context, item string) error {
			atomic.AddInt32(&processedCount, 1)
			return nil
		})

		assert.Empty(t, errs)
		assert.Equal(t, int32(3), processedCount)
	})

	t.Run("collects errors", func(t *testing.T) {
		items := []int{1, 2, 3}

		errs := ParallelForEachSimple(context.Background(), items, 2, func(ctx context.Context, item int) error {
			if item%2 == 0 {
				return errors.New("even number error")
			}
			return nil
		})

		assert.Len(t, errs, 1)
	})

	t.Run("handles empty input", func(t *testing.T) {
		items := []int{}

		errs := ParallelForEachSimple(context.Background(), items, 2, func(ctx context.Context, item int) error {
			return nil
		})

		assert.Nil(t, errs)
	})
}

func TestWorkerPool(t *testing.T) {
	t.Run("limits concurrency", func(t *testing.T) {
		pool := NewWorkerPool(3)
		assert.Equal(t, 3, pool.WorkerCount())

		var maxConcurrent int32
		var currentConcurrent int32

		for i := 0; i < 10; i++ {
			go func() {
				pool.Execute(func() error {
					cur := atomic.AddInt32(&currentConcurrent, 1)
					defer atomic.AddInt32(&currentConcurrent, -1)

					for {
						max := atomic.LoadInt32(&maxConcurrent)
						if cur <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, cur) {
							break
						}
					}

					time.Sleep(10 * time.Millisecond)
					return nil
				})
			}()
		}

		// Wait a bit for all goroutines to complete
		time.Sleep(100 * time.Millisecond)
		assert.LessOrEqual(t, maxConcurrent, int32(3))
	})

	t.Run("returns errors from executed functions", func(t *testing.T) {
		pool := NewWorkerPool(2)
		expectedErr := errors.New("test error")

		err := pool.Execute(func() error {
			return expectedErr
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestResult(t *testing.T) {
	t.Run("stores value and index", func(t *testing.T) {
		result := Result[string]{
			Value: "test",
			Index: 5,
		}
		assert.Equal(t, "test", result.Value)
		assert.Equal(t, 5, result.Index)
		assert.Nil(t, result.Error)
	})
}