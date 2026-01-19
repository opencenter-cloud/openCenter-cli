/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package security

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 2: Command Execution Uses Parameterized Calls
// For any external command execution, the system SHALL use exec.Command with separate arguments rather
// than shell invocation, preventing command injection.
// **Validates: Requirements 1.3, 1.4**
func TestProperty_CommandExecutionUsesParameterizedCalls(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	sanitizer := NewDefaultCommandSanitizer()

	// Property 2.1: Commands with shell metacharacters are rejected
	properties.Property("commands with shell metacharacters are rejected", prop.ForAll(
		func(cmd string) bool {
			// If command contains shell metacharacters, it must be rejected
			shellMetachars := []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"}
			for _, metachar := range shellMetachars {
				if strings.Contains(cmd, metachar) {
					_, err := sanitizer.SanitizeCommand(cmd, []string{})
					return err != nil && IsCommandSanitizationError(err)
				}
			}
			return true // Commands without metacharacters may pass or fail based on other rules
		},
		gen.AnyString(),
	))

	// Property 2.2: Shell invocation with -c flag is blocked
	properties.Property("shell invocation with -c flag is blocked", prop.ForAll(
		func(shellCmd string) bool {
			// Test various shell commands with -c flag
			shells := []string{"sh", "bash", "zsh", "fish", "/bin/sh", "/bin/bash", "/usr/bin/zsh"}
			for _, shell := range shells {
				_, err := sanitizer.SanitizeCommand(shell, []string{"-c", shellCmd})
				if err == nil || !IsCommandSanitizationError(err) {
					return false // Must reject shell -c invocation
				}
			}
			return true
		},
		gen.AnyString(),
	))

	// Property 2.3: Shell invocation with --command flag is blocked
	properties.Property("shell invocation with --command flag is blocked", prop.ForAll(
		func(shellCmd string) bool {
			// Test various shell commands with --command flag
			shells := []string{"sh", "bash", "zsh", "fish"}
			for _, shell := range shells {
				_, err := sanitizer.SanitizeCommand(shell, []string{"--command", shellCmd})
				if err == nil || !IsCommandSanitizationError(err) {
					return false // Must reject shell --command invocation
				}
			}
			return true
		},
		gen.AnyString(),
	))

	// Property 2.4: Empty command names are rejected
	properties.Property("empty command names are rejected", prop.ForAll(
		func(args []string) bool {
			_, err := sanitizer.SanitizeCommand("", args)
			return err != nil && IsCommandSanitizationError(err)
		},
		gen.SliceOf(gen.AlphaString()),
	))

	// Property 2.5: Valid commands without metacharacters are accepted
	properties.Property("valid commands without metacharacters are accepted", prop.ForAll(
		func(cmd string) bool {
			// Generate a valid command name (alphanumeric and safe chars only)
			if cmd == "" {
				return true // Skip empty
			}

			validCmd := genValidCommandName(cmd)
			if validCmd == "" {
				return true // Skip if we couldn't generate a valid command
			}

			// Valid commands should be accepted
			execCmd, err := sanitizer.SanitizeCommand(validCmd, []string{})
			return err == nil && execCmd != nil
		},
		gen.AlphaString(),
	))

	// Property 2.6: Parameterized execution preserves arguments
	properties.Property("parameterized execution preserves arguments", prop.ForAll(
		func(args []string) bool {
			// Use a safe command
			validCmd := "echo"

			// Filter out empty args
			filteredArgs := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != "" {
					filteredArgs = append(filteredArgs, arg)
				}
			}

			execCmd, err := sanitizer.SanitizeCommand(validCmd, filteredArgs)
			if err != nil {
				return false
			}

			// Verify arguments are preserved (command + args)
			cmdArgs := execCmd.Args
			if len(cmdArgs) != len(filteredArgs)+1 {
				return false
			}

			// First arg should be the command itself
			if cmdArgs[0] != validCmd {
				return false
			}

			// Remaining args should match
			for i, arg := range filteredArgs {
				if cmdArgs[i+1] != arg {
					return false
				}
			}

			return true
		},
		gen.SliceOf(gen.AlphaString()),
	))

	// Property 2.7: EDITOR validation rejects shell metacharacters
	properties.Property("EDITOR validation rejects shell metacharacters", prop.ForAll(
		func(editor string) bool {
			// If editor contains shell metacharacters, it must be rejected
			shellMetachars := []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"}
			for _, metachar := range shellMetachars {
				if strings.Contains(editor, metachar) {
					err := sanitizer.ValidateEditor(editor)
					return err != nil && IsCommandSanitizationError(err)
				}
			}
			return true // Editors without metacharacters may pass or fail based on whitelist
		},
		gen.AnyString(),
	))

	// Property 2.8: EDITOR validation accepts whitelisted editors
	properties.Property("EDITOR validation accepts whitelisted editors", prop.ForAll(
		func(_ bool) bool {
			// Test all whitelisted editors
			safeEditors := []string{"vim", "vi", "nvim", "nano", "emacs", "code", "subl", "atom", "gedit"}
			for _, editor := range safeEditors {
				err := sanitizer.ValidateEditor(editor)
				if err != nil {
					return false
				}
			}
			return true
		},
		gen.Const(true),
	))

	// Property 2.9: EDITOR validation rejects non-whitelisted editors
	properties.Property("EDITOR validation rejects non-whitelisted editors", prop.ForAll(
		func(editor string) bool {
			// Skip empty (empty is allowed)
			if editor == "" {
				return true
			}

			// Generate a non-whitelisted editor name
			safeEditors := map[string]bool{
				"vim": true, "vi": true, "nvim": true, "nano": true,
				"emacs": true, "code": true, "subl": true, "atom": true, "gedit": true,
			}

			// If the editor is not in the whitelist and doesn't contain metacharacters
			editorBase := strings.Split(editor, " ")[0]
			editorBase = strings.TrimPrefix(editorBase, "/usr/bin/")
			editorBase = strings.TrimPrefix(editorBase, "/bin/")

			if !safeEditors[editorBase] {
				// Make sure it doesn't contain metacharacters (those are tested separately)
				shellMetachars := []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"}
				hasMetachar := false
				for _, metachar := range shellMetachars {
					if strings.Contains(editor, metachar) {
						hasMetachar = true
						break
					}
				}

				if !hasMetachar {
					err := sanitizer.ValidateEditor(editor)
					return err != nil && IsCommandSanitizationError(err)
				}
			}
			return true // Whitelisted editors should pass (tested in other property)
		},
		gen.AlphaString(),
	))

	// Property 2.10: Git args with dangerous metacharacters are rejected
	properties.Property("git args with dangerous metacharacters are rejected", prop.ForAll(
		func(arg string) bool {
			// If arg contains dangerous metacharacters, it must be rejected
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, char := range dangerousChars {
				if strings.Contains(arg, char) {
					_, err := sanitizer.SanitizeGitArgs([]string{arg})
					return err != nil && IsCommandSanitizationError(err)
				}
			}
			return true // Args without dangerous chars should be sanitized successfully
		},
		gen.AnyString(),
	))

	// Property 2.11: Git args without dangerous chars are sanitized successfully
	properties.Property("git args without dangerous chars are sanitized successfully", prop.ForAll(
		func(arg string) bool {
			// If arg doesn't contain dangerous characters, sanitization should succeed
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			hasDangerous := false
			for _, char := range dangerousChars {
				if strings.Contains(arg, char) {
					hasDangerous = true
					break
				}
			}

			if !hasDangerous {
				sanitized, err := sanitizer.SanitizeGitArgs([]string{arg})
				return err == nil && sanitized != nil && len(sanitized) == 1
			}
			return true // Dangerous args should be rejected (tested in other property)
		},
		gen.AlphaString(),
	))

	// Property 2.12: Git flags are preserved without modification
	properties.Property("git flags are preserved without modification", prop.ForAll(
		func(flag string) bool {
			// Generate a valid flag (starts with -)
			if flag == "" {
				return true
			}

			validFlag := "-" + genValidFlagName(flag)

			// Flags should be passed through as-is
			sanitized, err := sanitizer.SanitizeGitArgs([]string{validFlag})
			if err != nil {
				return false
			}

			return len(sanitized) == 1 && sanitized[0] == validFlag
		},
		gen.AlphaString(),
	))

	// Property 2.13: Git args escape special characters
	properties.Property("git args escape special characters", prop.ForAll(
		func(arg string) bool {
			// Skip empty args
			if arg == "" {
				return true
			}

			// Skip args with dangerous characters (those should be rejected)
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, char := range dangerousChars {
				if strings.Contains(arg, char) {
					return true
				}
			}

			// Skip flags (they're not escaped)
			if strings.HasPrefix(arg, "-") {
				return true
			}

			// For non-flag args with special chars, they should be escaped
			specialChars := []string{"$", "<", ">", "(", ")", "{", "}"}
			hasSpecial := false
			for _, char := range specialChars {
				if strings.Contains(arg, char) {
					hasSpecial = true
					break
				}
			}

			if hasSpecial {
				sanitized, err := sanitizer.SanitizeGitArgs([]string{arg})
				if err != nil {
					return false
				}

				// Verify that special characters are escaped
				for _, char := range specialChars {
					if strings.Contains(arg, char) {
						// The sanitized version should have the escaped version
						if !strings.Contains(sanitized[0], "\\"+char) {
							return false
						}
					}
				}
			}

			return true
		},
		gen.AnyString(),
	))

	// Property 2.14: Empty git args are handled correctly
	properties.Property("empty git args are handled correctly", prop.ForAll(
		func(_ bool) bool {
			sanitized, err := sanitizer.SanitizeGitArgs([]string{})
			return err == nil && len(sanitized) == 0
		},
		gen.Const(true),
	))

	// Property 2.15: Multiple git args are all sanitized
	properties.Property("multiple git args are all sanitized", prop.ForAll(
		func(args []string) bool {
			// Filter out args with dangerous characters
			safeArgs := make([]string, 0, len(args))
			dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
			for _, arg := range args {
				hasDangerous := false
				for _, char := range dangerousChars {
					if strings.Contains(arg, char) {
						hasDangerous = true
						break
					}
				}
				if !hasDangerous && arg != "" {
					safeArgs = append(safeArgs, arg)
				}
			}

			if len(safeArgs) == 0 {
				return true
			}

			sanitized, err := sanitizer.SanitizeGitArgs(safeArgs)
			if err != nil {
				return false
			}

			// Should have same number of args
			return len(sanitized) == len(safeArgs)
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper functions for generating valid test data

// genValidCommandName generates a valid command name from arbitrary input
func genValidCommandName(input string) string {
	if input == "" {
		return ""
	}

	// Keep only alphanumeric, hyphen, underscore, and forward slash (for paths)
	var result strings.Builder
	for _, ch := range input {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '/' {
			result.WriteRune(ch)
		}
	}

	cmd := result.String()
	if cmd == "" {
		return "echo" // Default safe command
	}
	return cmd
}

// genValidFlagName generates a valid flag name from arbitrary input
func genValidFlagName(input string) string {
	if input == "" {
		return "flag"
	}

	// Keep only alphanumeric and hyphen
	var result strings.Builder
	for _, ch := range input {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' {
			result.WriteRune(ch)
		}
	}

	flag := result.String()
	if flag == "" {
		return "flag"
	}
	return flag
}
