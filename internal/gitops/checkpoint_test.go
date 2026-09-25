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

package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointTempPrefixSiblingCreateAndRestore(t *testing.T) {
	rootDir := t.TempDir()
	tempDir := filepath.Join(rootDir, ".tmp")
	siblingDir := tempDir + "-sibling"
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatalf("create temp prefix sibling: %v", err)
	}

	siblingFile := filepath.Join(siblingDir, "state.txt")
	if err := os.WriteFile(siblingFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("write sibling file: %v", err)
	}

	workspace := &GitOpsWorkspace{
		RootDir:     rootDir,
		TempDir:     tempDir,
		Checkpoints: make(map[string]WorkspaceCheckpoint),
	}
	checkpoint, err := workspace.CreateCheckpoint("prefix-boundary")
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	relSiblingFile, err := filepath.Rel(rootDir, siblingFile)
	if err != nil {
		t.Fatalf("relative sibling path: %v", err)
	}
	if len(checkpoint.Files) != 1 || checkpoint.Files[0] != relSiblingFile {
		t.Fatalf("checkpoint files = %v, want [%s]", checkpoint.Files, relSiblingFile)
	}

	if err := os.WriteFile(siblingFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modify sibling file: %v", err)
	}
	if err := workspace.RestoreCheckpoint("prefix-boundary"); err != nil {
		t.Fatalf("RestoreCheckpoint() error = %v", err)
	}

	content, err := os.ReadFile(siblingFile)
	if err != nil {
		t.Fatalf("read restored sibling file: %v", err)
	}
	if got, want := string(content), "original"; got != want {
		t.Errorf("restored sibling content = %q, want %q", got, want)
	}
}

func TestIsPathWithin(t *testing.T) {
	base := filepath.Join(t.TempDir(), "temp")
	tests := []struct {
		name   string
		target string
		within bool
	}{
		{name: "base", target: base, within: true},
		{name: "child", target: filepath.Join(base, "file"), within: true},
		{name: "prefix sibling", target: base + "-sibling", within: false},
		{name: "parent", target: filepath.Dir(base), within: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathWithin(base, tt.target); got != tt.within {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v", base, tt.target, got, tt.within)
			}
		})
	}
}
