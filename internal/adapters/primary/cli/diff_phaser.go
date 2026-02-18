package cli

import (
	"context"
	"sync"

	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
	"github.com/sirupsen/logrus"
)

type DiffResult struct {
	Task         syncTask
	RemoteExists bool
	LocalExists  bool
	RemoteError  error
	LocalError   error
}

type DiffPhaser struct {
	logger         *logrus.Logger
	dockerClient   ports.DockerClient
	registryClient ports.RegistryClient
	cfg            *config.Config
}

func NewDiffPhaser(logger *logrus.Logger, dockerClient ports.DockerClient, registryClient ports.RegistryClient, cfg *config.Config) *DiffPhaser {
	return &DiffPhaser{
		logger:         logger,
		dockerClient:   dockerClient,
		registryClient: registryClient,
		cfg:            cfg,
	}
}

func (p *DiffPhaser) Execute(ctx context.Context, tasks []syncTask, workerCount int) []DiffResult {
	results := make([]DiffResult, len(tasks))

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, task syncTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			sourceID := task.Source
			destID := p.registryClient.BuildDestImageID(sourceID, p.cfg.Registry.Host, p.cfg.Registry.Namespace)
			remoteExists, remoteErr := p.registryClient.CheckImageExists(ctx, destID, p.cfg.Registry.Username, p.cfg.Registry.Password)
			localExists, localErr := p.dockerClient.ImageExists(ctx, task.Source)

			if remoteErr != nil {
				p.logger.Warnf("Failed to check remote image %s: %v", destID, remoteErr)
			}
			if localErr != nil {
				p.logger.Warnf("Failed to check local image %s: %v", task.Source, localErr)
			}

			results[idx] = DiffResult{
				Task:         task,
				RemoteExists: remoteExists,
				LocalExists:  localExists,
				RemoteError:  remoteErr,
				LocalError:   localErr,
			}
		}(i, t)
	}

	wg.Wait()
	return results
}

func (p *DiffPhaser) PartitionResults(results []DiffResult, force bool) (synced, needsSync []syncTask) {
	for _, r := range results {
		if r.LocalExists && !force {
			synced = append(synced, r.Task)
		} else {
			needsSync = append(needsSync, r.Task)
		}
	}
	return synced, needsSync
}
