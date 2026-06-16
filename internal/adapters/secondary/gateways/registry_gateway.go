package gateways

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/shared/auth"
	sharederrors "github.com/gaodengpan/image-copier/internal/shared/errors"
	"github.com/gaodengpan/image-copier/internal/shared/sanitizer"
)

const (
	SkopeoCommand = "skopeo"
)

type SkopeoAdapter struct {
	commandRunner     func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator         *validators.ImageValidator
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

// errAuthFileCreation is a sentinel error used to distinguish auth file creation
// failures from callback execution failures in withAuthFile.
var errAuthFileCreation = errors.New("auth file creation failed")

// withAuthFile creates a temp auth file and calls fn with it, cleaning up afterward.
// Auth file creation errors are wrapped with errAuthFileCreation sentinel so callers
// can distinguish them from callback execution errors via errors.Is.
func (a *SkopeoAdapter) withAuthFile(imageID, username, password string, fn func(authFile string) error) error {
	registry := extractRegistry(imageID)
	authFile, err := auth.CreateAuthFile(registry, username, password)
	if err != nil {
		return fmt.Errorf("%w: %v", errAuthFileCreation, err)
	}
	defer os.Remove(authFile)
	return fn(authFile)
}

func (a *SkopeoAdapter) ImageExists(ctx context.Context, opts output.RegistryAuthOptions) (bool, error) {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return false, sharederrors.NewRegistryError("ImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return false, sharederrors.NewRegistryError("ImageExists", "invalid credentials", nil)
	}

	var result bool
	err := a.withAuthFile(opts.ImageID, opts.Username, opts.Password, func(authFile string) error {
		ctx, cancel := context.WithTimeout(ctx, a.imageCheckTimeout)
		defer cancel()

		cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "inspect", "--authfile", authFile, "docker://"+opts.ImageID)

		_, err := cmd.Output()
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				result = false
				return nil
			}
			return sharederrors.NewRegistryError("ImageExists", "failed to check image", err)
		}

		result = true
		return nil
	})
	if err != nil {
		if errors.Is(err, errAuthFileCreation) {
			return false, sharederrors.NewRegistryError("ImageExists", "failed to create auth file", err)
		}
		return false, err
	}

	return result, nil
}

func (a *SkopeoAdapter) SaveImageToFile(ctx context.Context, opts output.RegistrySaveOptions) error {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return sharederrors.NewRegistryError("SaveImageToFile", "invalid image name", nil)
	}
	if !a.validator.ValidateFilePath(opts.OutputPath) {
		return sharederrors.NewRegistryError("SaveImageToFile", "invalid file path", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return sharederrors.NewRegistryError("SaveImageToFile", "invalid credentials", nil)
	}

	err := a.withAuthFile(opts.ImageID, opts.Username, opts.Password, func(authFile string) error {
		cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "copy", "--authfile", authFile, "docker://"+opts.ImageID, "docker-archive:"+opts.OutputPath+":"+opts.ImageTag)

		output, err := cmd.CombinedOutput()
		if err != nil {
			safeOutput := sanitizer.SanitizeError(string(output), 500)
			return sharederrors.NewRegistryError("SaveImageToFile", fmt.Sprintf("failed to save image: %s", safeOutput), err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errAuthFileCreation) {
			return sharederrors.NewRegistryError("SaveImageToFile", "failed to create auth file", err)
		}
		return err
	}

	return nil
}

func (a *SkopeoAdapter) BuildDestImageID(opts output.BuildDestOptions) string {
	return value_objects.ParseImageID(opts.SourceID).BuildDestImageID(opts.RegistryHost, opts.RegistryNamespace)
}

// SaveImageToWriter saves a registry image to a writer using streaming
func (a *SkopeoAdapter) SaveImageToWriter(ctx context.Context, opts output.RegistrySaveOptions) error {
	if !a.validator.IsValidImageName(opts.ImageID) {
		return sharederrors.NewRegistryError("SaveImageToWriter", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(opts.Username, opts.Password) {
		return sharederrors.NewRegistryError("SaveImageToWriter", "invalid credentials", nil)
	}

	if opts.Writer == nil {
		return sharederrors.NewRegistryError("SaveImageToWriter", "writer is required", nil)
	}

	err := a.withAuthFile(opts.ImageID, opts.Username, opts.Password, func(authFile string) error {
		cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "copy", "--authfile", authFile, "docker://"+opts.ImageID, "docker-archive:/dev/stdout:"+opts.ImageTag)
		cmd.Stdout = opts.Writer
		cmd.Stderr = os.Stderr // Redirect stderr to show progress

		if err := cmd.Run(); err != nil {
			return sharederrors.NewRegistryError("SaveImageToWriter", fmt.Sprintf("failed to save image to writer: %v", err), err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errAuthFileCreation) {
			return sharederrors.NewRegistryError("SaveImageToWriter", "failed to create auth file", err)
		}
		return err
	}

	return nil
}

var _ output.RegistryClient = (*SkopeoAdapter)(nil)
