package system

import (
	"context"
	"os/exec"

	"github.com/gaodengpan/image-copier/internal/application/ports"
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

type CommandRunnerAdapter struct{}

func NewCommandRunnerAdapter() *CommandRunnerAdapter {
	return &CommandRunnerAdapter{}
}

func (a *CommandRunnerAdapter) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

func (a *CommandRunnerAdapter) RunWithOutput(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), "", err
}

var _ ports.CommandRunner = (*CommandRunnerAdapter)(nil)
