package system

import (
	"context"

	"github.com/gaodengpan/image-copier/internal/domain/ports"
	"github.com/gaodengpan/image-copier/internal/utils"
)

type SystemAdapter struct{}

func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

func (a *SystemAdapter) CommandExists(ctx context.Context, cmd string) (bool, error) {
	return utils.CheckCommandExists(cmd)
}

func (a *SystemAdapter) DockerRunning(ctx context.Context) (bool, error) {
	return utils.CheckDockerService()
}

var _ ports.SystemClient = (*SystemAdapter)(nil)
