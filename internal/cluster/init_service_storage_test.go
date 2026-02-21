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

package cluster

import (
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// TestStoragePluginAutoConfiguration tests that storage plugins are automatically
// configured based on the provider type during cluster initialization.
func TestStoragePluginAutoConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		expectCinder  bool
		expectVsphere bool
	}{
		{
			name:          "OpenStack provider enables Cinder",
			provider:      "openstack",
			expectCinder:  true,
			expectVsphere: false,
		},
		{
			name:          "VMware provider enables vSphere",
			provider:      "vmware",
			expectCinder:  false,
			expectVsphere: true,
		},
		{
			name:          "Kind provider disables both",
			provider:      "kind",
			expectCinder:  false,
			expectVsphere: false,
		},
		{
			name:          "Baremetal provider disables both",
			provider:      "baremetal",
			expectCinder:  false,
			expectVsphere: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filesystem
			errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
			fileSystem := fs.NewDefaultFileSystem(errorHandler)

			// Create path resolver
			pathResolver := paths.NewPathResolver("/test/clusters")

			// Create validation engine
			validationEngine := validation.NewValidationEngine()

			// Create init service
			initService := &InitService{
				pathResolver:     pathResolver,
				validationEngine: validationEngine,
				fileSystem:       fileSystem,
			}

			// Create init options
			opts := InitOptions{
				ClusterName:  "test-cluster",
				Organization: "test-org",
				Provider:     tt.provider,
				NoKeyGen:     true,
				NoGitInit:    true,
			}

			// Create default config
			cfg, _, err := initService.createDefaultConfig(opts)
			if err != nil {
				t.Fatalf("createDefaultConfig failed: %v", err)
			}

			// Verify provider is set correctly
			if cfg.OpenCenter.Infrastructure.Provider != tt.provider {
				t.Errorf("expected provider %s, got %s", tt.provider, cfg.OpenCenter.Infrastructure.Provider)
			}

			// Verify Cinder storage plugin
			if cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Cinder.Enabled != tt.expectCinder {
				t.Errorf("expected Cinder.Enabled=%v, got %v", tt.expectCinder, cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Cinder.Enabled)
			}

			// Verify vSphere storage plugin
			if cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Vsphere.Enabled != tt.expectVsphere {
				t.Errorf("expected Vsphere.Enabled=%v, got %v", tt.expectVsphere, cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Vsphere.Enabled)
			}
		})
	}
}

// TestStoragePluginMutualExclusion tests that Cinder and vSphere are mutually exclusive
func TestStoragePluginMutualExclusion(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{"OpenStack", "openstack"},
		{"VMware", "vmware"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filesystem
			errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
			fileSystem := fs.NewDefaultFileSystem(errorHandler)

			// Create path resolver
			pathResolver := paths.NewPathResolver("/test/clusters")

			// Create validation engine
			validationEngine := validation.NewValidationEngine()

			// Create init service
			initService := &InitService{
				pathResolver:     pathResolver,
				validationEngine: validationEngine,
				fileSystem:       fileSystem,
			}

			// Create init options
			opts := InitOptions{
				ClusterName:  "test-cluster",
				Organization: "test-org",
				Provider:     tt.provider,
				NoKeyGen:     true,
				NoGitInit:    true,
			}

			// Create default config
			cfg, _, err := initService.createDefaultConfig(opts)
			if err != nil {
				t.Fatalf("createDefaultConfig failed: %v", err)
			}

			// Verify that Cinder and vSphere are mutually exclusive
			cinderEnabled := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Cinder.Enabled
			vsphereEnabled := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Vsphere.Enabled

			if cinderEnabled && vsphereEnabled {
				t.Errorf("both Cinder and vSphere are enabled - they should be mutually exclusive")
			}

			// Verify at least one is enabled for these providers
			if !cinderEnabled && !vsphereEnabled {
				t.Errorf("neither Cinder nor vSphere is enabled for provider %s", tt.provider)
			}
		})
	}
}
