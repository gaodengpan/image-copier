// Package concurrency provides utilities for concurrent execution with semaphore-based rate limiting.
package concurrency

import (
	"context"
	"sync"
)

// Processor defines a function that processes an item and returns a result or error.
type Processor[T any, R any] func(ctx context.Context, item T) (R, error)

// Result holds the outcome of processing a single item.
type Result[R any] struct {
	Value R
	Error error
	Index int
}

// ParallelForEach processes items in parallel with semaphore-limited concurrency.
// It returns a channel that emits results as they complete, allowing for streaming processing.
//
// Example usage:
//
//	results := concurrency.ParallelForEach(ctx, items, workerCount, func(ctx context.Context, item string) (string, error) {
//	    return processItem(item)
//	})
//	for result := range results {
//	    if result.Error != nil {
//	        log.Error(result.Error)
//	    }
//	}
func ParallelForEach[T any, R any](
	ctx context.Context,
	items []T,
	workerCount int,
	processor Processor[T, R],
) <-chan Result[R] {
	resultChan := make(chan Result[R], len(items))

	go func() {
		defer close(resultChan)

		if len(items) == 0 {
			return
		}

		sem := make(chan struct{}, workerCount)
		var wg sync.WaitGroup

		for i, item := range items {
			// Check for context cancellation before starting new work
			select {
			case <-ctx.Done():
				return
			default:
			}

			wg.Add(1)
			go func(idx int, it T) {
				defer wg.Done()

				// Acquire semaphore
				sem <- struct{}{}
				defer func() { <-sem }()

				// Check context again after acquiring semaphore
				select {
				case <-ctx.Done():
					resultChan <- Result[R]{Index: idx, Error: ctx.Err()}
					return
				default:
				}

				// Process the item
				value, err := processor(ctx, it)
				resultChan <- Result[R]{
					Value: value,
					Error: err,
					Index: idx,
				}
			}(i, item)
		}

		wg.Wait()
	}()

	return resultChan
}

// ParallelForEachSimple is a simpler version that doesn't return values,
// useful for fire-and-forget operations where you only care about errors.
//
// Example usage:
//
//	errs := concurrency.ParallelForEachSimple(ctx, items, workerCount, func(ctx context.Context, item string) error {
//	    return processItem(item)
//	})
//	for _, err := range errs {
//	    if err != nil {
//	        log.Error(err)
//	    }
//	}
func ParallelForEachSimple[T any](
	ctx context.Context,
	items []T,
	workerCount int,
	processor func(ctx context.Context, item T) error,
) []error {
	if len(items) == 0 {
		return nil
	}

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	for i, item := range items {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return []error{ctx.Err()}
		default:
		}

		wg.Add(1)
		go func(idx int, it T) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context after acquiring semaphore
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := processor(ctx, it); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(i, item)
	}

	wg.Wait()
	return errors
}

// CollectResults collects all results from a result channel into a slice.
// This is useful when you need all results before proceeding.
func CollectResults[R any](resultChan <-chan Result[R]) []Result[R] {
	results := make([]Result[R], 0)
	for result := range resultChan {
		results = append(results, result)
	}
	return results
}

// WorkerPool provides a reusable worker pool for processing items.
// It's useful when you have multiple batches of work to process with the same concurrency limit.
type WorkerPool struct {
	workerCount int
	sem         chan struct{}
}

// NewWorkerPool creates a new worker pool with the specified concurrency limit.
func NewWorkerPool(workerCount int) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		sem:         make(chan struct{}, workerCount),
	}
}

// Execute runs a function within the pool's concurrency limit.
// It blocks until a worker slot is available.
func (p *WorkerPool) Execute(fn func() error) error {
	p.sem <- struct{}{}
	defer func() { <-p.sem }()
	return fn()
}

// WorkerCount returns the configured worker count.
func (p *WorkerPool) WorkerCount() int {
	return p.workerCount
}