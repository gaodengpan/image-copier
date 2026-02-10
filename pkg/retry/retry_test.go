package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
)

// TestDefaultConfig tests the default configuration
func TestDefaultConfig(t *testing.T) {
	config := retry.DefaultConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}

	if config.InitialInterval != 1*time.Second {
		t.Errorf("Expected InitialInterval to be 1s, got %v", config.InitialInterval)
	}

	if config.MaxInterval != 30*time.Second {
		t.Errorf("Expected MaxInterval to be 30s, got %v", config.MaxInterval)
	}
}

// TestRetryableError tests the retryable error wrapping
func TestRetryableError(t *testing.T) {
	baseErr := errors.New("network error")
	retryErr := retry.NewRetryableError(baseErr)

	if retryErr == nil {
		t.Fatal("Expected non-nil retryable error")
	}

	if retryErr.Error() != "network error" {
		t.Errorf("Expected error message 'network error', got '%s'", retryErr.Error())
	}

	// Test Unwrap
	if errors.Unwrap(retryErr) != baseErr {
		t.Errorf("Expected unwrapped error to be base error")
	}
}

// TestIsRetryable tests the IsRetryable function
func TestIsRetryable(t *testing.T) {
	baseErr := errors.New("some error")
	retryErr := retry.NewRetryableError(baseErr)

	if !retry.IsRetryable(retryErr) {
		t.Error("Expected retryable error to be retryable")
	}

	if retry.IsRetryable(baseErr) {
		t.Error("Expected non-retryable error to not be retryable")
	}
}

// TestRetry_Success tests successful retry scenario
func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := &retry.Config{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	attempts := 0
	err := retry.Retry(ctx, config, func() error {
		attempts++
		if attempts < 2 {
			return retry.NewRetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestRetry_NonRetryableError tests non-retryable error scenario
func TestRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := retry.DefaultConfig()

	attempts := 0
	err := retry.Retry(ctx, config, func() error {
		attempts++
		return errors.New("permanent error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}

	if !errors.Is(err, errors.New("permanent error")) {
		// Error should not be wrapped for non-retryable errors
		if err.Error() != "permanent error" {
			t.Errorf("Expected permanent error, got %v", err)
		}
	}
}

// TestRetry_AllAttemptsFailed tests scenario where all attempts fail
func TestRetry_AllAttemptsFailed(t *testing.T) {
	ctx := context.Background()
	config := &retry.Config{
		MaxAttempts:     2,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
	}

	attempts := 0
	err := retry.Retry(ctx, config, func() error {
		attempts++
		return retry.NewRetryableError(errors.New("always fails"))
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestRetry_ContextCancellation tests context cancellation
func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &retry.Config{
		MaxAttempts:     10,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	attempts := 0
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	err := retry.Retry(ctx, config, func() error {
		attempts++
		return retry.NewRetryableError(errors.New("always fails"))
	})

	if err == nil {
		t.Error("Expected error from context cancellation, got nil")
	}

	// Should have made at most 2 attempts before cancellation
	if attempts > 2 {
		t.Errorf("Expected at most 2 attempts before cancellation, got %d", attempts)
	}
}

// TestRetry_NoErrorImmediate tests successful execution on first try
func TestRetry_NoErrorImmediate(t *testing.T) {
	ctx := context.Background()
	config := retry.DefaultConfig()

	attempts := 0
	err := retry.Retry(ctx, config, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestRetry_NilConfig tests with nil config (should use default)
func TestRetry_NilConfig(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	err := retry.Retry(ctx, nil, func() error {
		attempts++
		if attempts < 2 {
			return retry.NewRetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestRetry_Backoff increases with each retry attempt
func TestRetry_Backoff(t *testing.T) {
	ctx := context.Background()
	config := &retry.Config{
		MaxAttempts:     4,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
	}

	var timestamps []time.Time
	attempts := 0

	err := retry.Retry(ctx, config, func() error {
		attempts++
		timestamps = append(timestamps, time.Now())
		if attempts < 4 {
			return retry.NewRetryableError(errors.New("temporary failure"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(timestamps) != 4 {
		t.Fatalf("Expected 4 timestamps, got %d", len(timestamps))
	}

	// Check that intervals increase (with some tolerance for jitter)
	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1])
		minExpected := config.InitialInterval * time.Duration(1<<(i-1)) * 9 / 10 // 90% of expected (allowing jitter)
		maxExpected := config.MaxInterval

		if interval < minExpected {
			t.Errorf("Attempt %d interval %v is less than expected %v", i+1, interval, minExpected)
		}
		if interval > maxExpected {
			t.Errorf("Attempt %d interval %v is greater than max %v", i+1, interval, maxExpected)
		}
	}
}

// BenchmarkRetry successful retry
func BenchmarkRetry_Success(b *testing.B) {
	ctx := context.Background()
	config := retry.DefaultConfig()

	for i := 0; i < b.N; i++ {
		attempts := 0
		_ = retry.Retry(ctx, config, func() error {
			attempts++
			if attempts < 2 {
				return retry.NewRetryableError(errors.New("temporary failure"))
			}
			return nil
		})
	}
}

// BenchmarkRetry parallel retries
func BenchmarkRetry_Parallel(b *testing.B) {
	ctx := context.Background()
	config := retry.DefaultConfig()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			attempts := 0
			_ = retry.Retry(ctx, config, func() error {
				attempts++
				if attempts < 2 {
					return retry.NewRetryableError(errors.New("temporary failure"))
				}
				return nil
			})
		}
	})
}