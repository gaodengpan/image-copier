package use_cases

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
)

type PullStage int

const (
	StageCheckLocal PullStage = iota
	StageCheckRegistry
	StageTriggerWorkflow
	StageWaitWorkflow
	StageDownloadImage
	StageLoadImage
)

type PullSingleImageInput struct {
	ImageID      string
	RegistryHost string
	RegistryUser string
	RegistryPass string
	RegistryNS   string
	RegistryArch string
	RegistryOs   string
	Force        bool
	DryRun       bool
}

type PullSingleImageOutput struct {
	Skipped bool
	DryRun  bool
	Error   error
}

type PullSingleImageUseCase interface {
	Execute(ctx context.Context, input PullSingleImageInput) (PullSingleImageOutput, error)
}

type SyncImagesInput struct {
	Tasks         []entities.SyncTask
	RegistryHost  string
	RegistryUser  string
	RegistryPass  string
	RegistryNS    string
	Force         bool
	DryRun        bool
	WorkerCount   int
	Logger        output.Logger
	StageCallback func(stage PullStage, polls int)

	TargetType        output.SyncTargetType
	TargetStrategy    output.SyncTargetStrategy
	TargetRegistry    string
	PrivateRegistries []struct {
		Name     string
		Host     string
		Username string
		Password string
	}
}

type SyncImagesUseCase interface {
	Execute(ctx context.Context, input SyncImagesInput) (synced []entities.SyncTask, needsSync []entities.SyncTask, err error)
}
