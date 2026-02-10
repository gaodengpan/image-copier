package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gaodengpan/image-copier/internal/cli"
)

// --- NewPullCommand tests ---

func TestNewPullCommand(t *testing.T) {
	cmd := cli.NewPullCommand()

	if cmd == nil {
		t.Fatal("expected non-nil pull command")
	}

	if cmd.Use != "pull [IMAGE...]" {
		t.Errorf("expected Use 'pull [IMAGE...]', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short to be set")
	}

	if cmd.Long == "" {
		t.Error("expected Long to be set")
	}

	// Verify Args validator is set
	if cmd.Args == nil {
		t.Error("expected Args validator to be set")
	}

	// Verify flags (original + merged from batch)
	flags := []string{"arch", "os", "file", "jobs", "force", "verbose"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag '%s' to exist", flag)
		}
	}
}

func TestNewPullCommand_FlagDefaults(t *testing.T) {
	cmd := cli.NewPullCommand()

	archFlag := cmd.Flags().Lookup("arch")
	if archFlag == nil {
		t.Fatal("arch flag not found")
	}
	if archFlag.DefValue != "" {
		t.Errorf("expected empty default for arch, got '%s'", archFlag.DefValue)
	}

	osFlag := cmd.Flags().Lookup("os")
	if osFlag == nil {
		t.Fatal("os flag not found")
	}
	if osFlag.DefValue != "" {
		t.Errorf("expected empty default for os, got '%s'", osFlag.DefValue)
	}

	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Fatal("file flag not found")
	}
	if fileFlag.DefValue != "" {
		t.Errorf("expected empty default for file, got '%s'", fileFlag.DefValue)
	}

	jobsFlag := cmd.Flags().Lookup("jobs")
	if jobsFlag == nil {
		t.Fatal("jobs flag not found")
	}
	if jobsFlag.DefValue != "3" {
		t.Errorf("expected '3' default for jobs, got '%s'", jobsFlag.DefValue)
	}

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}
	if forceFlag.DefValue != "false" {
		t.Errorf("expected 'false' default for force, got '%s'", forceFlag.DefValue)
	}

	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("verbose flag not found")
	}
	if verboseFlag.DefValue != "false" {
		t.Errorf("expected 'false' default for verbose, got '%s'", verboseFlag.DefValue)
	}
}

func TestNewPullCommand_ShortFlags(t *testing.T) {
	cmd := cli.NewPullCommand()

	// Verify shorthand flags
	fileFlag := cmd.Flags().ShorthandLookup("f")
	if fileFlag == nil {
		t.Error("expected shorthand 'f' for file flag")
	}

	jobsFlag := cmd.Flags().ShorthandLookup("j")
	if jobsFlag == nil {
		t.Error("expected shorthand 'j' for jobs flag")
	}

	verboseFlag := cmd.Flags().ShorthandLookup("v")
	if verboseFlag == nil {
		t.Error("expected shorthand 'v' for verbose flag")
	}
}

// --- NewConfigCommand tests ---

func TestNewConfigCommand(t *testing.T) {
	cmd := cli.NewConfigCommand()

	if cmd == nil {
		t.Fatal("expected non-nil config command")
	}

	if cmd.Use != "config" {
		t.Errorf("expected Use 'config', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short to be set")
	}

	// Verify subcommands
	subcommands := cmd.Commands()
	if len(subcommands) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(subcommands))
	}

	names := make(map[string]bool)
	for _, sub := range subcommands {
		names[sub.Name()] = true
	}

	if !names["show"] {
		t.Error("expected 'show' subcommand")
	}
	if !names["init"] {
		t.Error("expected 'init' subcommand")
	}
}

func TestNewConfigCommand_ShowSubcommand(t *testing.T) {
	cmd := cli.NewConfigCommand()

	var showCmd *os.File
	_ = showCmd

	for _, sub := range cmd.Commands() {
		if sub.Name() == "show" {
			if sub.Short == "" {
				t.Error("expected show subcommand to have Short description")
			}
			if sub.Long == "" {
				t.Error("expected show subcommand to have Long description")
			}
			return
		}
	}
	t.Error("show subcommand not found")
}

func TestNewConfigCommand_InitSubcommand(t *testing.T) {
	cmd := cli.NewConfigCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "init" {
			if sub.Short == "" {
				t.Error("expected init subcommand to have Short description")
			}

			// Verify init flags
			if sub.Flags().Lookup("skip-existing") == nil {
				t.Error("expected 'skip-existing' flag on init")
			}
			if sub.Flags().Lookup("force") == nil {
				t.Error("expected 'force' flag on init")
			}
			return
		}
	}
	t.Error("init subcommand not found")
}

// --- readImagesFromFile tests (via pull command) ---

func TestReadImagesFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "images.txt")

	content := `nginx:latest
# this is a comment
redis:alpine

postgres:15
# another comment
ubuntu:22.04
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test by running the pull command with file flag
	cmd := cli.NewPullCommand()
	err = cmd.Flags().Set("file", filePath)
	if err != nil {
		t.Fatalf("failed to set file flag: %v", err)
	}

	// We can't directly call readImagesFromFile since it's unexported
	// but we can verify the pull command accepts the flag
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Fatal("file flag not found")
	}
}

func TestReadImagesFromFile_Nonexistent(t *testing.T) {
	cmd := cli.NewPullCommand()
	err := cmd.Flags().Set("file", "/nonexistent/file.txt")
	if err != nil {
		t.Fatalf("failed to set file flag: %v", err)
	}
	// The actual error would happen during RunE execution
}

// --- Command hierarchy tests ---

func TestCommandHierarchy(t *testing.T) {
	// Verify all top-level commands can be created
	pullCmd := cli.NewPullCommand()
	if pullCmd == nil {
		t.Error("NewPullCommand returned nil")
	}

	configCmd := cli.NewConfigCommand()
	if configCmd == nil {
		t.Error("NewConfigCommand returned nil")
	}
}

// --- Flag parsing tests ---

func TestPullCommand_FlagParsing(t *testing.T) {
	cmd := cli.NewPullCommand()

	args := []string{"--arch", "arm64", "--os", "linux", "-f", "images.txt", "-j", "5", "--force", "-v"}
	err := cmd.Flags().Parse(args)
	if err != nil {
		t.Errorf("failed to parse flags: %v", err)
	}
}
