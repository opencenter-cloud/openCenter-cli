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
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestShouldSkipFile_DisabledServiceSources(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		cfg      config.Config
		expected bool
	}{
		{
			name:    "skip source file for disabled service",
			relPath: "services/sources/opencenter-cert-manager.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{
						"cert-manager": {Enabled: false},
					},
				},
			},
			expected: true,
		},
		{
			name:    "include source file for enabled service",
			relPath: "services/sources/opencenter-cert-manager.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{
						"cert-manager": {Enabled: true},
					},
				},
			},
			expected: false,
		},
		{
			name:    "skip source file for disabled managed service",
			relPath: "managed-services/sources/opencenter-alert-proxy.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					ManagedService: map[string]config.ServiceCfg{
						"alert-proxy": {Enabled: false},
					},
				},
			},
			expected: true,
		},
		{
			name:    "include source file for enabled managed service",
			relPath: "managed-services/sources/opencenter-alert-proxy.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					ManagedService: map[string]config.ServiceCfg{
						"alert-proxy": {Enabled: true},
					},
				},
			},
			expected: false,
		},
		{
			name:    "skip service directory for disabled service",
			relPath: "services/cert-manager/kustomization.yaml",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{
						"cert-manager": {Enabled: false},
					},
				},
			},
			expected: true,
		},
		{
			name:    "include service directory for enabled service",
			relPath: "services/cert-manager/kustomization.yaml",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{
						"cert-manager": {Enabled: true},
					},
				},
			},
			expected: false,
		},
		{
			name:    "include source file for non-existent service (default behavior)",
			relPath: "services/sources/opencenter-unknown-service.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{},
				},
			},
			expected: false,
		},
		{
			name:    "include kustomization file in sources directory",
			relPath: "services/sources/kustomization.yaml.tpl",
			cfg: config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Services: map[string]config.ServiceCfg{
						"cert-manager": {Enabled: false},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipFile(tt.relPath, tt.cfg)
			if result != tt.expected {
				t.Errorf("shouldSkipFile(%q) = %v, want %v", tt.relPath, result, tt.expected)
			}
		})
	}
}
