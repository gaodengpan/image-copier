package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckDockerService checks if the Docker service is running
func CheckDockerService() (bool, error) {
	// Use 'docker info' to check if Docker service is running
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Docker service is not accessible or not running
		return false, fmt.Errorf("docker service check failed: %s, output: %s", err.Error(), string(output))
	}

	outputStr := string(output)
	if strings.Contains(strings.ToLower(outputStr), "error") || strings.Contains(strings.ToLower(outputStr), "daemon") {
		// Additional checks for common error patterns in output
		return false, fmt.Errorf("docker service reported errors: %s", outputStr)
	}

	return true, nil
}

// GetDockerVersion returns the Docker client and server version information
func GetDockerVersion() (string, error) {
	cmd := exec.Command("docker", "--version")
	output, err := cmd.Output()

	if err != nil {
		return "", fmt.Errorf("could not get docker version: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckDockerConnectivity tests connectivity to the Docker daemon
func CheckDockerConnectivity() error {
	cmd := exec.Command("docker", "ps")
	_, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("docker connectivity test failed: %w", err)
	}

	return nil
}
