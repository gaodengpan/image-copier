package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig tests the default configuration
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialInterval)
	assert.Equal(t, 30*time.Second, config.MaxInterval)
}

// TestNewRetryableError tests creating a retryable error
func TestNewRetryableError(t *testing.T) {
	origErr := errors.New("original error")
	retryableErr := NewRetryableError(origErr)

	assert.True(t, IsRetryable(retryableErr))
	assert.Equal(t, origErr.Error(), retryableErr.Error())

	// Test unwrapping
	unwrapped := errors.Unwrap(retryableErr)
	assert.Equal(t, origErr, unwrapped)
}

// TestIsRetryable tests the retryable error checker
func TestIsRetryable(t *testing.T) {
	origErr := errors.New("original error")
	retryableErr := NewRetryableError(origErr)
	plainErr := errors.New("plain error")

	assert.True(t, IsRetryable(retryableErr))
	assert.False(t, IsRetryable(plainErr))
	assert.False(t, IsRetryable(nil))
}

// TestIsAuthError tests the authentication error checker
func TestIsAuthError(t *testing.T) {
	authErrors := []error{
		errors.New("authentication failed"),
		errors.New("unauthorized access"),
		errors.New("response code 401"),
		errors.New("status 403 forbidden"),
	}

	nonAuthErrors := []error{
		errors.New("connection timeout"),
		errors.New("network error"),
		errors.New("response code 200"),
	}

	for _, err := range authErrors {
		t.Run(err.Error(), func(t *testing.T) {
			assert.True(t, IsAuthError(err))
		})
	}

	for _, err := range nonAuthErrors {
		t.Run(err.Error(), func(t *testing.T) {
			assert.False(t, IsAuthError(err))
		})
	}

	assert.False(t, IsAuthError(nil))
}

// TestIsNotFoundError tests the not found error checker
func TestIsNotFoundError(t *testing.T) {
	notFoundErrors := []error{
		errors.New("resource not found"),
		errors.New("response code 404"),
	}

	nonNotFoundErrors := []error{
		errors.New("connection timeout"),
		errors.New("network error"),
		errors.New("response code 200"),
		errors.New("response code 401"),
	}

	for _, err := range notFoundErrors {
		t.Run(err.Error(), func(t *testing.T) {
			assert.True(t, IsNotFoundError(err))
		})
	}

	for _, err := range nonNotFoundErrors {
		t.Run(err.Error(), func(t *testing.T) {
			assert.False(t, IsNotFoundError(err))
		})
	}

	assert.False(t, IsNotFoundError(nil))
}

// TestCalculateBackoff tests the backoff calculation
func TestCalculateBackoff(t *testing.T) {
	initial := 1 * time.Second
	max := 30 * time.Second

	// First attempt (attempt 1) should be around initial interval
	backoff1 := calculateBackoff(1, initial, max)
	assert.True(t, backoff1 >= 900*time.Millisecond && backoff1 <= 1100*time.Millisecond)

	// Second attempt (attempt 2) should be around 2x initial with jitter
	backoff2 := calculateBackoff(2, initial, max)
	assert.True(t, backoff2 <= 2200*time.Millisecond) // 2s + jitter
	assert.True(t, backoff2 >= 1800*time.Millisecond) // 2s - jitter

	// Later attempts should cap at max interval
	backoffLate := calculateBackoff(10, initial, max)
	assert.True(t, backoffLate <= max)
}

// TestRetrySuccessOnFirstAttempt tests retry when function succeeds immediately
func TestRetrySuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return nil // Success on first try
	}

	err := Retry(ctx, config, fn)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// TestRetryEventualSuccess tests retry when function succeeds after some attempts
func TestRetryEventualSuccess(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(errors.New("temporary error"))
		}
		return nil // Success on third try
	}

	err := Retry(ctx, config, fn)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

// TestRetryMaxAttemptsReached tests when all attempts are exhausted with retryable errors
func TestRetryMaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	expectedErr := NewRetryableError(errors.New("temporary error"))
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := Retry(ctx, config, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retry attempts (3) reached")
	assert.Contains(t, err.Error(), "temporary error")
	assert.Equal(t, 3, callCount)
}

// TestRetryNonRetryableError tests when function returns non-retryable error
func TestRetryNonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	expectedErr := errors.New("non-retryable error")
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := Retry(ctx, config, fn)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, 1, callCount) // Should stop after first attempt
}

// TestRetryAuthError tests when function returns authentication error
func TestRetryAuthError(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	authErr := errors.New("unauthorized access")
	fn := func() error {
		callCount++
		return authErr
	}

	err := Retry(ctx, config, fn)
	assert.Equal(t, authErr, err)
	assert.Equal(t, 1, callCount) // Should stop after first attempt
}

// TestRetryNotFoundError tests when function returns not found error
func TestRetryNotFoundError(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	callCount := 0
	notFoundErr := errors.New("resource not found")
	fn := func() error {
		callCount++
		return notFoundErr
	}

	err := Retry(ctx, config, fn)
	assert.Equal(t, notFoundErr, err)
	assert.Equal(t, 1, callCount) // Should stop after first attempt
}

// TestRetryWithNilConfig tests retry with nil config (should use defaults)
func TestRetryWithNilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 2 {
			return NewRetryableError(errors.New("temporary error"))
		}
		return nil
	}

	err := Retry(ctx, nil, fn) // nil config should use defaults
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

// TestRetryWithContextCancellation tests retry with context cancellation
func TestRetryWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &Config{
		MaxAttempts:     5,
		InitialInterval: 1 * time.Second, // Long interval to test cancellation
		MaxInterval:     2 * time.Second,
	}

	// Cancel context immediately
	cancel()

	fn := func() error {
		return NewRetryableError(errors.New("temporary error"))
	}

	err := Retry(ctx, config, fn)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "retry canceled"))
}

// TestRetryErrorWrapping tests that errors are properly wrapped
func TestRetryErrorWrapping(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts:     2,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     1 * time.Second,
	}

	originalErr := NewRetryableError(errors.New("original error message"))
	fn := func() error {
		return originalErr
	}

	err := Retry(ctx, config, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retry attempts (2) reached")
	assert.Contains(t, err.Error(), "original error message")
}
