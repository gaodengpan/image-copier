package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckCommandExists verifies if a command is available in the system PATH
func CheckCommandExists(command string) (bool, error) {
	// Use 'command -v' to check if the command exists in PATH
	cmd := exec.Command("sh", "-c", "command -v "+command)
	output, err := cmd.Output()

	if err != nil {
		// Command not found
		return false, nil
	}

	// Trim whitespace and check if we got a valid path
	commandPath := strings.TrimSpace(string(output))
	if commandPath == "" {
		return false, nil
	}

	return true, nil
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