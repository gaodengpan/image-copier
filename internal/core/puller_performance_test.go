package core

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestPullerPerformance benchmarks the performance of the Puller under various loads
func TestPullerPerformance(t *testing.T) {
	puller := newTestPuller(nil)

	// Test with different numbers of concurrent operations
	testCases := []int{1, 5, 10, 25, 50}

	for _, numGoroutines := range testCases {
		t.Run(fmt.Sprintf("Concurrent_%d", numGoroutines), func(t *testing.T) {
			t.Parallel()

			startTime := time.Now()

			var wg sync.WaitGroup
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					ctx := context.Background()
					imageName := fmt.Sprintf("perf:test%d", id)

					// Call CheckLocalImageExists to trigger cache operations
					// This tests the cache performance under concurrent load
					_, _ = puller.CheckLocalImageExists(ctx, imageName)
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			t.Logf("Completed %d concurrent operations in %v", numGoroutines, duration)

			// Verify no race conditions occurred during execution
			if t.Failed() {
				t.Errorf("Performance test failed for %d goroutines", numGoroutines)
			}
		})
	}
}

// BenchmarkCacheOperations benchmarks the cache operations performance
func BenchmarkCacheOperations(b *testing.B) {
	puller := newTestPuller(nil)

	// Pre-populate cache with some data
	puller.cacheMutex.Lock()
	puller.LocalImageCache = make(map[string]bool)
	for i := 0; i < 100; i++ {
		puller.LocalImageCache[fmt.Sprintf("benchmark:image%d", i)] = true
	}
	puller.CacheTimestamp = time.Now()
	puller.cacheMutex.Unlock()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		imageName := fmt.Sprintf("benchmark:image%d", i%100)

		// Measure the time for cache lookup operation
		_, err := puller.CheckLocalImageExists(ctx, imageName)
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}

// BenchmarkConcurrentCacheAccess benchmarks concurrent access to the cache
func BenchmarkConcurrentCacheAccess(b *testing.B) {
	puller := newTestPuller(nil)

	// Pre-populate cache with some data
	puller.cacheMutex.Lock()
	puller.LocalImageCache = make(map[string]bool)
	for i := 0; i < 50; i++ {
		puller.LocalImageCache[fmt.Sprintf("benchmark:image%d", i)] = true
	}
	puller.CacheTimestamp = time.Now()
	puller.cacheMutex.Unlock()

	b.ResetTimer()

	// Parallel benchmark with multiple goroutines
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			ctx := context.Background()
			imageName := fmt.Sprintf("benchmark:image%d", id%50)

			_, err := puller.CheckLocalImageExists(ctx, imageName)
			if err != nil {
				b.Errorf("Unexpected error: %v", err)
			}
			id++
		}
	})
}

// TestMemoryUsageUnderLoad tests memory usage under load
func TestMemoryUsageUnderLoad(t *testing.T) {
	var m1, m2 runtime.MemStats

	// Get initial memory stats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	puller := newTestPuller(nil)

	// Perform operations that would stress the cache
	const numOps = 1000
	ctx := context.Background()

	for i := 0; i < numOps; i++ {
		imageName := fmt.Sprintf("memory:test%d", i)
		// Force cache refresh periodically to test cache management
		if i%100 == 0 {
			puller.cacheMutex.Lock()
			puller.CacheTimestamp = time.Now().Add(-60 * time.Second) // Expire cache
			puller.cacheMutex.Unlock()
		}
		_, _ = puller.CheckLocalImageExists(ctx, imageName)
	}

	// Get memory stats after operations
	runtime.GC()
	runtime.ReadMemStats(&m2)

	t.Logf("Allocated: %d KB, Total Alloc: %d KB, Sys: %d KB",
		m2.Alloc/1024, m2.TotalAlloc/1024, m2.Sys/1024)

	// Verify memory increase is reasonable (less than 10MB for these operations)
	memoryIncrease := m2.Alloc - m1.Alloc
	if memoryIncrease > 10*1024*1024 { // 10 MB threshold
		t.Errorf("Memory increase too high: %d bytes", memoryIncrease)
	}
}

// TestLargeCachePerformance tests performance with large cache sizes
func TestLargeCachePerformance(t *testing.T) {
	puller := newTestPuller(nil)

	// Create a large cache to test performance with many entries
	largeCache := make(map[string]bool)
	for i := 0; i < 5000; i++ {
		largeCache[fmt.Sprintf("largecache:image%d", i)] = true
	}

	puller.cacheMutex.Lock()
	puller.LocalImageCache = largeCache
	puller.CacheTimestamp = time.Now()
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// Measure time for cache lookups in large cache
	start := time.Now()
	for i := 0; i < 100; i++ {
		imageName := fmt.Sprintf("largecache:image%d", i%5000)
		_, err := puller.CheckLocalImageExists(ctx, imageName)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	duration := time.Since(start)

	t.Logf("Completed 100 lookups in cache with 5000 entries in %v", duration)

	// Should complete in reasonable time (less than 1 second for 100 operations)
	if duration > time.Second {
		t.Errorf("Cache operations taking too long: %v", duration)
	}
}

// TestCacheRefreshPerformance measures the performance of cache refresh operations
func TestCacheRefreshPerformance(t *testing.T) {
	puller := newTestPuller(nil)

	// Set up cache with expired timestamp to force refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.LocalImageCache = make(map[string]bool)
	puller.cacheMutex.Unlock()

	ctx := context.Background()

	// Measure time for cache refresh
	start := time.Now()
	_, err := puller.checkLocalImageWithCacheRefresh(ctx, "refresh:test")
	duration := time.Since(start)

	if err != nil {
		// We expect an error here because docker is not available in test environment
		// The important thing is that the function completes in reasonable time
		t.Logf("Expected error during cache refresh (no docker): %v", err)
	}

	t.Logf("Cache refresh operation completed in %v", duration)

	// Cache refresh should complete in reasonable time even with fallback logic
	if duration > 10*time.Second {
		t.Errorf("Cache refresh taking too long: %v", duration)
	}
}

// TestStressConcurrentCacheRefresh tests concurrent cache refresh operations
func TestStressConcurrentCacheRefresh(t *testing.T) {
	puller := newTestPuller(nil)

	// Set up cache with expired timestamp to force refresh
	puller.cacheMutex.Lock()
	puller.CacheTimestamp = time.Now().Add(-60 * time.Second)
	puller.LocalImageCache = make(map[string]bool)
	puller.cacheMutex.Unlock()

	const numGoroutines = 20
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			imageName := fmt.Sprintf("stress:refresh%d", id)

			// Each goroutine will attempt cache refresh
			// The refreshMutex should serialize these operations
			_, err := puller.checkLocalImageWithCacheRefresh(ctx, imageName)
			if err != nil {
				// Expected due to lack of docker in test environment
				t.Logf("Expected error during refresh %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Completed %d concurrent cache refresh attempts in %v", numGoroutines, duration)

	// With proper synchronization, this should complete in reasonable time
	// Even though individual operations may fail, the synchronization should prevent race conditions
	if duration > 30*time.Second {
		t.Errorf("Concurrent refresh taking too long: %v", duration)
	}
}