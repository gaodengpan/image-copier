package gateways

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/services"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
	"github.com/gaodengpan/image-copier/internal/shared/sanitizer"
)

const (
	SkopeoCommand = "skopeo"
)

type SkopeoAdapter struct {
	commandRunner      func(ctx context.Context, name string, args ...string) *exec.Cmd
	validator          *validators.ImageValidator
	imageIDService     *services.ImageIDService
	imageCheckTimeout  time.Duration
}

func NewSkopeoAdapter() *SkopeoAdapter {
	return &SkopeoAdapter{
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		validator:          validators.NewImageValidator(),
		imageIDService:     services.NewImageIDService(),
		imageCheckTimeout:  30 * time.Second,
	}
}

// WithImageCheckTimeout sets the image check timeout
func (a *SkopeoAdapter) WithImageCheckTimeout(timeout time.Duration) *SkopeoAdapter {
	a.imageCheckTimeout = timeout
	return a
}

// dockerConfig represents Docker config.json format for authentication
type dockerConfig struct {
	Auths map[string]dockerAuth `json:"auths"`
}

type dockerAuth struct {
	Auth string `json:"auth"`
}

// createAuthFile creates a temporary authentication file for skopeo
// and returns the file path. Caller is responsible for deleting the file.
func createAuthFile(username, password string) (string, error) {
	// Create auth entry (base64 encoded username:password)
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	config := dockerConfig{
		Auths: map[string]dockerAuth{
			"": {Auth: auth}, // Empty key works as default for all registries
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "skopeo-auth-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp auth file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write auth config: %w", err)
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
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

func (a *SkopeoAdapter) ImageExists(ctx context.Context, imageID, username, password string) (bool, error) {
	if !a.validator.IsValidImageName(imageID) {
		return false, errors.NewRegistryError("ImageExists", "invalid image name", nil)
	}

	if !a.validator.ValidateCredentials(username, password) {
		return false, errors.NewRegistryError("ImageExists", "invalid credentials", nil)
	}

	// Create temp auth file
	authFile, err := createAuthFile(username, password)
	if err != nil {
		return false, errors.NewRegistryError("ImageExists", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	ctx, cancel := context.WithTimeout(ctx, a.imageCheckTimeout)
	defer cancel()

	// Use --authfile to pass credentials securely (not visible in process list)
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "inspect", "--authfile", authFile, "docker://"+imageID)

	_, err = cmd.Output()
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

	// Create temp auth file
	authFile, err := createAuthFile(username, password)
	if err != nil {
		return errors.NewRegistryError("SaveImageToFile", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	// Use --authfile to pass credentials securely
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "copy", "--authfile", authFile, "docker://"+imageID, "docker-archive:"+outputPath+":"+imageTag)

	output, err := cmd.CombinedOutput()
	if err != nil {
		safeOutput := sanitizer.SanitizeError(string(output), 500)
		return errors.NewRegistryError("SaveImageToFile", fmt.Sprintf("failed to save image: %s", safeOutput), err)
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

	// Create temp auth file
	authFile, err := createAuthFile(username, password)
	if err != nil {
		return false, errors.NewRegistryError("CheckImageExists", "failed to create auth file", err)
	}
	defer os.Remove(authFile)

	ctx, cancel := context.WithTimeout(ctx, a.imageCheckTimeout)
	defer cancel()

	// Use --authfile to pass credentials securely
	cmd := a.buildSkopeoCmdWithAuth(ctx, authFile, "inspect", "--authfile", authFile, "docker://"+imageID)

	_, err = cmd.Output()
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