package gateways

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
)

const (
	DockerCommand     = "docker"
	DockerImageFormat = "{{.Repository}}:{{.Tag}}"
	ListImagesTimeout = 15 * time.Second
	CheckLocalTimeout = 10 * time.Second
)

type ExecDockerAdapter struct {
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator     *validators.ImageValidator
}

func NewExecDockerAdapter() *ExecDockerAdapter {
	return &ExecDockerAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		validator: validators.NewImageValidator(),
	}
}

func (a *ExecDockerAdapter) ImageExists(ctx context.Context, imageID string) (bool, error) {
	if !a.validator.IsValidImageName(imageID) {
		return false, errors.NewDockerError("ImageExists", "invalid image name", nil)
	}

	ctx, cancel := context.WithTimeout(ctx, CheckLocalTimeout)
	defer cancel()

	cmd := a.commandRunner(ctx, DockerCommand, "image", "inspect", imageID)
	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}

	return false, errors.NewDockerError("ImageExists", "failed to check image", err)
}

func (a *ExecDockerAdapter) ListImages(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, ListImagesTimeout)
	defer cancel()

	cmd := a.commandRunner(ctx, DockerCommand, "image", "ls", "--format", DockerImageFormat)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.NewDockerError("ListImages", "failed to list images", err)
	}

	lines := strings.Split(string(output), "\n")
	var images []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && a.validator.ValidateImageNameInput(line) {
			images = append(images, line)
		}
	}

	return images, nil
}

func (a *ExecDockerAdapter) LoadImage(ctx context.Context, tarPath string) error {
	cmd := a.commandRunner(ctx, DockerCommand, "load", "-i", tarPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewDockerError("LoadImage", fmt.Sprintf("failed to load image from %s: %s", tarPath, string(output)), err)
	}

	return nil
}

var _ ports.DockerClient = (*ExecDockerAdapter)(nil)
