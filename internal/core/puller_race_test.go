package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestCheckLocalImageExistsForRaceConditions verifies thread safety of CheckLocalImageExists under concurrent access
func TestCheckLocalImageExistsForRaceConditions(t *testing.T) {
	// Note: This test should be run with 'go test -race' to detect race conditions
	puller := newTestPuller(nil)

	// Initialize with some cache data
	puller.LocalImageCache = map[string]bool{
		"test:image1": true,
		"test:image2": false,
	}
	puller.CacheTimestamp = time.Now()

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Multiple goroutines trying to check and modify cache simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()

			imageName := "test:image" + string(rune('0'+j%3)) // Cycle through 3 images
			ctx := context.Background()

			// This should be safe due to proper synchronization in the implementation
			_, err := puller.CheckLocalImageExists(ctx, imageName)
			if err != nil {
				t.Logf("Error checking local image (may be expected): %v", err)
			}
		}(i)
	}

	wg.Wait()

	// If we reached this point without panics, basic thread safety is maintained
	// The race detector will report any actual race conditions
	t.Log("Completed CheckLocalImageExists race condition test - check race detector output")
}

// TestCacheRefreshForRaceConditions verifies thread safety of cache refresh mechanism under concurrent access
func TestCacheRefreshForRaceConditions(t *testing.T) {
	// Note: This test should be run with 'go test -race' to detect race conditions
	puller := newTestPuller(nil)

	// Force cache to expire by setting timestamp in the past
	pastTime := time.Now().Add(-60 * time.Second) // Past expiration time
	puller.CacheTimestamp = pastTime
	puller.LocalImageCache = make(map[string]bool)

	const numGoroutines = 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()

			imageName := "race:test" + string(rune('0'+j))
			ctx := context.Background()

			// Multiple goroutines calling this will trigger cache refresh concurrently
			// The implementation should handle this safely with refreshMutex
			_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
			if err != nil {
				t.Logf("Error during cache refresh (may be expected): %v", err)
			}
		}(i)
	}

	wg.Wait()

	// If we reached this point without panics, cache refresh synchronization is working
	// The race detector will report any actual race conditions
	t.Log("Completed CacheRefresh race condition test - check race detector output")
}

// TestConcurrentCacheAccess verifies thread safety of cache operations under mixed read/write access
func TestConcurrentCacheAccess(t *testing.T) {
	// Note: This test should be run with 'go test -race' to detect race conditions
	puller := newTestPuller(nil)

	// Initialize cache with some data
	puller.LocalImageCache = map[string]bool{
		"busybox:latest": true,
		"alpine:latest":  false,
	}
	puller.CacheTimestamp = time.Now()

	const numReaders = 5
	const numWriters = 3
	var wg sync.WaitGroup

	// Concurrent readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			ctx := context.Background()
			for j := 0; j < 10; j++ {
				imageName := "busybox:latest"
				_, err := puller.CheckLocalImageExists(ctx, imageName)
				if err != nil {
					t.Logf("Reader %d: Error checking image: %v", readerID, err)
				}
			}
		}(i)
	}

	// Concurrent writers (triggering cache refresh)
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			ctx := context.Background()
			for j := 0; j < 5; j++ {
				imageName := "new:image" + string(rune('0'+writerID+j))
				// Set past timestamp to force cache refresh
				puller.cacheMutex.Lock()
				puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
				puller.cacheMutex.Unlock()

				_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
				if err != nil {
					t.Logf("Writer %d: Error refreshing cache: %v", writerID, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// If we reached this point without panics, concurrent cache access is synchronized
	// The race detector will report any actual race conditions
	t.Log("Completed ConcurrentCacheAccess test - check race detector output")
}

// TestRaceDetectionWithGoTest runs with go test -race to detect actual race conditions
func TestRaceDetectionWithGoTest(t *testing.T) {
	// This test is meant to be run with the -race flag
	// It will catch actual race conditions in the implementation

	puller := newTestPuller(nil)

	// Set up initial state
	puller.LocalImageCache = map[string]bool{
		"test:image": true,
	}
	puller.CacheTimestamp = time.Now()

	var wg sync.WaitGroup
	const goroutines = 20

	// Run multiple goroutines accessing the cache
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			imageName := "test:image"

			for j := 0; j < 10; j++ {
				// Alternate between reading and forcing refresh
				if j%2 == 0 {
					_, err := puller.CheckLocalImageExists(ctx, imageName)
					if err != nil {
						t.Logf("Goroutine %d: Error reading: %v", id, err)
					}
				} else {
					// Force cache refresh by setting expired time
					puller.cacheMutex.Lock()
					puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
					puller.cacheMutex.Unlock()

					_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
					if err != nil {
						t.Logf("Goroutine %d: Error refreshing: %v", id, err)
					}
				}

				// Small delay to allow other goroutines to interleave
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// If running with -race flag, race detector will report any issues
	// If no races are detected by the race detector, the test passes
	t.Log("Completed comprehensive race condition test - check race detector output")
}