package output

import (
	"context"
	"fmt"
	"io"
)

// RegistryAuthOptions contains authentication options for registry operations
type RegistryAuthOptions struct {
	ImageID  string
	Username string
	Password string
}

// String returns a safe string representation that masks the password
func (o RegistryAuthOptions) String() string {
	return fmt.Sprintf("ImageID:%s, Username:%s, Password:***", o.ImageID, o.Username)
}

// RegistrySaveOptions contains options for saving registry images
type RegistrySaveOptions struct {
	ImageID    string
	ImageTag   string
	Username   string
	Password   string
	OutputPath string    // for SaveImageToFile
	Writer     io.Writer // for SaveImageToWriter
}

// String returns a safe string representation that masks the password
func (o RegistrySaveOptions) String() string {
	return fmt.Sprintf("ImageID:%s, ImageTag:%s, Username:%s, Password:***, OutputPath:%s",
		o.ImageID, o.ImageTag, o.Username, o.OutputPath)
}

// BuildDestOptions contains options for building destination image ID
type BuildDestOptions struct {
	SourceID          string
	RegistryHost      string
	RegistryNamespace string
}

// String returns a string representation of the options
func (o BuildDestOptions) String() string {
	return fmt.Sprintf("SourceID:%s, RegistryHost:%s, RegistryNamespace:%s",
		o.SourceID, o.RegistryHost, o.RegistryNamespace)
}

type RegistryClient interface {
	ImageExists(ctx context.Context, opts RegistryAuthOptions) (bool, error)
	SaveImageToFile(ctx context.Context, opts RegistrySaveOptions) error
	// SaveImageToWriter saves a registry image to a writer (for streaming)
	SaveImageToWriter(ctx context.Context, opts RegistrySaveOptions) error
	BuildDestImageID(opts BuildDestOptions) string
}
