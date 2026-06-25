package gateways

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
)

const (
	DockerCommand     = "docker"
	DockerImageFormat = "{{.Repository}}:{{.Tag}}"
	ListImagesTimeout = 15 * time.Second
	CheckLocalTimeout = 10 * time.Second
	// EnvDockerCommand allows users to override the docker command
	// Example: IMAGE_COPIER_DOCKER_CMD="limactl shell k3s-master -- sudo nerdctl --address /run/k3s/containerd/containerd.sock -n k8s.io"
	EnvDockerCommand = "IMAGE_COPIER_DOCKER_CMD"
)

type ExecDockerAdapter struct {
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator     *validators.ImageValidator
}

// NewExecDockerAdapter creates a new Docker adapter
// It respects IMAGE_COPIER_DOCKER_CMD environment variable for custom docker command
func NewExecDockerAdapter() *ExecDockerAdapter {
	// Check for custom docker command from environment variable
	customCmd := os.Getenv(EnvDockerCommand)

	return &ExecDockerAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name == DockerCommand && customCmd != "" {
				// Parse the custom command (e.g., "limactl shell k3s-master -- sudo nerdctl ...")
				// Split the command while respecting quoted strings
				parts := parseCommand(customCmd)
				if len(parts) > 0 {
					fullArgs := append(parts[1:], args...)
					return exec.CommandContext(ctx, parts[0], fullArgs...)
				}
			}
			return exec.CommandContext(ctx, name, args...)
		},
		validator: validators.NewImageValidator(),
	}
}

// parseCommand splits a command string into parts, respecting quoted strings
func parseCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
		case !inQuote && r == ' ':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
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
	// Read the tar file and pass it to docker load via stdin
	// This works around filesystem isolation issues (e.g., lima VM)
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return errors.NewDockerError("LoadImage", fmt.Sprintf("failed to open tar file %s: %v", tarPath, err), err)
	}
	defer func() {
		_ = tarFile.Close()
	}()

	return a.LoadImageFromReader(ctx, tarFile)
}

// LoadImageFromReader loads a Docker image from a reader using streaming
func (a *ExecDockerAdapter) LoadImageFromReader(ctx context.Context, reader io.Reader) error {
	cmd := a.commandRunner(ctx, DockerCommand, "load")
	cmd.Stdin = reader
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewDockerError("LoadImageFromReader", fmt.Sprintf("failed to load image from stdin: %s", string(output)), err)
	}

	return nil
}

var _ output.DockerClient = (*ExecDockerAdapter)(nil)
