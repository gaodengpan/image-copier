package value_objects

import (
	"errors"
	"strings"
)

type RegistryCredentials struct {
	username string
	password string
}

func NewRegistryCredentials(username, password string) (*RegistryCredentials, error) {
	if username == "" {
		return nil, errors.New("username required")
	}
	if password == "" {
		return nil, errors.New("password required")
	}
	if strings.ContainsAny(username, "\n\r") || strings.ContainsAny(password, "\n\r") {
		return nil, errors.New("credentials contain invalid characters")
	}
	return &RegistryCredentials{
		username: username,
		password: password,
	}, nil
}

func (c *RegistryCredentials) Username() string {
	return c.username
}

func (c *RegistryCredentials) Password() string {
	return c.password
}

func (c *RegistryCredentials) String() string {
	return "***:***"
}
