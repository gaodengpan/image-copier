package gateways

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/auth"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
	"github.com/gaodengpan/image-copier/internal/shared/sanitizer"
)

const (
	SkopeoCommand = "skopeo"
)

type SkopeoAdapter struct {
	commandRunner     func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator         *validators.ImageValidator
	imageIDService    *services.ImageIDService
	imageCheckTimeout time.Duration
}

// extractRegistry extracts the registry host from an image ID
func extractRegistry(imageID string) string {
	parts := strings.SplitN(imageID, "/", 2)
	if len(parts) == 2 {
		first := parts[0]
		// Check if first part looks like a registry (contains . or : or is localhost)
		if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
			return first
		}
	}
	// Default registry for Docker Hub images
	return "docker.io"
}

func NewSkopeoAdapter() *SkopeoAdapter {
	return &SkopeoAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		validator:         validators.NewImageValidator(),
		imageIDService:    services.NewImageIDService(),
		imageCheckTimeout: 30 * time.Second,
	}
}

// WithImageCheckTimeout sets the image check timeout
func (a *SkopeoAdapter) WithImageCheckTimeout(timeout time.Duration) *SkopeoAdapter {
	a.imageCheckTimeout = timeout
	return a
}

// buildSkopeoCmdWithAuth creates a skopeo command using auth file instead of command line credentials
func (a *SkopeoAdapter) buildSkopeoCmdWithAuth(ctx context.Context, authFile string, args ...string) *exec.Cmd {
	cmd := a.commandRunner(ctx, SkopeoCommand, args...)

	// Pass auth file via environment variable
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("REGISTRY_AUTH_FILE=%s", authFile),
	)

	return cmd
}

func (a *SkopeoAdapter) ImageExists(ctx context.Context, opts output.RegistryAuthOptions) (bool, error) {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return false, errors.NewRegistryError("ImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return false, errors.NewRegistryError("ImageExists", "invalid credentials", nil)
	}

	registry := extractRegistry(opts.ImageID)

	// Create temp auth file
	authFile, err := auth.CreateAuthFile(registry, opts.Username, opts.Password)
	if err != nil {
		return false, errors.NewRegistryError("ImageExists", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	ctx, cancel := context.WithTimeout(ctx, a.imageCheckTimeout)
	defer cancel()

	// Use --authfile to pass credentials securely (not visible in process list)
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "inspect", "--authfile", authFile, "docker://"+opts.ImageID)

	_, err = cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.NewRegistryError("ImageExists", "failed to check image", err)
	}

	return true, nil
}

func (a *SkopeoAdapter) SaveImageToFile(ctx context.Context, opts output.RegistrySaveOptions) error {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return errors.NewRegistryError("SaveImageToFile", "invalid image name", nil)
	}
	if !a.validator.ValidateFilePath(opts.OutputPath) {
		return errors.NewRegistryError("SaveImageToFile", "invalid file path", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return errors.NewRegistryError("SaveImageToFile", "invalid credentials", nil)
	}

	registry := extractRegistry(opts.ImageID)

	// Create temp auth file
	authFile, err := auth.CreateAuthFile(registry, opts.Username, opts.Password)
	if err != nil {
		return errors.NewRegistryError("SaveImageToFile", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	// Use --authfile to pass credentials securely
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "copy", "--authfile", authFile, "docker://"+opts.ImageID, "docker-archive:"+opts.OutputPath+":"+opts.ImageTag)

	output, err := cmd.CombinedOutput()
	if err != nil {
		safeOutput := sanitizer.SanitizeError(string(output), 500)
		return errors.NewRegistryError("SaveImageToFile", fmt.Sprintf("failed to save image: %s", safeOutput), err)
	}

	return nil
}

func (a *SkopeoAdapter) CheckImageExists(ctx context.Context, opts output.RegistryAuthOptions) (bool, error) {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return false, errors.NewRegistryError("CheckImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return false, errors.NewRegistryError("CheckImageExists", "invalid credentials", nil)
	}

	registry := extractRegistry(opts.ImageID)

	// Create temp auth file
	authFile, err := auth.CreateAuthFile(registry, opts.Username, opts.Password)
	if err != nil {
		return false, errors.NewRegistryError("CheckImageExists", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	ctx, cancel := context.WithTimeout(ctx, a.imageCheckTimeout)
	defer cancel()

	// Use --authfile to pass credentials securely
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "inspect", "--authfile", authFile, "docker://"+opts.ImageID)

	_, err = cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.NewRegistryError("CheckImageExists", "failed to check image", err)
	}

	return true, nil
}

func (a *SkopeoAdapter) BuildDestImageID(opts output.BuildDestOptions) string {
	return a.imageIDService.BuildDestImageID(opts.SourceID, opts.RegistryHost, opts.RegistryNamespace)
}

// SaveImageToWriter saves a registry image to a writer using streaming
func (a *SkopeoAdapter) SaveImageToWriter(ctx context.Context, opts output.RegistrySaveOptions) error {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return errors.NewRegistryError("SaveImageToWriter", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return errors.NewRegistryError("SaveImageToWriter", "invalid credentials", nil)
	}

	if opts.Writer == nil {
		return errors.NewRegistryError("SaveImageToWriter", "writer is required", nil)
	}

	registry := extractRegistry(opts.ImageID)

	// Create temp auth file
	authFile, err := auth.CreateAuthFile(registry, opts.Username, opts.Password)
	if err != nil {
		return errors.NewRegistryError("SaveImageToWriter", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	// Use skopeo copy to stdout
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "copy", "--authfile", authFile, "docker://"+opts.ImageID, "docker-archive:/dev/stdout:"+opts.ImageTag)
	cmd.Stdout = opts.Writer
	cmd.Stderr = os.Stderr // Redirect stderr to show progress

	if err := cmd.Run(); err != nil {
		return errors.NewRegistryError("SaveImageToWriter", fmt.Sprintf("failed to save image to writer: %v", err), err)
	}

	return nil
}

var _ output.RegistryClient = (*SkopeoAdapter)(nil)
