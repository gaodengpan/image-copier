package registry

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/gaodengpan/image-copier/internal/adapters/errors"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/ports"
)

const (
	SkopeoCommand     = "skopeo"
	SkopeoCopyTimeout = 120 * time.Second
)

type SkopeoAdapter struct {
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator     *validators.ImageValidator
}

func NewSkopeoAdapter() *SkopeoAdapter {
	return &SkopeoAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		validator: validators.NewImageValidator(),
	}
}

func (a *SkopeoAdapter) ImageExists(ctx context.Context, imageID, username, password string) (bool, error) {
	if !a.validator.IsValidImageName(imageID) {
		return false, errors.NewRegistryError("ImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(username, password) {
		return false, errors.NewRegistryError("ImageExists", "invalid credentials", nil)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	creds := fmt.Sprintf("%s:%s", username, password)
	cmd := a.commandRunner(ctx, SkopeoCommand, "inspect", "--creds="+creds, "docker://"+imageID)

	_, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.NewRegistryError("ImageExists", "failed to check image", err)
	}

	return true, nil
}

func (a *SkopeoAdapter) CopyImage(ctx context.Context, source, dest, username, password string) error {
	if !a.validator.IsValidImageName(source) || !a.validator.IsValidImageName(dest) {
		return errors.NewRegistryError("CopyImage", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(username, password) {
		return errors.NewRegistryError("CopyImage", "invalid credentials", nil)
	}

	ctx, cancel := context.WithTimeout(ctx, SkopeoCopyTimeout)
	defer cancel()

	creds := fmt.Sprintf("%s:%s", username, password)
	cmd := a.commandRunner(ctx, SkopeoCommand, "copy", "--src-creds="+creds, "docker://"+source, "docker-archive:"+dest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewRegistryError("CopyImage", fmt.Sprintf("failed to copy image: %s", string(output)), err)
	}

	return nil
}

func (a *SkopeoAdapter) CheckImageExists(ctx context.Context, imageID, username, password string) (bool, error) {
	if !a.validator.IsValidImageName(imageID) {
		return false, errors.NewRegistryError("CheckImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(username, password) {
		return false, errors.NewRegistryError("CheckImageExists", "invalid credentials", nil)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	creds := fmt.Sprintf("%s:%s", username, password)
	cmd := a.commandRunner(ctx, SkopeoCommand, "inspect", "--creds="+creds, "docker://"+imageID)

	_, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.NewRegistryError("CheckImageExists", "failed to check image", err)
	}

	return true, nil
}

var _ ports.RegistryClient = (*SkopeoAdapter)(nil)
