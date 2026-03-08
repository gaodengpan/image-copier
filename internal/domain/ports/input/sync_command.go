package input

import (
	"context"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
)

// SyncCommandInput represents the input for the sync command
type SyncCommandInput struct {
	// Image sources
	Images       []string // List of image IDs to sync
	ManifestFile string   // Path to YAML manifest file

	// Image options
	Arch string // Architecture (e.g., amd64, arm64)
	Os   string // Operating system (e.g., linux)

	// Execution options
	Force       bool          // Force re-sync even if image exists
	DryRun      bool          // Show what would be done without executing
	WorkerCount int           // Number of concurrent workers
	Timeout     time.Duration // Overall timeout

	// Distribution options
	Targets        []string // Target names (empty = use default from config)
	SkipSync       bool     // Skip sync phase (only distribute)
	SkipDistribute bool     // Skip distribute phase (only sync)
}

// SyncPhaseResult represents the result of the sync phase (Phase 1)
type SyncPhaseResult struct {
	AlreadyExisted []*entities.SyncTask // Images already in staging registry
	NewlySynced    []*entities.SyncTask // Images that were newly synced
	Failed         []*entities.SyncTask // Images that failed to sync
	Errors         []error              // Errors encountered
}

// DistributePhaseResult represents the result of the distribute phase (Phase 2)
type DistributePhaseResult struct {
	Tasks        []*entities.DistributeTask // All distribution tasks
	SuccessCount int                        // Total successful distributions
	SkippedCount int                        // Total skipped distributions
	FailedCount  int                        // Total failed distributions
	Errors       []TargetError              // Errors by target
}

// TargetError represents an error for a specific target
type TargetError struct {
	ImageName  string
	TargetName string
	Error      error
}

// SyncCommandResult represents the combined result of both phases
type SyncCommandResult struct {
	SyncPhase       *SyncPhaseResult
	DistributePhase *DistributePhaseResult
	Duration        time.Duration
}

// SyncCommandUseCase defines the interface for the sync command use case
type SyncCommandUseCase interface {
	// Execute executes the full two-phase sync command
	Execute(ctx context.Context, input SyncCommandInput) (*SyncCommandResult, error)

	// SyncPhase executes only the sync phase (sync to staging registry)
	SyncPhase(ctx context.Context, input SyncCommandInput) (*SyncPhaseResult, error)

	// DistributePhase executes only the distribute phase (from staging registry to targets)
	DistributePhase(ctx context.Context, syncedImages []string, input SyncCommandInput) (*DistributePhaseResult, error)
}
