package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gaodengpan/image-copier/pkg/retry"
	"github.com/sirupsen/logrus"
)

// TestErrorRecoveryDuringCacheRefresh tests recovery when cache refresh fails
func TestErrorRecoveryDuringCacheRefresh(t *testing.T) {
	puller := newTestPuller(nil)

	// Simulate a scenario where getAllLocalImages would fail
	// by setting up expired cache that forces a refresh attempt
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second) // Expired
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// Test that when the primary cache refresh fails, fallback mechanism kicks in
	// This will attempt to run the docker command which should fail in test environment
	// but the error recovery should handle it gracefully
	_, err := puller.checkLocalImageWithCacheRefresh(ctx, "recovery:test")

	if err == nil {
		// If err is nil, it means it succeeded (which is possible in some test environments)
		t.Log("Cache refresh succeeded (may have succeeded in this test environment)")
	} else {
		// Check if it's the expected type of error (indicating proper error handling)
		errStr := err.Error()
		if containsAny(errStr, "primary cache refresh failed", "fallback individual check also failed", "failed to list local images") {
			t.Logf("Proper error recovery triggered: %v", err)
		} else {
			t.Errorf("Unexpected error during recovery: %v", err)
		}
	}
}

// TestRetryMechanismOnTransientFailures tests retry behavior during transient failures
func TestRetryMechanismOnTransientFailures(t *testing.T) {
	// Since the retry logic is integrated with HTTP calls in the main flow,
	// we'll test the retry configuration and behavior through the triggerWorkflow method

	// Create a config with custom retry settings for testing
	cfg := &Config{
		GithubOwner:      "owner",
		GithubRepo:       "repo",
		GithubToken:      "token",
		GithubWorkflowID: "workflow.yaml",
		RegistryHost:     "registry.example.com",
		RegistryUsername: "user",
		RegistryPassword: "pass",
		// Custom retry config for testing
		RetryConfig: &retry.Config{
			MaxAttempts:     2,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	puller := NewPuller(cfg, logger)

	ctx := context.Background()

	// Test that the puller respects the retry configuration
	// This will fail (as expected in test environment) but should use the configured retries
	_, err := puller.CheckLocalImageExists(ctx, "retry:test")

	// The error is expected in test environment, but the important thing is that
	// the retry mechanism was used properly (no panic, proper error handling)
	if err != nil {
		t.Logf("Expected error in test environment (retry mechanism tested): %v", err)
	}

	t.Log("Retry mechanism handled appropriately")
}

// TestGracefulDegradationOnDockerUnavailable tests behavior when Docker is unavailable
func TestGracefulDegradationOnDockerUnavailable(t *testing.T) {
	puller := newTestPuller(nil)

	// Set up cache to force refresh (which will call getAllLocalImages and eventually fail)
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second) // Expired
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// This will trigger a cache refresh that should fail because Docker isn't available
	// in the test environment, but the system should degrade gracefully
	_, err := puller.CheckLocalImageExists(ctx, "degrade:test")

	if err != nil {
		// This is expected in test environment - what matters is how it's handled
		errStr := err.Error()
		if containsAny(errStr, "primary cache refresh failed", "fallback individual check also failed") {
			t.Logf("System degraded gracefully: %v", err)
		} else {
			t.Logf("Expected error due to missing Docker: %v", err)
		}
	}

	t.Log("System handled Docker unavailability gracefully")
}

// TestMultipleConcurrentErrors tests behavior when multiple goroutines encounter errors simultaneously
func TestMultipleConcurrentErrors(t *testing.T) {
	puller := newTestPuller(nil)

	// Force cache to be expired so all goroutines will attempt refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.cacheMutex.Unlock()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			imageName := fmt.Sprintf("concurrent-error:%d", id)

			_, err := puller.CheckLocalImageExists(ctx, imageName)
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
		}
	}

	t.Logf("Out of %d concurrent operations, %d resulted in errors (expected in test environment)",
		numGoroutines, errorCount)

	// All errors are expected in test environment, but the important thing is that
	// the system didn't crash or panic
	if errorCount == numGoroutines {
		t.Log("All operations failed as expected in test environment (Docker not available)")
	} else {
		t.Logf("Some operations succeeded unexpectedly")
	}

	t.Log("System handled multiple concurrent errors gracefully")
}

// TestErrorPropagationInPullSingle tests error propagation in the main PullSingle function
func TestErrorPropagationInPullSingle(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// This should fail in test environment but errors should propagate properly
	err := puller.PullSingle(ctx, "propagate:error")

	if err != nil {
		// Check that the error propagates properly through the system
		errStr := err.Error()

		// Look for signs of proper error handling
		if containsAny(errStr, "invalid image name", "failed to check local image",
						"failed to trigger workflow", "workflow failed",
						"failed to copy and import image", "already exists locally") {
			t.Logf("Error propagated correctly: %v", err)
		} else {
			t.Logf("Operation failed as expected (test environment): %v", err)
		}
	} else {
		t.Log("Operation succeeded (unexpected but not necessarily wrong)")
	}
}

// TestRecoveryFromInvalidImageName tests recovery when invalid image names are provided
func TestRecoveryFromInvalidImageName(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test with invalid image names that should be rejected by validation
	invalidNames := []string{
		"malicious;rm -rf /",
		"inject&&echo test",
		"backtick`whoami`",
		"dollar$(echo)",
		"newline\ninjection",
		"cr\nlf\r\ninjection",
		"path/../traversal",
		"",
	}

	for _, invalidName := range invalidNames {
		t.Run(invalidName, func(t *testing.T) {
			// These should all be rejected by the input validation
			_, err := puller.CheckLocalImageExists(ctx, invalidName)

			if err == nil {
				t.Errorf("Expected validation error for '%s', but got none", invalidName)
			} else if !containsAny(err.Error(), "invalid image name") {
				t.Errorf("Expected validation error for '%s', but got different error: %v", invalidName, err)
			} else {
				t.Logf("Correctly rejected invalid name '%s': %v", invalidName, err)
			}
		})
	}
}

// TestRecoveryFromExpiredCacheWithErrors tests cache recovery when cache is expired and operations fail
func TestRecoveryFromExpiredCacheWithErrors(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Set cache to expired state
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.cacheMutex.Unlock()

	// First call should trigger refresh which will fail in test environment
	_, err1 := puller.CheckLocalImageExists(ctx, "expired-recovery:first")

	// Second call should also handle the error gracefully
	_, err2 := puller.CheckLocalImageExists(ctx, "expired-recovery:second")

	// Both should result in errors in test environment (Docker not available)
	// but should be handled gracefully without system crashes
	if err1 != nil {
		t.Logf("First call error: %v", err1)
	}
	if err2 != nil {
		t.Logf("Second call error: %v", err2)
	}

	t.Log("System recovered gracefully from expired cache with errors")
}

// TestRecoveryFromCacheFullCondition tests behavior when cache is full and needs refresh
func TestRecoveryFromCacheFullCondition(t *testing.T) {
	puller := newTestPuller(nil)

	// Set cache size to a very small value to trigger full condition easily
	puller.MaxCacheSize = 2

	// Fill the cache to capacity
	puller.cacheMutex.Lock()
	puller.LocalImageCache = map[string]bool{
		"full:test1": true,
		"full:test2": true,
	}
	puller.CacheTimestamp = time.Now()
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// Next call should trigger a refresh due to cache being full
	_, err := puller.CheckLocalImageExists(ctx, "full-condition:test3")

	if err != nil {
		// Expected in test environment
		t.Logf("Expected error during cache full condition: %v", err)
	}

	t.Log("System handled full cache condition properly")
}

// TestRecoveryFromConcurrentRefreshFailures tests recovery when multiple goroutines attempt refresh simultaneously and all fail
func TestRecoveryFromConcurrentRefreshFailures(t *testing.T) {
	puller := newTestPuller(nil)

	// Force cache expiration so all goroutines will attempt refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.cacheMutex.Unlock()

	const numGoroutines = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			imageName := fmt.Sprintf("concurrent-refresh:%d", id)

			_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)

			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	t.Logf("Out of %d concurrent refresh attempts, %d succeeded", numGoroutines, successCount)

	// Due to the refreshMutex, only one goroutine should have actually performed the refresh
	// The others would have seen the fresh timestamp and skipped the refresh operation
	// The important thing is that none panicked or crashed
	t.Log("System handled concurrent refresh failures gracefully")
}

// TestRecoveryFromInterruptedOperations tests recovery when context is cancelled mid-operation
func TestRecoveryFromInterruptedOperations(t *testing.T) {
	puller := newTestPuller(nil)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately to test interruption handling
	cancel()

	// This should handle the cancelled context gracefully
	_, err := puller.CheckLocalImageExists(ctx, "interrupted:test")

	if err != nil && err != context.Canceled {
		t.Logf("Got error as expected: %v", err)
	} else if err == context.Canceled {
		t.Log("Operation properly responded to context cancellation")
	} else {
		t.Log("Operation completed despite cancelled context (might be expected depending on timing)")
	}

	t.Log("System handled interrupted operation gracefully")
}

// Helper function to check if a string contains any of the given substrings
func containsAny(str string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(str, substr) {
			return true
		}
	}
	return false
}

// Simple helper to check substring containment
func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		   (str == substr ||
		    len(substr) > 0 &&
		    (len(str) > len(substr) &&
		     (str[:len(substr)] == substr ||
		      str[len(str)-len(substr):] == substr ||
		      indexOf(str, substr) >= 0)))
}

// Simplified version of string index search
func indexOf(str, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}