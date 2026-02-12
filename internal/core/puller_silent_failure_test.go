package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestCheckLocalImagesFunctionErrorHandling verifies that getAllLocalImages properly handles and propagates errors
func TestCheckLocalImagesFunctionErrorHandling(t *testing.T) {
	// Create a puller instance
	puller := newTestPuller(nil)

	// Create a context that will definitely cause timeout in getAllLocalImages
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// First test direct call to getAllLocalImages
	_, err := puller.getAllLocalImages(ctx)
	if err == nil {
		t.Errorf("getAllLocalImages should fail with timeout context")
	} else {
		t.Logf("getAllLocalImages correctly failed with: %v", err)
	}
}

// TestCheckLocalImageExistsErrorPropagation verifies that errors from getAllLocalImages propagate properly through CheckLocalImageExists
func TestCheckLocalImageExistsErrorPropagation(t *testing.T) {
	puller := newTestPuller(nil)

	// Create a context that will definitely cause timeout in getAllLocalImages
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// First test direct call to getAllLocalImages
	_, getAllErr := puller.getAllLocalImages(ctx)
	if getAllErr == nil {
		t.Errorf("getAllLocalImages should fail with timeout context")
	}

	// Then test how this error propagates through checkLocalImageWithCacheRefresh
	// This should trigger cache refresh which calls getAllLocalImages
	_, checkErr := puller.checkLocalImageWithCacheRefresh(ctx, "test:image")

	// Check if the underlying error from getAllLocalImages is properly communicated
	if checkErr == nil {
		t.Errorf("Errors from getAllLocalImages should propagate through checkLocalImageWithCacheRefresh")
	} else {
		// Verify that the error message contains relevant information
		errStr := checkErr.Error()
		if !strings.Contains(errStr, "primary cache refresh failed") {
			t.Errorf("Error message should indicate primary cache refresh failure, got: %s", errStr)
		}
		t.Logf("Proper error propagation confirmed: %v", checkErr)
	}
}

// TestCheckCacheRefreshFallbackBehavior verifies that fallback mechanism in cache refresh works correctly when primary method fails
func TestCheckCacheRefreshFallbackBehavior(t *testing.T) {
	// Create a puller instance
	puller := newTestPuller(nil)

	ctx := context.Background()

	// The checkLocalImageWithCacheRefresh method has fallback logic that should execute when primary cache refresh fails
	// First, try to cause a failure in the main cache refresh path by using a timeout context
	failingCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	// This should fail the primary path and trigger fallback logic
	_, err := puller.checkLocalImageWithCacheRefresh(failingCtx, "test-image:latest")

	// Verify that the error from the failing path is properly handled by the fallback
	if err == nil {
		t.Error("Expected error when getAllLocalImages fails in checkLocalImageWithCacheRefresh")
	} else {
		// The error should indicate that both primary and fallback failed
		errStr := err.Error()
		if !strings.Contains(errStr, "fallback") && !strings.Contains(errStr, "individual check") {
			t.Errorf("Error should indicate fallback mechanism attempted, got: %s", errStr)
		}
		t.Logf("Fallback behavior correctly triggered: %v", err)
	}
}

// TestSilentFailuresInGetAllLocalImagesDetection verifies that silent failures in getAllLocalImages are properly detected
func TestSilentFailuresInGetAllLocalImagesDetection(t *testing.T) {
	puller := newTestPuller(nil)

	// We want to make sure that failures in getAllLocalImages are not silently ignored
	// Test with a context that will cause the function to timeout, causing a failure

	// Use a very short timeout to force failure
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Set cache to expired state to force refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second) // Expire cache
	puller.cacheMutex.Unlock()

	// Now call CheckLocalImageExists which should trigger cache refresh
	// and handle the error appropriately rather than silently failing
	_, checkErr := puller.CheckLocalImageExists(ctx, "test:image")

	// Verify that the error from getAllLocalImages is properly handled
	if checkErr == nil {
		t.Error("Errors from getAllLocalImages should be properly handled in CheckLocalImageExists")
	} else {
		t.Logf("Error properly handled in CheckLocalImageExists: %v", checkErr)

		// Verify the error contains information about the underlying failure
		errStr := checkErr.Error()
		if !strings.Contains(errStr, "failed to check local image") && !strings.Contains(errStr, "primary cache refresh failed") {
			t.Errorf("Error message should be informative about the failure, got: %s", errStr)
		}
	}
}

// TestGetAllLocalImagesWithValidContext verifies normal operation when getAllLocalImages succeeds
func TestGetAllLocalImagesWithValidContext(t *testing.T) {
	puller := newTestPuller(nil)

	// Use a context with reasonable timeout for successful operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This should succeed with a valid context
	_, err := puller.getAllLocalImages(ctx)

	// Since this involves external commands (docker), the test may fail in environments without docker
	// But if it fails, it should be with a meaningful error, not a silent one
	if err != nil {
		// Check that the error is descriptive
		errStr := err.Error()
		if !strings.Contains(errStr, "list local images") {
			t.Errorf("Error should indicate the specific operation that failed, got: %s", errStr)
		}
		t.Logf("Expected operational error (e.g., docker not available): %v", err)
	} else {
		t.Log("getAllLocalImages succeeded as expected")
	}
}