package value_objects

import (
	"errors"
	"strings"
)

type OperatingSystem string

const (
	OSLinux   OperatingSystem = "linux"
	OSDarwin  OperatingSystem = "darwin"
	OSWindows OperatingSystem = "windows"
)

var validOperatingSystems = []OperatingSystem{
	OSLinux,
	OSDarwin,
	OSWindows,
}

func NewOperatingSystem(os string) (*OperatingSystem, error) {
	if os == "" {
		return nil, errors.New("operating system is required")
	}

	o := OperatingSystem(strings.ToLower(os))

	if !o.IsValid() {
		return nil, errors.New("invalid operating system: " + os)
	}

	return &o, nil
}

func (o *OperatingSystem) IsValid() bool {
	if o == nil {
		return false
	}
	for _, valid := range validOperatingSystems {
		if *o == valid {
			return true
		}
	}
	return false
}

func (o *OperatingSystem) String() string {
	if o == nil {
		return ""
	}
	return string(*o)
}
