package value_objects

import "fmt"

const credentialsSeparator = ":"

type Credentials struct {
	username string
	password string
}

func NewCredentials(username, password string) (*Credentials, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	v := NewImageIDValidator()
	if !v.ValidateCredentials(username, password) {
		return nil, fmt.Errorf("invalid credentials: contains dangerous characters")
	}

	return &Credentials{
		username: username,
		password: password,
	}, nil
}

func (c *Credentials) Username() string {
	return c.username
}

func (c *Credentials) Password() string {
	return c.password
}

func (c *Credentials) FormatForSkopeo() string {
	return c.username + credentialsSeparator + c.password
}

func (c *Credentials) FormatForDocker() string {
	return c.username + credentialsSeparator + c.password
}
