package gateways

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/infrastructure/encryption"
)

type DockerSyncStrategy struct {
	dockerClient   output.DockerClient
	registryClient output.RegistryClient
	fileSystem     output.FileSystem
}

func NewDockerSyncStrategy(
	dockerClient output.DockerClient,
	registryClient output.RegistryClient,
	fileSystem output.FileSystem,
) *DockerSyncStrategy {
	return &DockerSyncStrategy{
		dockerClient:   dockerClient,
		registryClient: registryClient,
		fileSystem:     fileSystem,
	}
}

// isAuthenticationError checks if the error indicates an authentication failure
func isAuthenticationError(err error) bool {
	errMsg := strings.ToLower(err.Error())
	authKeywords := []string{
		"access is denied",
		"unauthorized",
		"authentication required",
		"invalid credentials",
		"401",
		"403",
	}

	for _, keyword := range authKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}
	return false
}

// generateAuthErrorDiagnostics creates diagnostic information for authentication failures
func generateAuthErrorDiagnostics(registryHost, username string) string {
	var diagnostics []string

	diagnostics = append(diagnostics, fmt.Sprintf("registry: %s", registryHost))

	if username == "" {
		diagnostics = append(diagnostics, "username: (empty)")
	} else {
		// Only show first 2 characters of username for privacy
		if len(username) > 2 {
			diagnostics = append(diagnostics, fmt.Sprintf("username: %s***", username[:2]))
		} else {
			diagnostics = append(diagnostics, "username: ***")
		}
	}

	// Check IMAGE_COPIER_ENCRYPT_KEY status
	encryptKey := os.Getenv("IMAGE_COPIER_ENCRYPT_KEY")
	if encryptKey == "" {
		diagnostics = append(diagnostics, "IMAGE_COPIER_ENCRYPT_KEY: not set")
	} else {
		diagnostics = append(diagnostics, "IMAGE_COPIER_ENCRYPT_KEY: set")
	}

	return fmt.Sprintf("\nAuthentication diagnostic info: %s",
		strings.Join(diagnostics, ", "))
}

// checkPasswordFormat checks if the password looks like it might be corrupted
func checkPasswordFormat(password string) string {
	if password == "" {
		return "\nWarning: password is empty"
	}

	// Check if password starts with "encrypted:" but isn't valid
	if strings.HasPrefix(password, "encrypted:") {
		if !encryption.IsValidEncryptedFormat(password) {
			return "\nWarning: password has 'encrypted:' prefix but is not valid encrypted format. " +
				"Please re-encrypt your configuration or use plaintext password."
		}
	}

	return ""
}

func (s *DockerSyncStrategy) SyncFromRegistry(ctx context.Context, opts output.SyncTargetOptions) error {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})

	// For cross-VM scenarios (like lima), we still need to use a temp file
	// because io.Pipe only works within the same process
	tmpPath, err := s.fileSystem.CreateTempFile("image-copier-*.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := func() {
		_ = s.fileSystem.RemoveFile(tmpPath)
	}
	defer cleanup()

	if err := s.registryClient.SaveImageToFile(ctx, output.RegistrySaveOptions{
		ImageID:    sourceImageID,
		ImageTag:   opts.TargetImageTag,
		OutputPath: tmpPath,
		Username:   opts.SourceRegistryUsername,
		Password:   opts.SourceRegistryPassword,
	}); err != nil {
		baseErr := fmt.Errorf("failed to save image to file: %w", err)

		// Add diagnostics for authentication errors
		if isAuthenticationError(err) {
			diagMsg := generateAuthErrorDiagnostics(opts.SourceRegistryHost, opts.SourceRegistryUsername)
			passwordWarning := checkPasswordFormat(opts.SourceRegistryPassword)
			return fmt.Errorf("%w%s%s", baseErr, diagMsg, passwordWarning)
		}

		return baseErr
	}

	if err := s.dockerClient.LoadImage(ctx, tmpPath); err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}

	return nil
}

func (s *DockerSyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})
	return s.dockerClient.ImageExists(ctx, sourceImageID)
}

func (s *DockerSyncStrategy) Name() output.SyncTargetType {
	return output.SyncTargetDocker
}

// Distribute implements DistributionStrategy interface
func (s *DockerSyncStrategy) Distribute(ctx context.Context, opts output.DistributionOptions) error {
	syncOpts := opts.ToSyncTargetOptions()
	syncOpts.TargetImageTag = opts.SourceImageID
	return s.SyncFromRegistry(ctx, syncOpts)
}

// ExistsInDistributionTarget checks if the image exists in the distribution target
// This method implements DistributionStrategy interface
func (s *DockerSyncStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	syncOpts := opts.ToSyncTargetOptions()
	return s.ExistsInTarget(ctx, syncOpts)
}

// TargetType implements DistributionStrategy interface
func (s *DockerSyncStrategy) TargetType() value_objects.TargetType {
	return value_objects.TargetTypeDocker
}

var _ output.SyncTargetStrategy = (*DockerSyncStrategy)(nil)
var _ output.DistributionStrategy = (*DockerSyncStrategy)(nil)
