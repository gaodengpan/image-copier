package input

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/domain/entities"
)

type SyncImagesInput struct {
	Tasks         []entities.SyncTask
	RegistryHost  string
	RegistryUser  string
	RegistryPass  string
	RegistryNS    string
	Force         bool
	DryRun        bool
	WorkerCount   int
	Logger        interface{}
	StageCallback func(stage int, polls int)
}

type SyncImagesUseCase interface {
	Execute(ctx context.Context, input SyncImagesInput) (synced []entities.SyncTask, needsSync []entities.SyncTask, err error)
	Diff(ctx context.Context, tasks []entities.SyncTask, workerCount int, force bool) (synced []entities.SyncTask, needsSync []entities.SyncTask, err error)
	Sync(ctx context.Context, tasks []entities.SyncTask, workerCount int) error
}
