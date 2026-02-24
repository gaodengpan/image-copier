package gateways

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
)

const (
	SkopeoCommand = "skopeo"
)

type SkopeoAdapter struct {
	commandRunner  func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator      *validators.ImageValidator
	imageIDService *services.ImageIDService
}

func NewSkopeoAdapter() *SkopeoAdapter {
	return &SkopeoAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		validator:      validators.NewImageValidator(),
		imageIDService: services.NewImageIDService(),
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

func (a *SkopeoAdapter) SaveImageToFile(ctx context.Context, imageID, imageTag, outputPath, username, password string) error {
	if !a.validator.IsValidImageName(imageID) {
		return errors.NewRegistryError("SaveImageToFile", "invalid image name", nil)
	}
	if !a.validator.ValidateFilePath(outputPath) {
		return errors.NewRegistryError("SaveImageToFile", "invalid file path", nil)
	}

	if !a.validator.ValidateCredentials(username, password) {
		return errors.NewRegistryError("SaveImageToFile", "invalid credentials", nil)
	}

	creds := fmt.Sprintf("%s:%s", username, password)
	cmd := a.commandRunner(ctx, SkopeoCommand, "copy", "--src-creds="+creds, "docker://"+imageID, "docker-archive:"+outputPath+":"+imageTag)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewRegistryError("SaveImageToFile", fmt.Sprintf("failed to save image: %s", string(output)), err)
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

func (a *SkopeoAdapter) BuildDestImageID(sourceID, registryHost, registryNamespace string) string {
	return a.imageIDService.BuildDestImageID(sourceID, registryHost, registryNamespace)
}

var _ output.RegistryClient = (*SkopeoAdapter)(nil)
