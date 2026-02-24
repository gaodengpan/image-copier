package value_objects

import (
	"errors"
	"strings"
)

type Architecture string

const (
	ArchAMD64 Architecture = "amd64"
	ArchARM64 Architecture = "arm64"
	ArchARM   Architecture = "arm"
	Arch386   Architecture = "386"
)

var validArchitectures = []Architecture{
	ArchAMD64,
	ArchARM64,
	ArchARM,
	Arch386,
}

func NewArchitecture(arch string) (*Architecture, error) {
	if arch == "" {
		return nil, errors.New("architecture is required")
	}

	a := Architecture(strings.ToLower(arch))

	if !a.IsValid() {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &a, nil
}

func (a *Architecture) IsValid() bool {
	if a == nil {
		return false
	}
	for _, valid := range validArchitectures {
		if *a == valid {
			return true
		}
	}
	return false
}

func (a *Architecture) String() string {
	if a == nil {
		return ""
	}
	return string(*a)
}
