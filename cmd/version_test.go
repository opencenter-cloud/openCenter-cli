// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	// Save original values
	origVersion := Version
	origGitCommit := GitCommit
	origGitBranch := GitBranch
	origGitTag := GitTag
	origBuildDate := BuildDate

	// Restore after test
	defer func() {
		Version = origVersion
		GitCommit = origGitCommit
		GitBranch = origGitBranch
		GitTag = origGitTag
		BuildDate = origBuildDate
	}()

	tests := []struct {
		name           string
		version        string
		gitCommit      string
		gitBranch      string
		gitTag         string
		buildDate      string
		shortFlag      bool
		expectedOutput []string
	}{
		{
			name:           "full version with tag",
			version:        "1.0.0",
			gitCommit:      "abc123def456",
			gitBranch:      "main",
			gitTag:         "v1.0.0",
			buildDate:      "2025-11-07T20:00:00Z",
			shortFlag:      false,
			expectedOutput: []string{"v1.0.0", "abc123def456", "main", "2025-11-07T20:00:00Z"},
		},
		{
			name:           "full version without tag",
			version:        "0.0.1",
			gitCommit:      "abc123def456",
			gitBranch:      "develop",
			gitTag:         "",
			buildDate:      "2025-11-07T20:00:00Z",
			shortFlag:      false,
			expectedOutput: []string{"0.0.1-abc123d", "abc123def456", "develop", "2025-11-07T20:00:00Z"},
		},
		{
			name:           "short version with tag",
			version:        "1.0.0",
			gitCommit:      "abc123def456",
			gitBranch:      "main",
			gitTag:         "v1.0.0",
			buildDate:      "2025-11-07T20:00:00Z",
			shortFlag:      true,
			expectedOutput: []string{"v1.0.0"},
		},
		{
			name:           "short version without tag",
			version:        "0.0.1",
			gitCommit:      "abc123def456",
			gitBranch:      "main",
			gitTag:         "",
			buildDate:      "2025-11-07T20:00:00Z",
			shortFlag:      true,
			expectedOutput: []string{"0.0.1-abc123d"},
		},
		{
			name:           "dev version",
			version:        "dev",
			gitCommit:      "unknown",
			gitBranch:      "unknown",
			gitTag:         "",
			buildDate:      "unknown",
			shortFlag:      false,
			expectedOutput: []string{"dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test values
			Version = tt.version
			GitCommit = tt.gitCommit
			GitBranch = tt.gitBranch
			GitTag = tt.gitTag
			BuildDate = tt.buildDate

			// Create command
			cmd := NewVersionCmd()
			if tt.shortFlag {
				cmd.SetArgs([]string{"--short"})
			}

			// Capture output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// Execute command
			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check output
			output := buf.String()
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestGetVersionString(t *testing.T) {
	// Save original values
	origVersion := Version
	origGitCommit := GitCommit
	origGitTag := GitTag

	// Restore after test
	defer func() {
		Version = origVersion
		GitCommit = origGitCommit
		GitTag = origGitTag
	}()

	tests := []struct {
		name      string
		version   string
		gitCommit string
		gitTag    string
		expected  string
	}{
		{
			name:      "with tag",
			version:   "1.0.0",
			gitCommit: "abc123def456",
			gitTag:    "v1.0.0",
			expected:  "v1.0.0",
		},
		{
			name:      "without tag",
			version:   "0.0.1",
			gitCommit: "abc123def456",
			gitTag:    "",
			expected:  "0.0.1-abc123d",
		},
		{
			name:      "dev version",
			version:   "dev",
			gitCommit: "unknown",
			gitTag:    "",
			expected:  "dev",
		},
		{
			name:      "short commit",
			version:   "0.0.1",
			gitCommit: "abc",
			gitTag:    "",
			expected:  "0.0.1", // Too short for commit suffix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			GitCommit = tt.gitCommit
			GitTag = tt.gitTag

			result := getVersionString()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
