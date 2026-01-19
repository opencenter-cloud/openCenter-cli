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
	"strings"
	"testing"
)

func TestSanitizeCommand(t *testing.T) {
	sanitizer := NewDefaultCommandSanitizer()

	tests := []struct {
		name        string
		cmd         string
		args        []string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid command with args",
			cmd:         "git",
			args:        []string{"clone", "https://example.com/repo.git"},
			shouldError: false,
		},
		{
			name:        "valid command without args",
			cmd:         "ls",
			args:        []string{},
			shouldError: false,
		},
		{
			name:        "empty command",
			cmd:         "",
			args:        []string{"arg1"},
			shouldError: true,
			errorMsg:    "command name cannot be empty",
		},
		{
			name:        "command with semicolon",
			cmd:         "ls; rm -rf /",
			args:        []string{},
			shouldError: true,
			errorMsg:    "shell metacharacter: ;",
		},
		{
			name:        "command with pipe",
			cmd:         "cat file | grep secret",
			args:        []string{},
			shouldError: true,
			errorMsg:    "shell metacharacter: |",
		},
		{
			name:        "command with ampersand",
			cmd:         "sleep 10 &",
			args:        []string{},
			shouldError: true,
			errorMsg:    "shell metacharacter: &",
		},
		{
			name:        "command with backtick",
			cmd:         "echo `whoami`",
			args:        []string{},
			shouldError: true,
			errorMsg:    "shell metacharacter: `",
		},
		{
			name:        "command with dollar sign",
			cmd:         "echo $HOME",
			args:        []string{},
			shouldError: true,
			errorMsg:    "shell metacharacter: $",
		},
		{
			name:        "shell invocation with -c flag",
			cmd:         "sh",
			args:        []string{"-c", "rm -rf /"},
			shouldError: true,
			errorMsg:    "shell invocation with -c flag is not allowed",
		},
		{
			name:        "bash invocation with -c flag",
			cmd:         "bash",
			args:        []string{"-c", "malicious command"},
			shouldError: true,
			errorMsg:    "shell invocation with -c flag is not allowed",
		},
		{
			name:        "bash invocation with --command flag",
			cmd:         "/bin/bash",
			args:        []string{"--command", "malicious command"},
			shouldError: true,
			errorMsg:    "shell invocation with -c flag is not allowed",
		},
		{
			name:        "valid bash script execution",
			cmd:         "bash",
			args:        []string{"script.sh"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := sanitizer.SanitizeCommand(tt.cmd, tt.args)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if !IsCommandSanitizationError(err) {
					t.Errorf("expected CommandSanitizationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if cmd == nil {
					t.Errorf("expected non-nil command")
					return
				}
				// Verify the command uses parameterized execution
				if cmd.Path == "" {
					t.Errorf("expected command path to be set")
				}
			}
		})
	}
}

func TestValidateEditor(t *testing.T) {
	sanitizer := NewDefaultCommandSanitizer()

	tests := []struct {
		name        string
		editor      string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "empty editor",
			editor:      "",
			shouldError: false,
		},
		{
			name:        "vim",
			editor:      "vim",
			shouldError: false,
		},
		{
			name:        "vi",
			editor:      "vi",
			shouldError: false,
		},
		{
			name:        "nvim",
			editor:      "nvim",
			shouldError: false,
		},
		{
			name:        "nano",
			editor:      "nano",
			shouldError: false,
		},
		{
			name:        "emacs",
			editor:      "emacs",
			shouldError: false,
		},
		{
			name:        "code",
			editor:      "code",
			shouldError: false,
		},
		{
			name:        "subl",
			editor:      "subl",
			shouldError: false,
		},
		{
			name:        "atom",
			editor:      "atom",
			shouldError: false,
		},
		{
			name:        "gedit",
			editor:      "gedit",
			shouldError: false,
		},
		{
			name:        "vim with path",
			editor:      "/usr/bin/vim",
			shouldError: false,
		},
		{
			name:        "unsafe editor",
			editor:      "malicious-editor",
			shouldError: true,
			errorMsg:    "not in the safe editors whitelist",
		},
		{
			name:        "editor with semicolon",
			editor:      "vim; rm -rf /",
			shouldError: true,
			errorMsg:    "shell metacharacter: ;",
		},
		{
			name:        "editor with pipe",
			editor:      "vim | cat",
			shouldError: true,
			errorMsg:    "shell metacharacter: |",
		},
		{
			name:        "editor with ampersand",
			editor:      "vim &",
			shouldError: true,
			errorMsg:    "shell metacharacter: &",
		},
		{
			name:        "editor with backtick",
			editor:      "vim `whoami`",
			shouldError: true,
			errorMsg:    "shell metacharacter: `",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizer.ValidateEditor(tt.editor)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if !IsCommandSanitizationError(err) {
					t.Errorf("expected CommandSanitizationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSanitizeGitArgs(t *testing.T) {
	sanitizer := NewDefaultCommandSanitizer()

	tests := []struct {
		name        string
		args        []string
		expected    []string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "empty args",
			args:        []string{},
			expected:    []string{},
			shouldError: false,
		},
		{
			name:        "simple flags",
			args:        []string{"-a", "-b", "-c"},
			expected:    []string{"-a", "-b", "-c"},
			shouldError: false,
		},
		{
			name:        "flags with values",
			args:        []string{"--message", "commit message", "--author", "John Doe"},
			expected:    []string{"--message", "commit message", "--author", "John Doe"},
			shouldError: false,
		},
		{
			name:        "args with special chars",
			args:        []string{"clone", "repo$(whoami)"},
			expected:    []string{"clone", "repo\\$\\(whoami\\)"},
			shouldError: false,
		},
		{
			name:        "args with angle brackets",
			args:        []string{"commit", "-m", "fix <bug>"},
			expected:    []string{"commit", "-m", "fix \\<bug\\>"},
			shouldError: false,
		},
		{
			name:        "args with parentheses",
			args:        []string{"tag", "v1.0.0(beta)"},
			expected:    []string{"tag", "v1.0.0\\(beta\\)"},
			shouldError: false,
		},
		{
			name:        "args with braces",
			args:        []string{"commit", "-m", "fix {issue}"},
			expected:    []string{"commit", "-m", "fix \\{issue\\}"},
			shouldError: false,
		},
		{
			name:        "args with semicolon",
			args:        []string{"commit", "-m", "message; rm -rf /"},
			shouldError: true,
			errorMsg:    "dangerous shell metacharacter: ;",
		},
		{
			name:        "args with pipe",
			args:        []string{"log", "--format=%H | grep secret"},
			shouldError: true,
			errorMsg:    "dangerous shell metacharacter: |",
		},
		{
			name:        "args with ampersand",
			args:        []string{"push", "origin master &"},
			shouldError: true,
			errorMsg:    "dangerous shell metacharacter: &",
		},
		{
			name:        "args with backtick",
			args:        []string{"commit", "-m", "`whoami`"},
			shouldError: true,
			errorMsg:    "dangerous shell metacharacter: `",
		},
		{
			name:        "args with newline",
			args:        []string{"commit", "-m", "line1\nline2"},
			shouldError: true,
			errorMsg:    "dangerous shell metacharacter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizer.SanitizeGitArgs(tt.args)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if !IsCommandSanitizationError(err) {
					t.Errorf("expected CommandSanitizationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if len(result) != len(tt.expected) {
					t.Errorf("expected %d args, got %d", len(tt.expected), len(result))
					return
				}
				for i := range result {
					if result[i] != tt.expected[i] {
						t.Errorf("arg %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
					}
				}
			}
		})
	}
}

func TestCommandSanitizationError(t *testing.T) {
	err := &CommandSanitizationError{
		Command: "malicious-command",
		Message: "test error message",
	}

	errorStr := err.Error()
	if !strings.Contains(errorStr, "malicious-command") {
		t.Errorf("error message should contain command name")
	}
	if !strings.Contains(errorStr, "test error message") {
		t.Errorf("error message should contain message")
	}

	// Test long command masking
	longCmd := strings.Repeat("a", 100)
	err2 := &CommandSanitizationError{
		Command: longCmd,
		Message: "test",
	}
	errorStr2 := err2.Error()
	if len(errorStr2) > 200 {
		t.Errorf("error message should mask long commands")
	}
	if strings.Contains(errorStr2, longCmd) {
		t.Errorf("error message should not contain full long command")
	}
}

func TestIsCommandSanitizationError(t *testing.T) {
	err := &CommandSanitizationError{
		Command: "test",
		Message: "test",
	}

	if !IsCommandSanitizationError(err) {
		t.Errorf("IsCommandSanitizationError should return true for CommandSanitizationError")
	}

	if IsCommandSanitizationError(nil) {
		t.Errorf("IsCommandSanitizationError should return false for nil")
	}

	// Test with fmt.Errorf
	otherErr := fmt.Errorf("some other error")
	if IsCommandSanitizationError(otherErr) {
		t.Errorf("IsCommandSanitizationError should return false for other error types")
	}
}
