package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gaodengpan/image-copier/internal/infrastructure/config"
)

// RunWizard runs the interactive configuration wizard
func RunWizard(ctx context.Context, skipExisting bool, provider config.ConfigProvider) (*config.ConfigData, error) {
	reader := bufio.NewReader(os.Stdin)

	// Try to load existing config to provide defaults
	var existing *config.Config
	if skipExisting {
		if cfg, err := provider.Load(); err == nil {
			existing = cfg
		}
	}

	data := &config.ConfigData{
		GitHubWorkflowID: "image-copier-v2.yaml",
		RegistryArch:     "amd64",
		RegistryOs:       "linux",
	}

	// If existing config, use those as defaults
	if existing != nil {
		data.GitHubOwner = existing.Github.Owner
		data.GitHubRepo = existing.Github.Repo
		data.GitHubToken = maskToken(existing.Github.Token)
		data.GitHubWorkflowID = existing.Github.WorkflowID

		data.RegistryHost = existing.Registry.Host
		data.RegistryUsername = existing.Registry.Username
		data.RegistryPassword = maskToken(existing.Registry.Password)
		data.RegistryNamespace = existing.Registry.Namespace
		data.RegistryArch = existing.Registry.Arch
		data.RegistryOs = existing.Registry.Os

		fmt.Println("\nDetected existing configuration. Press Enter to keep current value.")
	}

	fmt.Println("\n=== Image Copier Configuration Wizard ===")

	// GitHub Configuration
	fmt.Println("--- GitHub Configuration ---")

	data.GitHubOwner = promptString(reader, "GitHub repository owner (user or organization)",
		data.GitHubOwner, "")
	data.GitHubRepo = promptString(reader, "GitHub repository name",
		data.GitHubRepo, "")

	data.GitHubToken = promptString(reader, "GitHub personal access token with workflow permissions",
		data.GitHubToken, "(empty to skip validation)")

	// Validate GitHub token if provided
	if data.GitHubToken != "" && !strings.Contains(data.GitHubToken, "*") {
		fmt.Print("Validating GitHub token... ")
		if err := validateGitHubToken(ctx, data.GitHubOwner, data.GitHubRepo, data.GitHubToken); err != nil {
			fmt.Printf("Warning: %v\n", err)
			continueWarn := promptYesNo(reader, "Continue anyway?", false)
			if !continueWarn {
				return nil, fmt.Errorf("configuration cancelled")
			}
		} else {
			fmt.Println("OK")
		}
	}

	data.GitHubWorkflowID = promptString(reader, "Workflow filename",
		data.GitHubWorkflowID, "")

	// Registry Configuration
	fmt.Println("\n--- Registry Configuration ---")

	data.RegistryHost = promptString(reader, "Domestic registry host (e.g., registry.cn-hangzhou.aliyuncs.com)",
		data.RegistryHost, "")
	data.RegistryUsername = promptString(reader, "Registry username",
		data.RegistryUsername, "")
	data.RegistryPassword = promptString(reader, "Registry password or access token",
		data.RegistryPassword, "(leave masked to keep existing)")

	data.RegistryNamespace = promptString(reader,
		"Optional namespace for organizing images (leave empty to skip)",
		data.RegistryNamespace, "")

	data.RegistryArch = promptString(reader, "Default architecture",
		data.RegistryArch, "")
	data.RegistryOs = promptString(reader, "Default operating system",
		data.RegistryOs, "")

	// Private Registries Configuration
	fmt.Println("\n--- Private Registries Configuration ---")
	addMore := promptYesNo(reader, "Add a private registry?", false)
	for addMore {
		var privReg config.PrivateRegistry
		privReg.Name = promptString(reader, "Private registry name (e.g., harbor)",
			"", "")
		privReg.Host = promptString(reader, "Private registry host (e.g., harbor.internal.com)",
			"", "")
		privReg.Username = promptString(reader, "Private registry username",
			"", "")
		privReg.Password = promptString(reader, "Private registry password",
			"", "(leave masked to keep existing)")

		if privReg.Name != "" && privReg.Host != "" {
			data.PrivateRegistries = append(data.PrivateRegistries, privReg)
		}

		addMore = promptYesNo(reader, "Add another private registry?", false)
	}

	return data, nil
}
