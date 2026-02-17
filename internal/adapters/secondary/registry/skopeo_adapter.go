package registry

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
)

const (
	SkopeoCommand = "skopeo"
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
	var tag, digest, imageName string

	digestIndex := strings.LastIndex(sourceID, "@")
	if digestIndex != -1 {
		digest = sourceID[digestIndex:]
		imageName = sourceID[:digestIndex]
	} else {
		imageName = sourceID
	}

	if digestIndex == -1 {
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:]
			imageName = imageName[:tagIndex]
		}
	} else {
		tagIndex := strings.LastIndex(imageName, ":")
		if tagIndex != -1 {
			tag = imageName[tagIndex:]
			imageName = imageName[:tagIndex]
		}
	}

	if registryHost == "" {
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ":", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		const maxLen = 50
		maxBaseLen := maxLen
		if tag != "" {
			maxBaseLen -= len(tag)
		}
		if digest != "" {
			maxBaseLen -= len(digest)
		}
		if maxBaseLen < 0 {
			maxBaseLen = 0
		}
		if len(normalized) > maxBaseLen {
			normalized = normalized[:maxBaseLen]
		}

		normalized = normalized + tag + digest
		return normalized
	}

	// When registryHost is not empty but registryNamespace is empty
	if registryNamespace == "" {
		normalized := strings.ReplaceAll(imageName, "/", "_")
		normalized = strings.ReplaceAll(normalized, ".", "_")
		normalized = strings.ReplaceAll(normalized, "-", "_")

		const maxLen = 50
		maxBaseLen := maxLen
		if tag != "" {
			maxBaseLen -= len(tag)
		}
		if digest != "" {
			maxBaseLen -= len(digest)
		}
		if maxBaseLen < 0 {
			maxBaseLen = 0
		}
		if len(normalized) > maxBaseLen {
			normalized = normalized[:maxBaseLen]
		}

		normalized = strings.TrimRight(normalized, "_")
		normalized = normalized + tag + digest
		return fmt.Sprintf("%s/%s", registryHost, normalized)
	}

	// When both registryHost and registryNamespace are not empty
	normalized := strings.ReplaceAll(imageName, "/", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	const maxLen = 50
	maxBaseLen := maxLen
	if tag != "" {
		maxBaseLen -= len(tag)
	}
	if digest != "" {
		maxBaseLen -= len(digest)
	}
	if maxBaseLen < 0 {
		maxBaseLen = 0
	}
	if len(normalized) > maxBaseLen {
		normalized = normalized[:maxBaseLen]
	}

	normalized = strings.TrimRight(normalized, "_")
	normalized = normalized + tag + digest
	return fmt.Sprintf("%s/%s/%s", registryHost, registryNamespace, normalized)
}

var _ ports.RegistryClient = (*SkopeoAdapter)(nil)
