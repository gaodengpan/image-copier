package input

import "context"

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
