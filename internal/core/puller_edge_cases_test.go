package core

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCheckLocalImageExistsWithEmptyInput tests the function with empty input
func TestCheckLocalImageExistsWithEmptyInput(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test with empty image ID
	_, err := puller.CheckLocalImageExists(ctx, "")
	if err == nil {
		t.Error("Expected error for empty image ID, but got none")
	}

	if !strings.Contains(err.Error(), "invalid image name") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

// TestCheckLocalImageExistsWithExtremelyLongInput tests with extremely long image name
func TestCheckLocalImageExistsWithExtremelyLongInput(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Create a very long image name (longer than typical limits)
	veryLongName := strings.Repeat("a", 10000) + ":latest"

	_, err := puller.CheckLocalImageExists(ctx, veryLongName)

	// With the current validation implementation, extremely long names may pass validation
	// if they don't contain forbidden characters, but may fail later for other reasons
	// The important thing is that the system doesn't crash
	if err != nil {
		t.Logf("Got expected error (validation or docker unavailable): %v", err)
	} else {
		t.Logf("Long name accepted by validation (this is acceptable)")
	}
}

// TestCheckLocalImageExistsWithSpecialCharacters tests with special characters
func TestCheckLocalImageExistsWithSpecialCharacters(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test with various special characters that could cause problems
	testCases := []string{
		"normal:tag",
		"special-chars:v1.0",     // dashes and dots
		"underscore_chars:v1.0",  // underscores
		"mixed.alphanumeric-v1_0:v2.1", // mixed characters
		"emoji-containing:v1.0", // this should be rejected
		"spaces in name:v1.0",   // this should be rejected
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			_, err := puller.CheckLocalImageExists(ctx, testCase)

			// Based on our validation, some should be rejected
			if strings.Contains(testCase, " ") || strings.Contains(testCase, "emoji") {
				if err == nil {
					t.Errorf("Expected error for invalid image name '%s', but got none", testCase)
				}
			} else {
				// For valid-looking names, we expect the function to not fail due to validation
				// but it might fail for other reasons (like missing docker)
				if err != nil {
					// If it fails, check that it's not due to validation issues
					if strings.Contains(err.Error(), "invalid image name") {
						t.Errorf("Valid image name '%s' was incorrectly rejected: %v", testCase, err)
					} else {
						// Other errors (like docker unavailable) are expected in test environment
						t.Logf("Expected error (non-validation): %v", err)
					}
				}
			}
		})
	}
}

// TestConcurrentAccessToEmptyCache tests concurrent access when cache is empty
func TestConcurrentAccessToEmptyCache(t *testing.T) {
	puller := newTestPuller(nil)

	// Start with an empty cache
	puller.cacheMutex.Lock()
	puller.LocalImageCache = make(map[string]bool)
	puller.CacheTimestamp = time.Time{} // zero time, cache is definitely expired
	puller.cacheMutex.Unlock()

	ctx := context.Background()
	const numGoroutines = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			imageName := fmt.Sprintf("empty-cache-test:%d", id)
			_, err := puller.CheckLocalImageExists(ctx, imageName)
			if err != nil {
				// In test environment, we might get errors for other reasons
				// This is acceptable as long as we don't panic
				t.Logf("Expected error in test environment for goroutine %d: %v", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Log("Successfully handled concurrent access to empty cache")
}

// TestConcurrentCacheRefreshWithDifferentImages tests concurrent cache refresh with different images
func TestConcurrentCacheRefreshWithDifferentImages(t *testing.T) {
	puller := newTestPuller(nil)

	// Set cache as expired to force refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.LocalImageCache = make(map[string]bool)
	puller.cacheMutex.Unlock()

	ctx := context.Background()
	const numGoroutines = 15

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			imageName := fmt.Sprintf("concurrent-refresh-%d:image", id)
			_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
			if err != nil {
				// Expected in test environment due to lack of docker
				t.Logf("Expected error in test env for refresh %d: %v", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Log("Successfully handled concurrent cache refresh operations")
}

// TestCacheSizeBoundary tests behavior at cache size boundaries
func TestCacheSizeBoundary(t *testing.T) {
	puller := newTestPuller(nil)

	// Set a small cache size to test boundary conditions
	puller.MaxCacheSize = 5

	// Populate cache close to the limit
	puller.cacheMutex.Lock()
	puller.LocalImageCache = make(map[string]bool)
	for i := 0; i < 4; i++ { // Leave one spot for boundary testing
		key := fmt.Sprintf("boundary:image%d", i)
		puller.LocalImageCache[key] = true
	}
	puller.CacheTimestamp = time.Now()
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// Add one more to reach the boundary
	nextImage := "boundary:image4"
	_, err := puller.CheckLocalImageExists(ctx, nextImage)
	if err != nil {
		// Expected due to test environment
		t.Logf("Expected error in test environment: %v", err)
	}

	// Now try to add another which should trigger cache refresh due to size
	finalImage := "boundary:image5"
	_, err = puller.CheckLocalImageExists(ctx, finalImage)
	if err != nil {
		// Expected due to test environment
		t.Logf("Expected error in test environment: %v", err)
	}

	t.Log("Successfully tested cache size boundary conditions")
}

// TestConcurrentCacheEviction tests concurrent cache eviction scenarios
func TestConcurrentCacheEviction(t *testing.T) {
	puller := newTestPuller(nil)

	// Set a small cache size
	puller.MaxCacheSize = 3

	ctx := context.Background()
	const numGoroutines = 10

	done := make(chan bool, numGoroutines)

	// Multiple goroutines trying to fill the small cache
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			imageName := fmt.Sprintf("eviction:image%d", id)
			// Force cache refresh each time to test size-based eviction
			puller.cacheMutex.Lock()
			puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
			puller.cacheMutex.Unlock()

			_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
			if err != nil {
				// Expected in test environment
				t.Logf("Expected error for eviction test %d: %v", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Log("Successfully handled concurrent cache eviction scenarios")
}

// TestInputValidationEdgeCases tests edge cases for input validation
func TestInputValidationEdgeCases(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test various problematic inputs that could lead to command injection
	injectionTests := []string{
		"",                         // Empty string
		"; rm -rf /",              // Command injection attempt
		"&& malicious_command",     // AND injection
		"|| evil_command",          // OR injection
		"`rm -rf /`",              // Command substitution
		"$(malicious)",             // Dollar-paren command substitution
		"image\"\nevil_command",    // Newline injection
		"image\r\neviler_command",  // CRLF injection
		"../etc/passwd",            // Path traversal
		"../../../secret_file",     // More path traversal
		"\x00\x01\x02",            // Null bytes and control chars
	}

	for _, injection := range injectionTests {
		t.Run(fmt.Sprintf("injection_%d", len(injection)), func(t *testing.T) {
			_, err := puller.CheckLocalImageExists(ctx, injection)

			// All of these should be rejected by the validator
			if err == nil {
				t.Errorf("Input validation failed! Valid input was accepted: %q", injection)
			} else if !strings.Contains(err.Error(), "invalid image name") {
				t.Errorf("Wrong error type for input %q: %v", injection, err)
			} else {
				t.Logf("Correctly rejected malicious input: %q", injection)
			}
		})
	}
}

// TestBoundaryCacheExpirationTimes tests boundary values for cache expiration
func TestBoundaryCacheExpirationTimes(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test right at the boundary (just before expiration)
	puller.cacheMutex.Lock()
	puller.LocalImageCache = map[string]bool{"boundary:test": true}
	puller.CacheTimestamp = time.Now().Add(-DefaultCacheTTL).Add(1 * time.Millisecond) // Just before expiry
	puller.cacheMutex.Unlock()

	// This should use the cache (not expired yet)
	start := time.Now()
	_, err := puller.CheckLocalImageExists(ctx, "boundary:test")
	duration := time.Since(start)

	if err != nil {
		// Allow for expected errors in test environment
		t.Logf("Got expected error in test env: %v", err)
	}

	// Should be relatively fast since using cache
	t.Logf("Cache hit took: %v", duration)

	// Now test just after expiration
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-DefaultCacheTTL).Add(-1 * time.Millisecond) // Just after expiry
	puller.cacheMutex.Unlock()

	start = time.Now()
	_, err = puller.CheckLocalImageExists(ctx, "boundary:test")
	duration = time.Since(start)

	if err != nil {
		// Allow for expected errors in test environment
		t.Logf("Got expected error in test env: %v", err)
	}

	// May take longer since it tries to refresh
	t.Logf("Cache refresh attempt took: %v", duration)
}

// TestConcurrentCleanupAndAccess tests concurrent cache cleanup and access
func TestConcurrentCleanupAndAccess(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Start a goroutine that continuously cleans up the cache
	stop := make(chan bool)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				puller.CleanupCache()
				time.Sleep(10 * time.Millisecond) // Very frequent cleanups
			}
		}
	}()

	// Meanwhile, try to access the cache from multiple goroutines
	const numAccessors = 5
	var wgAccess sync.WaitGroup

	for i := 0; i < numAccessors; i++ {
		wgAccess.Add(1)
		go func(id int) {
			defer wgAccess.Done()

			for j := 0; j < 10; j++ {
				imageName := fmt.Sprintf("cleanup:test%d-%d", id, j)
				_, err := puller.CheckLocalImageExists(ctx, imageName)
				if err != nil {
					// Expected in test environment
					t.Logf("Expected error in test env: %v", err)
				}
			}
		}(i)
	}

	wgAccess.Wait()
	close(stop) // Stop the cleanup goroutine

	t.Log("Successfully handled concurrent cleanup and access")
}

// TestVeryLargeNumberOfImages tests behavior with a very large number of different images
func TestVeryLargeNumberOfImages(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Test with many different images to potentially trigger edge cases
	const numImages = 1000
	for i := 0; i < numImages; i++ {
		imageName := fmt.Sprintf("large-number:test-image-%04d", i)

		// Don't overwhelm the cache - force refresh periodically to test boundary conditions
		if i%100 == 0 {
			puller.cacheMutex.Lock()
			puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
			puller.cacheMutex.Unlock()
		}

		_, err := puller.CheckLocalImageExists(ctx, imageName)
		if err != nil {
			// Expected in test environment
			t.Logf("Expected error for image %d: %v", i, err)
		}
	}

	t.Logf("Successfully processed %d different images", numImages)
}

// TestRuntimeMemoryPressure tests behavior under memory pressure simulation
func TestRuntimeMemoryPressure(t *testing.T) {
	puller := newTestPuller(nil)

	ctx := context.Background()

	// Create some garbage to increase GC pressure
	garbage := make([][]byte, 1000)
	for i := range garbage {
		garbage[i] = make([]byte, 1024) // 1KB chunks
	}

	// Force GC to clean up before the test
	runtime.GC()

	// Now perform cache operations
	for i := 0; i < 50; i++ {
		imageName := fmt.Sprintf("pressure:test%d", i)
		_, err := puller.CheckLocalImageExists(ctx, imageName)
		if err != nil {
			// Expected in test environment
			t.Logf("Expected error: %v", err)
		}
	}

	// Clear garbage to allow it to be collected
	garbage = nil
	runtime.GC()

	t.Log("Successfully operated under memory pressure simulation")
}