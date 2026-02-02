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

package config

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
)

// TestLoad_DelegatesCorrectly tests that Load delegates to internal/config.Load
func TestLoad_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create a v2 config using the old implementation
	cfg := internalconfig.NewDefault("test-cluster")
	cfg.SchemaVersion = "2.0"
	err := internalconfig.Save(cfg)
	require.NoError(t, err)
	
	// Load using the new implementation
	loadedCfg, err := Load("test-cluster")
	require.NoError(t, err)
	assert.NotNil(t, loadedCfg)
	assert.Equal(t, "2.0", loadedCfg.SchemaVersion)
	assert.Equal(t, "test-cluster", loadedCfg.ClusterName())
}

// TestLoad_RejectsV1Config tests that Load rejects v1 configurations
func TestLoad_RejectsV1Config(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create a v1 config
	cfg := internalconfig.NewDefault("test-v1-cluster")
	cfg.SchemaVersion = "1.0"
	err := internalconfig.Save(cfg)
	require.NoError(t, err)
	
	// Attempt to load using the new implementation
	_, err = Load("test-v1-cluster")
	require.Error(t, err)
	
	// Should be a V1ConfigError
	var v1Err *V1ConfigError
	assert.ErrorAs(t, err, &v1Err)
}

// TestLoad_RejectsUnsupportedVersion tests that Load rejects unsupported schema versions
func TestLoad_RejectsUnsupportedVersion(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create a config with unsupported version
	cfg := internalconfig.NewDefault("test-unsupported-cluster")
	cfg.SchemaVersion = "3.0"
	err := internalconfig.Save(cfg)
	require.NoError(t, err)
	
	// Attempt to load using the new implementation
	_, err = Load("test-unsupported-cluster")
	require.Error(t, err)
	
	// Should be an UnsupportedVersionError
	var unsupportedErr *UnsupportedVersionError
	assert.ErrorAs(t, err, &unsupportedErr)
	assert.Equal(t, "3.0", unsupportedErr.Version)
}

// TestSave_DelegatesCorrectly tests that Save delegates to internal/config.Save
func TestSave_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create a config
	cfg := Config(internalconfig.NewDefault("test-save-cluster"))
	cfg.SchemaVersion = "2.0"
	
	// Save using the new implementation
	err := Save(&cfg)
	require.NoError(t, err)
	
	// Verify the file was created
	configPath := filepath.Join(tmpDir, "test-save-cluster.yaml")
	assert.FileExists(t, configPath)
	
	// Load it back to verify
	loadedCfg, err := Load("test-save-cluster")
	require.NoError(t, err)
	assert.Equal(t, "test-save-cluster", loadedCfg.ClusterName())
}

// TestConfigPath_DelegatesCorrectly tests that ConfigPath delegates correctly
func TestConfigPath_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create a config
	cfg := internalconfig.NewDefault("test-path-cluster")
	err := internalconfig.Save(cfg)
	require.NoError(t, err)
	
	// Get path using the new implementation
	path, err := ConfigPath("test-path-cluster")
	require.NoError(t, err)
	assert.Contains(t, path, "test-path-cluster")
	assert.FileExists(t, path)
}

// TestResolveConfigDir_DelegatesCorrectly tests that ResolveConfigDir delegates correctly
func TestResolveConfigDir_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Resolve using the new implementation
	dir, err := ResolveConfigDir()
	require.NoError(t, err)
	assert.Equal(t, tmpDir, dir)
}

// TestList_DelegatesCorrectly tests that List delegates correctly
func TestList_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory for test configs
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Create multiple configs
	for _, name := range []string{"cluster1", "cluster2", "cluster3"} {
		cfg := internalconfig.NewDefault(name)
		err := internalconfig.Save(cfg)
		require.NoError(t, err)
	}
	
	// List using the new implementation
	clusters, err := List()
	require.NoError(t, err)
	assert.Len(t, clusters, 3)
	assert.Contains(t, clusters, "cluster1")
	assert.Contains(t, clusters, "cluster2")
	assert.Contains(t, clusters, "cluster3")
}

// TestSetActive_DelegatesCorrectly tests that SetActive delegates correctly
func TestSetActive_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Set active using the new implementation
	err := SetActive("test-active-cluster")
	require.NoError(t, err)
	
	// Verify using GetActive
	active, err := GetActive()
	require.NoError(t, err)
	assert.Equal(t, "test-active-cluster", active)
}

// TestGetActive_DelegatesCorrectly tests that GetActive delegates correctly
func TestGetActive_DelegatesCorrectly(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")
	
	// Set active using old implementation
	err := internalconfig.SetActive("test-get-active-cluster")
	require.NoError(t, err)
	
	// Get active using the new implementation
	active, err := GetActive()
	require.NoError(t, err)
	assert.Equal(t, "test-get-active-cluster", active)
}
