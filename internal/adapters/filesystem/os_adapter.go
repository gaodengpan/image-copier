package filesystem

import (
	"os"

	"github.com/gaodengpan/image-copier/internal/adapters/errors"
	"github.com/gaodengpan/image-copier/internal/ports"
)

type OSAdapter struct{}

func NewOSAdapter() *OSAdapter {
	return &OSAdapter{}
}

func (a *OSAdapter) CreateTempFile(pattern string) (string, error) {
	tmpFile, err := os.CreateTemp("/tmp/", pattern)
	if err != nil {
		return "", errors.NewFileSystemError("CreateTempFile", "failed to create temp file", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	return tmpPath, nil
}

func (a *OSAdapter) RemoveFile(path string) error {
	if err := os.Remove(path); err != nil {
		return errors.NewFileSystemError("RemoveFile", "failed to remove file", err)
	}
	return nil
}

var _ ports.FileSystem = (*OSAdapter)(nil)
