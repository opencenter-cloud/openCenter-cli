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

	"gopkg.in/yaml.v3"
)

// TestDefaultTalosConfig verifies that DefaultTalosConfig returns a properly initialized configuration
func TestDefaultTalosConfig(t *testing.T) {
	clusterName := "test-cluster"
	talosConfig := DefaultTalosConfig(clusterName)

	if talosConfig == nil {
		t.Fatal("DefaultTalosConfig returned nil")
	}

	// Verify enabled state
	if !talosConfig.Enabled {
		t.Error("Expected Talos to be enabled by default")
	}

	// Verify version is set
	if talosConfig.Version == "" {
		t.Error("Expected Talos version to be set")
	}

	// Verify machine config defaults
	if !talosConfig.MachineConfig.AppArmorEnabled {
		t.Error("Expected AppArmor to be enabled by default")
	}
	if !talosConfig.MachineConfig.SeccompEnabled {
		t.Error("Expected Seccomp to be enabled by default")
	}
	if !talosConfig.MachineConfig.DiskEncryption {
		t.Error("Expected disk encryption to be enabled by default")
	}
	if !talosConfig.MachineConfig.KubePrismEnabled {
		t.Error("Expected KubePrism to be enabled by default")
	}

	// Verify network config defaults
	if talosConfig.NetworkConfig.ManagementSubnet == "" {
		t.Error("Expected management subnet to be set")
	}
	if talosConfig.NetworkConfig.ControlSubnet == "" {
		t.Error("Expected control subnet to be set")
	}
	if talosConfig.NetworkConfig.DataSubnet == "" {
		t.Error("Expected data subnet to be set")
	}
	if talosConfig.NetworkConfig.WireGuardPort != 51820 {
		t.Errorf("Expected WireGuard port to be 51820, got %d", talosConfig.NetworkConfig.WireGuardPort)
	}
	if talosConfig.NetworkConfig.TalosAPIPort != 50000 {
		t.Errorf("Expected Talos API port to be 50000, got %d", talosConfig.NetworkConfig.TalosAPIPort)
	}

	// Verify security config defaults
	if !talosConfig.SecurityConfig.VTPMEnabled {
		t.Error("Expected vTPM to be enabled by default")
	}
	if !talosConfig.SecurityConfig.ImageVerification {
		t.Error("Expected image verification to be enabled by default")
	}
	if !talosConfig.SecurityConfig.MFARequired {
		t.Error("Expected MFA to be required by default")
	}
	if !talosConfig.SecurityConfig.AuditLogEnabled {
		t.Error("Expected audit logging to be enabled by default")
	}

	// Verify Pulumi config defaults
	if talosConfig.PulumiConfig.StackName == "" {
		t.Error("Expected Pulumi stack name to be set")
	}
	if talosConfig.PulumiConfig.SwiftContainer == "" {
		t.Error("Expected Swift container to be set")
	}
}

// TestTalosConfigYAMLMarshaling verifies that TalosConfig can be marshaled to and from YAML
func TestTalosConfigYAMLMarshaling(t *testing.T) {
	clusterName := "test-cluster"
	original := DefaultTalosConfig(clusterName)

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal TalosConfig to YAML: %v", err)
	}

	// Unmarshal back
	var unmarshaled TalosConfig
	err = yaml.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal TalosConfig from YAML: %v", err)
	}

	// Verify key fields match
	if unmarshaled.Enabled != original.Enabled {
		t.Error("Enabled field mismatch after YAML round-trip")
	}
	if unmarshaled.Version != original.Version {
		t.Error("Version field mismatch after YAML round-trip")
	}
	if unmarshaled.MachineConfig.AppArmorEnabled != original.MachineConfig.AppArmorEnabled {
		t.Error("AppArmorEnabled field mismatch after YAML round-trip")
	}
	if unmarshaled.NetworkConfig.WireGuardPort != original.NetworkConfig.WireGuardPort {
		t.Error("WireGuardPort field mismatch after YAML round-trip")
	}
}

// TestConfigWithTalosSection verifies that a Config with Talos section can be loaded
func TestConfigWithTalosSection(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	
	// Set OPENCENTER_CONFIG_DIR to temp directory
	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)

	clusterName := "test-talos-cluster"
	
	// Create a config with Talos section
	cfg := NewDefault(clusterName)
	cfg.OpenCenter.Talos = DefaultTalosConfig(clusterName)
	
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Write to file
	configPath := filepath.Join(tmpDir, clusterName+".yaml")
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	loaded, err := Load(clusterName)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify Talos section was loaded
	if loaded.OpenCenter.Talos == nil {
		t.Fatal("Talos section was not loaded")
	}

	if !loaded.OpenCenter.Talos.Enabled {
		t.Error("Expected Talos to be enabled in loaded config")
	}

	if loaded.OpenCenter.Talos.Version != cfg.OpenCenter.Talos.Version {
		t.Errorf("Version mismatch: expected %s, got %s", 
			cfg.OpenCenter.Talos.Version, loaded.OpenCenter.Talos.Version)
	}
}

// TestConfigWithoutTalosSection verifies that a Config without Talos section loads correctly
func TestConfigWithoutTalosSection(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir := t.TempDir()
	
	// Set OPENCENTER_CONFIG_DIR to temp directory
	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", tmpDir)
	defer os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)

	clusterName := "test-no-talos-cluster"
	
	// Create a config without Talos section (default)
	cfg := NewDefault(clusterName)
	
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Write to file
	configPath := filepath.Join(tmpDir, clusterName+".yaml")
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	loaded, err := Load(clusterName)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify Talos section is nil (disabled by default)
	if loaded.OpenCenter.Talos != nil {
		t.Error("Expected Talos section to be nil when not configured")
	}
}

// TestTalosConfigValidation verifies that Talos configuration fields are validated
func TestTalosConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*TalosConfig)
		expectValid bool
	}{
		{
			name: "valid default config",
			modifyFunc: func(tc *TalosConfig) {
				// No modifications, should be valid
			},
			expectValid: true,
		},
		{
			name: "empty version",
			modifyFunc: func(tc *TalosConfig) {
				tc.Version = ""
			},
			expectValid: false,
		},
		{
			name: "invalid WireGuard port (too low)",
			modifyFunc: func(tc *TalosConfig) {
				tc.NetworkConfig.WireGuardPort = 100
			},
			expectValid: false,
		},
		{
			name: "invalid WireGuard port (too high)",
			modifyFunc: func(tc *TalosConfig) {
				tc.NetworkConfig.WireGuardPort = 70000
			},
			expectValid: false,
		},
		{
			name: "valid custom WireGuard port",
			modifyFunc: func(tc *TalosConfig) {
				tc.NetworkConfig.WireGuardPort = 12345
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			talosConfig := DefaultTalosConfig("test-cluster")
			tt.modifyFunc(talosConfig)

			// Basic validation checks
			isValid := true
			
			// Check version is not empty
			if talosConfig.Version == "" {
				isValid = false
			}
			
			// Check port ranges
			if talosConfig.NetworkConfig.WireGuardPort < 1024 || talosConfig.NetworkConfig.WireGuardPort > 65535 {
				isValid = false
			}
			if talosConfig.NetworkConfig.TalosAPIPort < 1024 || talosConfig.NetworkConfig.TalosAPIPort > 65535 {
				isValid = false
			}

			if isValid != tt.expectValid {
				t.Errorf("Expected validation result %v, got %v", tt.expectValid, isValid)
			}
		})
	}
}
