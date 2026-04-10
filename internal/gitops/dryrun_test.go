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

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDryRunWorkspace_NoFilesystemChanges verifies that dry-run mode doesn't create any files or directories.
// This is the core test for Property 17: Dry-Run Filesystem Safety.
func TestDryRunWorkspace_NoFilesystemChanges(t *testing.T) {
	// Create a test configuration
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}

	// Create a dry-run workspace
	dryRunWS := NewDryRunWorkspace(cfg)

	// Verify the simulated paths don't actually exist
	assert.NoDirExists(t, dryRunWS.RootDir, "Dry-run root directory should not be created")
	assert.NoDirExists(t, dryRunWS.TempDir, "Dry-run temp directory should not be created")

	// Create a dry-run writer
	writer := NewDryRunAtomicWriter(dryRunWS, "test-stage")

	// Perform various operations
	err := writer.MkdirAll("test-dir", 0o755)
	require.NoError(t, err)

	err = writer.WriteFileString("test-file.txt", "test content", 0o644)
	require.NoError(t, err)

	err = writer.MkdirAll("another-dir/nested", 0o755)
	require.NoError(t, err)

	err = writer.WriteFileString("another-dir/file.yaml", "yaml: content", 0o644)
	require.NoError(t, err)

	// Verify no actual filesystem changes occurred
	assert.NoDirExists(t, dryRunWS.RootDir, "Dry-run should not create root directory")
	assert.NoDirExists(t, filepath.Join(dryRunWS.RootDir, "test-dir"), "Dry-run should not create directories")
	assert.NoFileExists(t, filepath.Join(dryRunWS.RootDir, "test-file.txt"), "Dry-run should not create files")

	// Verify operations were recorded
	ops := dryRunWS.GetOperations()
	assert.Len(t, ops, 4, "Should have recorded 4 operations")

	// Verify files were tracked
	files := dryRunWS.GetFiles()
	assert.Len(t, files, 2, "Should have tracked 2 files")
	assert.Contains(t, files, "test-file.txt")
	assert.Contains(t, files, "another-dir/file.yaml")

	// Verify directories were tracked
	dirs := dryRunWS.GetDirectories()
	assert.Len(t, dirs, 2, "Should have tracked 2 directories")
}

// TestDryRunWorkspace_AccuratePreview verifies that dry-run provides accurate information.
// This tests the "accurate preview" part of Property 17.
func TestDryRunWorkspace_AccuratePreview(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}
	dryRunWS := NewDryRunWorkspace(cfg)
	writer := NewDryRunAtomicWriter(dryRunWS, "preview-stage")

	// Create some content
	testContent := "This is test content for preview"
	err := writer.WriteFileString("preview.txt", testContent, 0o644)
	require.NoError(t, err)

	// Verify the content is accurately recorded
	files := dryRunWS.GetFiles()
	require.Contains(t, files, "preview.txt")

	file := files["preview.txt"]
	assert.Equal(t, testContent, file.Content, "Content should be accurately recorded")
	assert.Equal(t, int64(len(testContent)), file.Size, "Size should be accurate")
	assert.Equal(t, os.FileMode(0o644), file.Mode, "Mode should be accurate")
	assert.Equal(t, "preview-stage", file.CreatedBy, "Stage should be tracked")
}

// TestDryRunWorkspace_Exists verifies that the Exists method works correctly in dry-run mode.
func TestDryRunWorkspace_Exists(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}
	dryRunWS := NewDryRunWorkspace(cfg)
	writer := NewDryRunAtomicWriter(dryRunWS, "exists-test")

	// Initially nothing exists
	assert.False(t, dryRunWS.Exists("test.txt"))
	assert.False(t, dryRunWS.Exists("test-dir"))

	// Create a file
	err := writer.WriteFileString("test.txt", "content", 0o644)
	require.NoError(t, err)

	// Now it should exist
	assert.True(t, dryRunWS.Exists("test.txt"))

	// Create a directory
	err = writer.MkdirAll("test-dir", 0o755)
	require.NoError(t, err)

	// Now it should exist
	assert.True(t, dryRunWS.Exists("test-dir"))

	// Create a nested file
	err = writer.WriteFileString("test-dir/nested.txt", "nested", 0o644)
	require.NoError(t, err)

	// Parent directory should exist
	assert.True(t, dryRunWS.Exists("test-dir"))
	assert.True(t, dryRunWS.Exists("test-dir/nested.txt"))
}

// TestDryRunWorkspace_Summary verifies the summary generation.
func TestDryRunWorkspace_Summary(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}
	dryRunWS := NewDryRunWorkspace(cfg)
	writer := NewDryRunAtomicWriter(dryRunWS, "summary-test")

	// Create some content
	err := writer.MkdirAll("dir1", 0o755)
	require.NoError(t, err)

	err = writer.WriteFileString("file1.txt", "content1", 0o644)
	require.NoError(t, err)

	err = writer.WriteFileString("file2.txt", "content2", 0o644)
	require.NoError(t, err)

	// Generate summary
	summary := dryRunWS.GenerateSummary()

	assert.Equal(t, dryRunWS.ID, summary.WorkspaceID)
	assert.Equal(t, 3, summary.TotalOperations)
	assert.Equal(t, 2, summary.FilesCreated)
	assert.Equal(t, 1, summary.DirectoriesCreated)
	assert.Greater(t, summary.TotalSize, int64(0))
}

// TestAtomicWriter_DryRunMode verifies that AtomicWriter detects and uses dry-run mode.
func TestAtomicWriter_DryRunMode(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}
	dryRunWS := NewDryRunWorkspace(cfg)

	// Create a GitOpsWorkspace with dry-run metadata
	workspace := &GitOpsWorkspace{
		ID:           dryRunWS.ID,
		RootDir:      dryRunWS.RootDir,
		TempDir:      dryRunWS.TempDir,
		Config:       dryRunWS.Config,
		Metadata:     make(map[string]interface{}),
		Checkpoints:  make(map[string]WorkspaceCheckpoint),
		createdAt:    dryRunWS.createdAt,
		lastModified: dryRunWS.lastModified,
	}

	// Set dry-run metadata
	workspace.SetMetadata("is_dryrun", true)
	workspace.SetMetadata("dryrun_workspace", dryRunWS)

	// Create atomic writer (should detect dry-run mode)
	writer := NewAtomicWriter(workspace)
	writer.SetStage("atomic-test")

	// Verify it's in dry-run mode
	assert.True(t, writer.IsDryRun(), "Writer should be in dry-run mode")

	// Perform operations
	err := writer.MkdirAll("atomic-dir", 0o755)
	require.NoError(t, err)

	err = writer.WriteFileString("atomic-file.txt", "atomic content", 0o644)
	require.NoError(t, err)

	// Verify no filesystem changes
	assert.NoDirExists(t, filepath.Join(workspace.RootDir, "atomic-dir"))
	assert.NoFileExists(t, filepath.Join(workspace.RootDir, "atomic-file.txt"))

	// Verify operations were recorded
	ops := dryRunWS.GetOperations()
	assert.Len(t, ops, 2)

	// Verify stage tracking
	for _, op := range ops {
		assert.Equal(t, "atomic-test", op.Stage)
	}
}

// TestDryRunWorkspace_Checkpoints verifies checkpoint operations in dry-run mode.
func TestDryRunWorkspace_Checkpoints(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
		},
	}
	dryRunWS := NewDryRunWorkspace(cfg)
	writer := NewDryRunAtomicWriter(dryRunWS, "checkpoint-test")

	// Create some initial content
	err := writer.WriteFileString("file1.txt", "content1", 0o644)
	require.NoError(t, err)

	// Create a checkpoint
	checkpoint, err := dryRunWS.CreateCheckpoint("test-checkpoint")
	require.NoError(t, err)
	assert.Equal(t, "test-checkpoint", checkpoint.ID)
	assert.Len(t, checkpoint.Files, 1)

	// Add more content
	err = writer.WriteFileString("file2.txt", "content2", 0o644)
	require.NoError(t, err)

	assert.Len(t, dryRunWS.GetFiles(), 2)

	// Restore checkpoint
	err = dryRunWS.RestoreCheckpoint("test-checkpoint")
	require.NoError(t, err)

	// Should have restored to checkpoint state
	assert.Len(t, dryRunWS.GetFiles(), 1)

	// Delete checkpoint
	err = dryRunWS.DeleteCheckpoint("test-checkpoint")
	require.NoError(t, err)

	assert.NotContains(t, dryRunWS.Checkpoints, "test-checkpoint")
}
