package output

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

// StagingRegistryConfig contains configuration for the staging registry
type StagingRegistryConfig struct {
	Host      string
	Namespace string
	Username  string
	Password  string
}

// DistributionOptions contains options for distributing an image
type DistributionOptions struct {
	// Source image information (from staging registry)
	SourceImageID      string
	SourceRegistryHost string
	SourceRegistryNS   string
	SourceRegistryUser string
	SourceRegistryPass string

	// Target information
	TargetName         string
	TargetType         value_objects.TargetType
	TargetRegistryHost string // Empty for docker target
	TargetRegistryUser string
	TargetRegistryPass string

	// Force re-distribution even if image exists
	Force bool
}

// ToSyncTargetOptions converts DistributionOptions to SyncTargetOptions
func (o DistributionOptions) ToSyncTargetOptions() SyncTargetOptions {
	return SyncTargetOptions{
		SourceImageID:          o.SourceImageID,
		SourceRegistryHost:     o.SourceRegistryHost,
		SourceRegistryNS:       o.SourceRegistryNS,
		SourceRegistryUsername: o.SourceRegistryUser,
		SourceRegistryPassword: o.SourceRegistryPass,
		TargetRegistryHost:     o.TargetRegistryHost,
		TargetRegistryUsername: o.TargetRegistryUser,
		TargetRegistryPassword: o.TargetRegistryPass,
	}
}

// DistributionStrategy defines the interface for distributing images to a target
type DistributionStrategy interface {
	// Distribute distributes an image from the staging registry to the target
	Distribute(ctx context.Context, opts DistributionOptions) error

	// ExistsInDistributionTarget checks if the image already exists in the target
	ExistsInDistributionTarget(ctx context.Context, opts DistributionOptions) (bool, error)

	// TargetType returns the type of target this strategy handles
	TargetType() value_objects.TargetType
}

// DistributeResult contains the results of distributing to all targets
type DistributeResult struct {
	Task    *entities.DistributeTask
	Results []entities.TargetResult
}

// MultiTargetDistributor defines the interface for distributing images to multiple targets
type MultiTargetDistributor interface {
	// DistributeToAll distributes an image to all specified targets
	DistributeToAll(
		ctx context.Context,
		task *entities.DistributeTask,
		targets []*value_objects.DistributionTarget,
		stagingConfig StagingRegistryConfig,
		force bool,
	) DistributeResult
}
