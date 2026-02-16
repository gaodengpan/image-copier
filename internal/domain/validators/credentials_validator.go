package validators

import "strings"

const shellChars = "$`\"'\\;&|()<>()[]{}"

type CredentialsValidator struct{}

func NewCredentialsValidator() *CredentialsValidator {
	return &CredentialsValidator{}
}

func (v *CredentialsValidator) Validate(username, password string) bool {
	if containsDangerous(username) || containsDangerous(password) {
		return false
	}

	if len(username) == 0 || len(password) == 0 {
		return false
	}

	if len(username) > 1000 || len(password) > 1000 {
		return false
	}

	return true
}

func containsDangerous(input string) bool {
	if strings.ContainsAny(input, "\n\r") {
		return true
	}
	return strings.ContainsAny(input, shellChars)
}
