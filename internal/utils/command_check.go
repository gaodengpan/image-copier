package utils

import (
	"fmt"
	"os/exec"
)

// CheckCommandExists verifies if a command is available in the system PATH.
// It uses exec.LookPath which safely searches for the executable without
// invoking a shell, preventing command injection vulnerabilities.
func CheckCommandExists(command string) (bool, error) {
	// Use exec.LookPath to safely find the command in PATH
	// This avoids shell injection by not using shell interpretation
	path, err := exec.LookPath(command)
	if err != nil {
		// Command not found in PATH
		return false, nil
	}

	// If we got a non-empty path, the command exists
	return path != "", nil
}

// CheckCommandsExist verifies multiple commands exist in the system PATH
func CheckCommandsExist(commands []string) map[string]bool {
	results := make(map[string]bool)

	for _, command := range commands {
		exists, _ := CheckCommandExists(command)
		results[command] = exists
	}

	return results
}

// GetMissingCommands returns a list of commands that are missing from the system PATH
func GetMissingCommands(requiredCommands []string) ([]string, error) {
	missing := []string{}

	for _, command := range requiredCommands {
		exists, err := CheckCommandExists(command)
		if err != nil {
			return nil, fmt.Errorf("error checking command %s: %w", command, err)
		}

		if !exists {
			missing = append(missing, command)
		}
	}

	return missing, nil
}
