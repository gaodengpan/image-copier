package value_objects

import (
	"fmt"
)

var ErrInvalidCredentials = fmt.Errorf("invalid credentials: username and password are required")

type Credentials struct {
	username string
	password string
}

func NewCredentials(username, password string) (*Credentials, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	if len(username) > 1000 || len(password) > 1000 {
		return nil, fmt.Errorf("invalid credentials: username or password too long")
	}
	return &Credentials{username: username, password: password}, nil
}

func (c *Credentials) Username() string { return c.username }
func (c *Credentials) Password() string { return c.password }

func (c *Credentials) Masked() string {
	if len(c.username) <= 4 {
		return "****"
	}
	return c.username[:2] + "**" + c.username[len(c.username)-2:]
}

func (c *Credentials) Format() string {
	return fmt.Sprintf("%s:%s", c.username, c.password)
}
