package gateways

import (
	"context"
	"sync"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	sharederrors "github.com/gaodengpan/image-copier/internal/shared/errors"
)

// MultiTargetDistributor distributes images to multiple targets in parallel
type MultiTargetDistributor struct {
	strategies map[value_objects.TargetType]output.DistributionStrategy
	logger     output.Logger
}

// NewMultiTargetDistributor creates a new MultiTargetDistributor
func NewMultiTargetDistributor(
	dockerStrategy output.DistributionStrategy,
	registryStrategy output.DistributionStrategy,
	logger output.Logger,
) *MultiTargetDistributor {
	strategies := make(map[value_objects.TargetType]output.DistributionStrategy)
	if dockerStrategy != nil {
		strategies[value_objects.TargetTypeDocker] = dockerStrategy
	}
	if registryStrategy != nil {
		strategies[value_objects.TargetTypeRegistry] = registryStrategy
	}
	return &MultiTargetDistributor{
		strategies: strategies,
		logger:     logger,
	}
}

// DistributeToAll distributes an image to all targets specified in the task
func (d *MultiTargetDistributor) DistributeToAll(
	ctx context.Context,
	task *entities.DistributeTask,
	targets []*value_objects.DistributionTarget,
	stagingConfig output.StagingRegistryConfig,
	force bool,
) output.DistributeResult {
	results := make([]entities.TargetResult, len(targets))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, target := range targets {
		// Check context before starting each goroutine
		select {
		case <-ctx.Done():
			// Context cancelled, mark remaining targets as failed
			for j := i; j < len(targets); j++ {
				results[j] = entities.TargetResult{
					TargetName: targets[j].Name(),
					Error:      ctx.Err(),
				}
			}
			return output.DistributeResult{
				Task:    task,
				Results: results,
			}
		default:
		}

		wg.Add(1)
		go func(idx int, t *value_objects.DistributionTarget) {
			defer wg.Done()

			// Check context at the start of goroutine
			select {
			case <-ctx.Done():
				mu.Lock()
				results[idx] = entities.TargetResult{
					TargetName: t.Name(),
					Error:      ctx.Err(),
				}
				mu.Unlock()
				return
			default:
			}

			result := d.distributeToTarget(ctx, task, t, stagingConfig, force)

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, target)
	}

	wg.Wait()

	return output.DistributeResult{
		Task:    task,
		Results: results,
	}
}

// distributeToTarget distributes an image to a single target
func (d *MultiTargetDistributor) distributeToTarget(
	ctx context.Context,
	task *entities.DistributeTask,
	target *value_objects.DistributionTarget,
	stagingConfig output.StagingRegistryConfig,
	force bool,
) entities.TargetResult {
	result := entities.TargetResult{
		TargetName: target.Name(),
	}

	strategy, ok := d.strategies[target.Type()]
	if !ok {
		result.Error = ErrUnsupportedTargetType
		return result
	}

	opts := output.DistributionOptions{
		SourceImageID:      task.SourceImageID,
		SourceRegistryHost: stagingConfig.Host,
		SourceRegistryNS:   stagingConfig.Namespace,
		SourceRegistryUser: stagingConfig.Username,
		SourceRegistryPass: stagingConfig.Password,
		TargetName:         target.Name(),
		TargetType:         target.Type(),
		TargetRegistryHost: target.Host(),
		TargetRegistryUser: target.Username(),
		TargetRegistryPass: target.Password(),
		Force:              force,
	}

	// Check if image exists in target (skip if force is true)
	if !force {
		exists, err := strategy.ExistsInDistributionTarget(ctx, opts)
		if err != nil {
			// Log the error but continue with distribution attempt
			if d.logger != nil {
				d.logger.Warn("Failed to check existence in ", target.Name(), ": ", err)
			}
		} else if exists {
			result.Skipped = true
			if d.logger != nil {
				d.logger.Infof("Image %s already exists in %s, skipping", task.OriginalSource, target.Name())
			}
			return result
		}
	}

	// Execute distribution
	if d.logger != nil {
		d.logger.Infof("Distributing %s to %s", task.OriginalSource, target.Name())
	}
	if err := strategy.Distribute(ctx, opts); err != nil {
		result.Error = err
		if d.logger != nil {
			d.logger.Errorf("Failed to distribute %s to %s: %v", task.OriginalSource, target.Name(), err)
		}
		return result
	}

	result.Success = true
	if d.logger != nil {
		d.logger.Infof("Successfully distributed %s to %s", task.OriginalSource, target.Name())
	}
	return result
}

// ErrUnsupportedTargetType is returned when an unsupported target type is encountered
var ErrUnsupportedTargetType = sharederrors.NewDomainError("MultiTargetDistributor", "DistributeToTarget", "unsupported target type")
