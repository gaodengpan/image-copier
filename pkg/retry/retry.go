package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts     int           // Maximum number of retry attempts
	InitialInterval time.Duration // Initial wait interval between retries
	MaxInterval     time.Duration // Maximum wait interval between retries
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:     3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
	}
}

// RetryableError is an error that can be retried
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError wraps an error as retryable
func NewRetryableError(err error) error {
	return &RetryableError{Err: err}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryErr *RetryableError
	return errors.As(err, &retryErr)
}

// Retry executes a function with exponential backoff retry
// Returns the last error if all attempts fail, or nil on success
func Retry(ctx context.Context, config *Config, fn func() error) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter
			backoff := calculateBackoff(attempt, config.InitialInterval, config.MaxInterval)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("retry canceled: %w", ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !IsRetryable(err) {
			return err
		}

		// Log retry attempt
		if attempt < config.MaxAttempts-1 {
			continue
		}
	}

	return fmt.Errorf("max retry attempts (%d) reached, last error: %w", config.MaxAttempts, lastErr)
}

// calculateBackoff calculates the backoff duration with exponential backoff and jitter
func calculateBackoff(attempt int, initialInterval, maxInterval time.Duration) time.Duration {
	// Exponential backoff: initialInterval * 2^(attempt-1)
	exponential := float64(initialInterval) * math.Pow(2, float64(attempt-1))

	// Add jitter: +/- 10% random variation
	jitter := exponential * 0.1 * (2*0.5 - 1) // -0.1 to +0.1
	backoff := float64(exponential) + jitter

	// Clamp to max interval
	if backoff > float64(maxInterval) {
		backoff = float64(maxInterval)
	}

	return time.Duration(backoff)
}