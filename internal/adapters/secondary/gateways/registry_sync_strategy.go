package gateways

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gaodengpan/image-copier/internal/domain/ports/output"
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
	"github.com/gaodengpan/image-copier/internal/shared/auth"
	"github.com/gaodengpan/image-copier/internal/shared/errors"
	"github.com/gaodengpan/image-copier/internal/shared/sanitizer"
)

type RegistrySyncStrategy struct {
	registryClient       output.RegistryClient
	validator            *validators.ImageValidator
	commandRunner        func(ctx context.Context, name string, args ...string) *exec.Cmd
	syncOperationTimeout time.Duration
}

func NewRegistrySyncStrategy(registryClient output.RegistryClient) *RegistrySyncStrategy {
	return &RegistrySyncStrategy{
		registryClient: registryClient,
		validator:      validators.NewImageValidator(),
		commandRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
		syncOperationTimeout: 10 * time.Minute,
	}
}

// WithSyncOperationTimeout sets the sync operation timeout
func (s *RegistrySyncStrategy) WithSyncOperationTimeout(timeout time.Duration) *RegistrySyncStrategy {
	s.syncOperationTimeout = timeout
	return s
}

func (s *RegistrySyncStrategy) SyncFromRegistry(ctx context.Context, opts output.SyncTargetOptions) error {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})

	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID, opts.TargetMirrorMode)

	ctx, cancel := context.WithTimeout(ctx, s.syncOperationTimeout)
	defer cancel()

	// Create temp auth files for source and destination
	// Use the actual registry host as the key for proper credential matching
	srcAuthFile, err := auth.CreateAuthFile(opts.SourceRegistryHost, opts.SourceRegistryUsername, opts.SourceRegistryPassword)
	if err != nil {
		return errors.NewRegistryError("SyncFromRegistry", "failed to create source auth file", err)
	}
	defer func() { _ = os.Remove(srcAuthFile) }()

	destAuthFile, err := auth.CreateAuthFile(opts.TargetRegistryHost, opts.TargetRegistryUsername, opts.TargetRegistryPassword)
	if err != nil {
		return errors.NewRegistryError("SyncFromRegistry", "failed to create destination auth file", err)
	}
	defer func() { _ = os.Remove(destAuthFile) }()

	// Use --src-authfile and --dest-authfile to pass credentials securely
	args := []string{
		"copy",
		"--src-authfile", srcAuthFile,
		"--dest-authfile", destAuthFile,
	}

	// Add TLS verification flags for insecure registries
	if opts.SourceInsecure {
		args = append(args, "--src-tls-verify=false")
	}
	if opts.TargetInsecure {
		args = append(args, "--dest-tls-verify=false")
	}

	args = append(args, "docker://"+sourceImageID, "docker://"+targetImageID)

	cmd := s.commandRunner(ctx, SkopeoCommand, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		safeOutput := sanitizer.SanitizeError(string(output), 500)
		baseErr := errors.NewRegistryError("SyncFromRegistry",
			fmt.Sprintf("failed to sync image from %s to %s: %s", sourceImageID, targetImageID, safeOutput), err)

		// Add diagnostics for authentication errors
		if isAuthenticationError(err) || strings.Contains(safeOutput, "access is denied") || strings.Contains(safeOutput, "unauthorized") {
			diagMsg := generateAuthErrorDiagnostics(opts.SourceRegistryHost, opts.SourceRegistryUsername)
			passwordWarning := checkPasswordFormat(opts.SourceRegistryPassword)
			return fmt.Errorf("%w%s%s", baseErr, diagMsg, passwordWarning)
		}

		return baseErr
	}

	return nil
}

func (s *RegistrySyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID, opts.TargetMirrorMode)
	return s.registryClient.ImageExists(ctx, output.RegistryAuthOptions{
		ImageID:  targetImageID,
		Username: opts.TargetRegistryUsername,
		Password: opts.TargetRegistryPassword,
	})
}

func (s *RegistrySyncStrategy) Name() output.SyncTargetType {
	return output.SyncTargetRegistry
}

// Distribute implements DistributionStrategy interface
func (s *RegistrySyncStrategy) Distribute(ctx context.Context, opts output.DistributionOptions) error {
	return s.SyncFromRegistry(ctx, opts.ToSyncTargetOptions())
}

// ExistsInDistributionTarget checks if the image exists in the distribution target
func (s *RegistrySyncStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	syncOpts := opts.ToSyncTargetOptions()
	return s.ExistsInTarget(ctx, syncOpts)
}

// TargetType implements DistributionStrategy interface
func (s *RegistrySyncStrategy) TargetType() value_objects.TargetType {
	return value_objects.TargetTypeRegistry
}

func (s *RegistrySyncStrategy) buildTargetImageID(registryHost, sourceImageID string, mirrorMode bool) string {
	// For mirror mode, keep original image name format
	if mirrorMode {
		// Remove docker.io/library/ prefix for Docker Hub images
		// e.g., docker.io/library/nginx:alpine -> nginx:alpine
		imageName := sourceImageID
		if strings.HasPrefix(imageName, "docker.io/library/") {
			imageName = strings.TrimPrefix(imageName, "docker.io/library/")
		} else if strings.HasPrefix(imageName, "docker.io/") {
			imageName = strings.TrimPrefix(imageName, "docker.io/")
		}
		return registryHost + "/" + imageName
	}
	// For non-mirror mode, use the normalized format
	return s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:     sourceImageID,
		RegistryHost: registryHost,
	})
}

var _ output.SyncTargetStrategy = (*RegistrySyncStrategy)(nil)
var _ output.DistributionStrategy = (*RegistrySyncStrategy)(nil)
