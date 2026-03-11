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
	"github.com/gaodengpan/image-copier/internal/domain/validators"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
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

// dockerAuthConfig represents Docker config.json format for authentication
type dockerAuthConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Auth string `json:"auth"`
}

// createAuthFileForRegistry creates a temporary authentication file for a specific registry
func createAuthFileForRegistry(username, password string) (string, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	config := dockerAuthConfig{
		Auths: map[string]dockerAuthEntry{
			"": {Auth: auth},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}

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

func (s *RegistrySyncStrategy) SyncFromRegistry(ctx context.Context, opts output.SyncTargetOptions) error {
	sourceImageID := s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:          opts.SourceImageID,
		RegistryHost:      opts.SourceRegistryHost,
		RegistryNamespace: opts.SourceRegistryNS,
	})

	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID)

	ctx, cancel := context.WithTimeout(ctx, s.syncOperationTimeout)
	defer cancel()

	// Create temp auth files for source and destination
	srcAuthFile, err := createAuthFileForRegistry(opts.SourceRegistryUsername, opts.SourceRegistryPassword)
	if err != nil {
		return errors.NewRegistryError("SyncFromRegistry", "failed to create source auth file", err)
	}
	defer os.Remove(srcAuthFile)

	destAuthFile, err := createAuthFileForRegistry(opts.TargetRegistryUsername, opts.TargetRegistryPassword)
	if err != nil {
		return errors.NewRegistryError("SyncFromRegistry", "failed to create destination auth file", err)
	}
	defer os.Remove(destAuthFile)

	// Use --src-authfile and --dest-authfile to pass credentials securely
	cmd := s.commandRunner(ctx, SkopeoCommand,
		"copy",
		"--src-authfile", srcAuthFile,
		"--dest-authfile", destAuthFile,
		"docker://"+sourceImageID,
		"docker://"+targetImageID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		safeOutput := sanitizer.SanitizeError(string(output), 500)
		return errors.NewRegistryError("SyncFromRegistry",
			fmt.Sprintf("failed to sync image from %s to %s: %s", sourceImageID, targetImageID, safeOutput), err)
	}

	return nil
}

func (s *RegistrySyncStrategy) ExistsInTarget(ctx context.Context, opts output.SyncTargetOptions) (bool, error) {
	targetImageID := s.buildTargetImageID(opts.TargetRegistryHost, opts.SourceImageID)
	return s.registryClient.CheckImageExists(ctx, output.RegistryAuthOptions{
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
	syncOpts := output.SyncTargetOptions{
		SourceImageID:          opts.SourceImageID,
		SourceRegistryHost:     opts.SourceRegistryHost,
		SourceRegistryNS:       opts.SourceRegistryNS,
		SourceRegistryUsername: opts.SourceRegistryUser,
		SourceRegistryPassword: opts.SourceRegistryPass,
		TargetRegistryHost:     opts.TargetRegistryHost,
		TargetRegistryUsername: opts.TargetRegistryUser,
		TargetRegistryPassword: opts.TargetRegistryPass,
	}
	return s.SyncFromRegistry(ctx, syncOpts)
}

// ExistsInDistributionTarget checks if the image exists in the distribution target
func (s *RegistrySyncStrategy) ExistsInDistributionTarget(ctx context.Context, opts output.DistributionOptions) (bool, error) {
	syncOpts := output.SyncTargetOptions{
		SourceImageID:          opts.SourceImageID,
		TargetRegistryHost:     opts.TargetRegistryHost,
		TargetRegistryUsername: opts.TargetRegistryUser,
		TargetRegistryPassword: opts.TargetRegistryPass,
	}
	return s.ExistsInTarget(ctx, syncOpts)
}

// TargetType implements DistributionStrategy interface
func (s *RegistrySyncStrategy) TargetType() value_objects.TargetType {
	return value_objects.TargetTypeRegistry
}

func (s *RegistrySyncStrategy) buildTargetImageID(registryHost, sourceImageID string) string {
	return s.registryClient.BuildDestImageID(output.BuildDestOptions{
		SourceID:     sourceImageID,
		RegistryHost: registryHost,
	})
}

var _ output.SyncTargetStrategy = (*RegistrySyncStrategy)(nil)
var _ output.DistributionStrategy = (*RegistrySyncStrategy)(nil)
