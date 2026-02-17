package use_cases

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

type CheckImageInput struct {
	ImageID      string
	RegistryHost string
	RegistryUser string
	RegistryPass string
	RegistryNS   string
}

type CheckImageOutput struct {
	LocalExists  bool
	RemoteExists bool
	Error        error
}

type CheckImageUseCase interface {
	ExecuteLocal(ctx context.Context, input CheckImageInput) (bool, error)
	ExecuteRemote(ctx context.Context, input CheckImageInput) (bool, error)
}

type SyncTask struct {
	Source string
	Arch   string
	Os     string
}

func (t SyncTask) DisplayName() string {
	return t.Source + " (" + t.Os + "/" + t.Arch + ")"
}

type PullImagesInput struct {
	Images       []string
	RegistryHost string
	RegistryUser string
	RegistryPass string
	RegistryNS   string
	RegistryArch string
	RegistryOs   string
	Force        bool
	DryRun       bool
	WorkerCount  int
	Logger       interface {
		Infof(format string, args ...interface{})
		Debugf(format string, args ...interface{})
		Errorf(format string, args ...interface{})
		Info(args ...interface{})
		Warn(args ...interface{})
	}
	StageCallback func(stage PullStage, polls int)
}

type PullImagesUseCase interface {
	Execute(ctx context.Context, input PullImagesInput) (failedCount int, err error)
}

type SyncImagesInput struct {
	Tasks        []SyncTask
	RegistryHost string
	RegistryUser string
	RegistryPass string
	RegistryNS   string
	Force        bool
	DryRun       bool
	WorkerCount  int
	Logger       interface {
		Infof(format string, args ...interface{})
		Debugf(format string, args ...interface{})
		Errorf(format string, args ...interface{})
		Info(args ...interface{})
		Warn(args ...interface{})
	}
	StageCallback func(stage PullStage, polls int)
}

type SyncImagesUseCase interface {
	Execute(ctx context.Context, input SyncImagesInput) (synced []SyncTask, needsSync []SyncTask, err error)
}
