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
	"sync"
	"testing"

	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/config/strategies"
)

func TestConfigManager_Load(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a test configuration file
	configPath := filepath.Join(tmpDir, "test-cluster-config.yaml")
	configContent := `schema_version: "1.0"
cluster:
  name: test-cluster
  fqdn: test.example.com
opencenter:
  infrastructure:
    provider: openstack
  meta:
    region: us-east-1
    env: dev
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create ConfigManager and register strategies
	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())
	manager.RegisterStrategy(strategies.NewV2Strategy())
	manager.RegisterStrategy(strategies.NewLegacyStrategy())

	tests := []struct {
		name    string
		path    string
		opts    LoadOptions
		wantErr bool
	}{
		{
			name: "load valid v1 config",
			path: configPath,
			opts: LoadOptions{
				AutoMigrate: false,
				Validate:    false,
				SkipCache:   false,
			},
			wantErr: false,
		},
		{
			name: "load from cache",
			path: configPath,
			opts: LoadOptions{
				AutoMigrate: false,
				Validate:    false,
				SkipCache:   false,
			},
			wantErr: false,
		},
		{
			name: "skip cache",
			path: configPath,
			opts: LoadOptions{
				AutoMigrate: false,
				Validate:    false,
				SkipCache:   true,
			},
			wantErr: false,
		},
		{
			name: "non-existent file",
			path: filepath.Join(tmpDir, "nonexistent.yaml"),
			opts: LoadOptions{
				AutoMigrate: false,
				Validate:    false,
				SkipCache:   false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := manager.Load(tt.path, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cfg == nil {
				t.Error("Load() returned nil config without error")
			}
		})
	}
}

func TestConfigManager_Save(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewConfigManager()

	// Create a minimal valid config
	validConfig := internalconfig.NewDefault("test-cluster")

	tests := []struct {
		name    string
		path    string
		config  *internalconfig.Config
		wantErr bool
	}{
		{
			name:    "save valid config",
			path:    filepath.Join(tmpDir, "save-test-config.yaml"),
			config:  &validConfig,
			wantErr: false,
		},
		{
			name:    "nil config",
			path:    filepath.Join(tmpDir, "nil-config.yaml"),
			config:  nil,
			wantErr: true,
		},
		{
			name:    "empty path",
			path:    "",
			config:  &validConfig,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Save(tt.path, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify file was created with correct permissions
			if !tt.wantErr {
				info, err := os.Stat(tt.path)
				if err != nil {
					t.Errorf("Failed to stat saved file: %v", err)
					return
				}
				if info.Mode().Perm() != 0600 {
					t.Errorf("File permissions = %o, want 0600", info.Mode().Perm())
				}
			}
		})
	}
}

func TestConfigManager_InvalidateCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cache-test-config.yaml")

	// Create test config
	configContent := `schema_version: "1.0"
cluster:
  name: cache-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())

	// Load config to populate cache
	_, err := manager.Load(configPath, LoadOptions{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify it's in cache
	if cached := manager.getFromCache(configPath); cached == nil {
		t.Error("Config not found in cache after Load()")
	}

	// Invalidate cache
	manager.InvalidateCache(configPath)

	// Verify it's removed from cache
	if cached := manager.getFromCache(configPath); cached != nil {
		t.Error("Config still in cache after InvalidateCache()")
	}
}

func TestConfigManager_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "thread-test-config.yaml")

	// Create test config
	configContent := `schema_version: "1.0"
cluster:
  name: thread-test
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())

	// Run concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent loads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.Load(configPath, LoadOptions{})
			if err != nil {
				t.Errorf("Concurrent Load() failed: %v", err)
			}
		}()
	}

	// Concurrent cache invalidations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.InvalidateCache(configPath)
		}()
	}

	// Concurrent strategy registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Create a mock strategy with unique version
			strategy := &mockStrategy{version: "test-" + string(rune('0'+idx))}
			manager.RegisterStrategy(strategy)
		}(i)
	}

	wg.Wait()
}

func TestConfigManager_ExtractClusterName(t *testing.T) {
	manager := NewConfigManager()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "standard config file",
			path: "/path/to/clusters/org/test-cluster-config.yaml",
			want: "test-cluster",
		},
		{
			name: "hidden config file",
			path: "/path/to/clusters/org/.test-cluster-config.yaml",
			want: "test-cluster",
		},
		{
			name: "no config suffix",
			path: "/path/to/clusters/org/test-cluster.yaml",
			want: "test-cluster",
		},
		{
			name: "simple filename",
			path: "my-cluster-config.yaml",
			want: "my-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.extractClusterName(tt.path)
			if got != tt.want {
				t.Errorf("extractClusterName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigManager_RegisterStrategy(t *testing.T) {
	manager := NewConfigManager()

	strategy := &mockStrategy{version: "test-1.0"}
	manager.RegisterStrategy(strategy)

	// Verify strategy was registered
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if _, exists := manager.strategies["test-1.0"]; !exists {
		t.Error("Strategy was not registered")
	}
}

// mockStrategy is a mock implementation of LoadStrategy for testing
type mockStrategy struct {
	version string
}

func (m *mockStrategy) CanLoad(data []byte) (bool, error) {
	return true, nil
}

func (m *mockStrategy) Load(data []byte, clusterName string) (*internalconfig.Config, error) {
	cfg := internalconfig.NewDefault(clusterName)
	cfg.SchemaVersion = m.version
	return &cfg, nil
}

func (m *mockStrategy) Version() string {
	return m.version
}
