package use_cases

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/application/ports"
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

type SyncTask struct {
	Source string
	Arch   string
	Os     string
}

func (t SyncTask) DisplayName() string {
	return t.Source + " (" + t.Os + "/" + t.Arch + ")"
}

type SyncImagesInput struct {
	Tasks         []SyncTask
	RegistryHost  string
	RegistryUser  string
	RegistryPass  string
	RegistryNS    string
	Force         bool
	DryRun        bool
	WorkerCount   int
	Logger        ports.Logger
	StageCallback func(stage PullStage, polls int)
}

type SyncImagesUseCase interface {
	Execute(ctx context.Context, input SyncImagesInput) (synced []SyncTask, needsSync []SyncTask, err error)
}
