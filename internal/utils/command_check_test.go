package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCommandExists(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "bash exists",
			command: "bash",
			want:    true,
		},
		{
			name:    "ls exists",
			command: "ls",
			want:    true,
		},
		{
			name:    "nonexistent command",
			command: "thiscommanddoesnotexist12345",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckCommandExists(tt.command)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckCommandsExist(t *testing.T) {
	commands := []string{"bash", "ls", "nonexistent12345"}
	results := CheckCommandsExist(commands)

	assert.Len(t, results, 3)
	assert.True(t, results["bash"])
	assert.True(t, results["ls"])
	assert.False(t, results["nonexistent12345"])
}

func TestGetMissingCommands(t *testing.T) {
	tests := []struct {
		name      string
		commands  []string
		wantLen   int
		wantEmpty bool
	}{
		{
			name:      "all exist",
			commands:  []string{"bash", "ls"},
			wantLen:   0,
			wantEmpty: true,
		},
		{
			name:      "none exist",
			commands:  []string{"nonexist1", "nonexist2"},
			wantLen:   2,
			wantEmpty: false,
		},
		{
			name:      "mixed",
			commands:  []string{"bash", "nonexist"},
			wantLen:   1,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, err := GetMissingCommands(tt.commands)
			assert.NoError(t, err)
			assert.Len(t, missing, tt.wantLen)
		})
	}
}

// TestCheckCommandExists_Security tests that malicious command inputs
// do not result in command injection. These inputs should be treated
// as literal command names to search for in PATH.
func TestCheckCommandExists_Security(t *testing.T) {
	// These malicious inputs should NOT execute any commands
	// They should simply be treated as non-existent command names
	maliciousInputs := []struct {
		name    string
		command string
		desc    string
	}{
		{
			name:    "semicolon injection",
			command: "ls; rm -rf /",
			desc:    "semicolons should not chain commands",
		},
		{
			name:    "double ampersand injection",
			command: "ls && echo pwned",
			desc:    "&& should not chain commands",
		},
		{
			name:    "pipe injection",
			command: "ls | cat",
			desc:    "pipes should not work",
		},
		{
			name:    "backtick injection",
			command: "ls`whoami`",
			desc:    "backticks should not execute commands",
		},
		{
			name:    "dollar substitution",
			command: "ls$(whoami)",
			desc:    "dollar substitution should not execute",
		},
		{
			name:    "newline injection",
			command: "ls\necho pwned",
			desc:    "newlines should not split commands",
		},
		{
			name:    "redirect injection",
			command: "ls > /tmp/pwned",
			desc:    "redirects should not create files",
		},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			// The function should safely handle malicious input
			// It should return false (command not found) without error
			// Most importantly, it should NOT execute any injected commands
			got, err := CheckCommandExists(tt.command)

			// Should not return an error
			assert.NoError(t, err, "CheckCommandExists should not error on malicious input: %s", tt.desc)

			// Should return false since these are not valid command names in PATH
			assert.False(t, got, "Malicious input should be treated as non-existent command: %s", tt.desc)
		})
	}
}
