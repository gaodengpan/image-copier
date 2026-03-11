package cli

import (
	"bufio"
	"fmt"
	"strings"
)

// promptString prompts for a string value
func promptString(reader *bufio.Reader, prompt, defaultValue, placeholder string) string {
	displayPrompt := prompt
	if defaultValue != "" {
		if placeholder == "" {
			displayPrompt += fmt.Sprintf(" [%s]", defaultValue)
		}
	} else if placeholder != "" {
		displayPrompt += fmt.Sprintf(" %s", placeholder)
	}
	displayPrompt += ": "

	fmt.Print(displayPrompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" || input == placeholder {
		return defaultValue
	}
	return input
}

// promptYesNo prompts for a yes/no answer
func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	displayPrompt := prompt
	if defaultYes {
		displayPrompt += " [Y/n]: "
	} else {
		displayPrompt += " [y/N]: "
	}

	for {
		fmt.Print(displayPrompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return defaultYes
		}
		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}
		fmt.Println("Please enter 'y' or 'n'")
	}
}

// maskToken masks a token with asterisks, leaving only first/last characters visible
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
