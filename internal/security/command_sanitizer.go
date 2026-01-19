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
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommandSanitizer prevents command injection in external command execution
type CommandSanitizer interface {
	SanitizeCommand(cmd string, args []string) (*exec.Cmd, error)
	ValidateEditor(editor string) error
	SanitizeGitArgs(args []string) ([]string, error)
}

// DefaultCommandSanitizer implements CommandSanitizer interface
type DefaultCommandSanitizer struct {
	// safeEditors is a whitelist of allowed editors
	safeEditors map[string]bool
	// shellMetachars are characters that should be rejected in command names
	shellMetachars []string
}

// NewDefaultCommandSanitizer creates a new command sanitizer with default settings
func NewDefaultCommandSanitizer() *DefaultCommandSanitizer {
	return &DefaultCommandSanitizer{
		safeEditors: map[string]bool{
			"vim":   true,
			"vi":    true,
			"nvim":  true,
			"nano":  true,
			"emacs": true,
			"code":  true,
			"subl":  true,
			"atom":  true,
			"gedit": true,
		},
		shellMetachars: []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"},
	}
}

// SanitizeCommand creates a safe exec.Cmd using parameterized execution
// Requirements: 1.3, 1.4
//
// This function prevents command injection by:
// 1. Rejecting command names with shell metacharacters
// 2. Using exec.Command with separate arguments instead of shell invocation
// 3. Never using sh -c or bash -c with user input
func (s *DefaultCommandSanitizer) SanitizeCommand(cmd string, args []string) (*exec.Cmd, error) {
	if cmd == "" {
		return nil, &CommandSanitizationError{
			Command: cmd,
			Message: "command name cannot be empty",
		}
	}

	// Check for shell metacharacters in command name
	for _, metachar := range s.shellMetachars {
		if strings.Contains(cmd, metachar) {
			return nil, &CommandSanitizationError{
				Command: cmd,
				Message: fmt.Sprintf("command name contains shell metacharacter: %s", metachar),
			}
		}
	}

	// Reject shell invocation commands
	cmdBase := filepath.Base(cmd)
	if cmdBase == "sh" || cmdBase == "bash" || cmdBase == "zsh" || cmdBase == "fish" {
		// Check if this is a shell invocation with -c flag
		if len(args) > 0 && (args[0] == "-c" || args[0] == "--command") {
			return nil, &CommandSanitizationError{
				Command: cmd,
				Message: "shell invocation with -c flag is not allowed (use parameterized execution instead)",
			}
		}
	}

	// Create command with separate arguments (parameterized execution)
	// This is safe because exec.Command does not invoke a shell
	execCmd := exec.Command(cmd, args...)

	return execCmd, nil
}

// ValidateEditor validates an editor command against the safe editors whitelist
// Requirements: 1.1, 1.2
//
// This function ensures that only known-safe editors can be used, preventing
// command injection through the EDITOR environment variable.
func (s *DefaultCommandSanitizer) ValidateEditor(editor string) error {
	if editor == "" {
		return nil // Empty is acceptable, system will use default
	}

	// Check for shell metacharacters first
	for _, metachar := range s.shellMetachars {
		if strings.Contains(editor, metachar) {
			return &CommandSanitizationError{
				Command: editor,
				Message: fmt.Sprintf("editor value contains shell metacharacter: %s", metachar),
			}
		}
	}

	// Extract just the command name (remove path and arguments)
	editorCmd := filepath.Base(editor)
	editorCmd = strings.Split(editorCmd, " ")[0]

	if !s.safeEditors[editorCmd] {
		return &CommandSanitizationError{
			Command: editor,
			Message: fmt.Sprintf("editor '%s' is not in the safe editors whitelist (allowed: vim, vi, nvim, nano, emacs, code, subl, atom, gedit)", editorCmd),
		}
	}

	return nil
}

// SanitizeGitArgs sanitizes arguments for Git commands
// Requirements: 1.3, 1.4
//
// This function escapes special characters in Git arguments to prevent
// command injection through Git operations.
func (s *DefaultCommandSanitizer) SanitizeGitArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	sanitized := make([]string, len(args))

	for i, arg := range args {
		// Check for dangerous metacharacters that should be rejected
		dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
		for _, char := range dangerousChars {
			if strings.Contains(arg, char) {
				return nil, &CommandSanitizationError{
					Command: "git",
					Message: fmt.Sprintf("git argument contains dangerous shell metacharacter: %s", char),
				}
			}
		}

		// For arguments that look like they might be user-controlled values
		// (not flags starting with -), wrap in quotes and escape special chars
		if !strings.HasPrefix(arg, "-") {
			// Escape special characters
			escaped := arg
			escaped = strings.ReplaceAll(escaped, "$", "\\$")
			escaped = strings.ReplaceAll(escaped, "<", "\\<")
			escaped = strings.ReplaceAll(escaped, ">", "\\>")
			escaped = strings.ReplaceAll(escaped, "(", "\\(")
			escaped = strings.ReplaceAll(escaped, ")", "\\)")
			escaped = strings.ReplaceAll(escaped, "{", "\\{")
			escaped = strings.ReplaceAll(escaped, "}", "\\}")

			sanitized[i] = escaped
		} else {
			// Flags are passed through as-is
			sanitized[i] = arg
		}
	}

	return sanitized, nil
}

// CommandSanitizationError represents a command sanitization error
type CommandSanitizationError struct {
	Command string
	Message string
}

// Error implements the error interface
func (e *CommandSanitizationError) Error() string {
	// Mask the command if it's too long
	displayCommand := e.Command
	if len(displayCommand) > 50 {
		displayCommand = displayCommand[:20] + "..." + displayCommand[len(displayCommand)-10:]
	}
	return fmt.Sprintf("command sanitization failed for '%s': %s", displayCommand, e.Message)
}

// IsCommandSanitizationError checks if an error is a CommandSanitizationError
func IsCommandSanitizationError(err error) bool {
	_, ok := err.(*CommandSanitizationError)
	return ok
}
