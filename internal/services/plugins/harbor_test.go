/*
Copyright 2025 Victor Palma <victor.palma@rackspace.com>
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

package plugins

import (
	"context"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

func TestNewHarborPlugin(t *testing.T) {
	plugin := NewHarborPlugin()

	if plugin == nil {
		t.Fatal("NewHarborPlugin() returned nil")
	}

	if plugin.Name() != "harbor" {
		t.Errorf("Name() = %v, want harbor", plugin.Name())
	}

	if plugin.Type() != svc.ServiceTypeStorage {
		t.Errorf("Type() = %v, want %v", plugin.Type(), svc.ServiceTypeStorage)
	}
}

func TestHarborPlugin_Validate(t *testing.T) {
	plugin := NewHarborPlugin()

	tests := []struct {
		name    string
		config  *services.HarborConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with https external URL",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				ExternalURL:        "https://harbor.example.com",
				StorageType:        "filesystem",
				DatabaseType:       "internal",
				RegistryVolumeSize: 100,
			},
			wantErr: false,
		},
		{
			name: "valid config with http external URL",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				ExternalURL:  "http://harbor.local",
				StorageType:  "filesystem",
				DatabaseType: "internal",
			},
			wantErr: false,
		},
		{
			name: "invalid external URL format",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				ExternalURL: "not-a-url",
			},
			wantErr: true,
			errMsg:  "harbor external_url must be a valid HTTP(S) URL",
		},
		{
			name: "s3 storage requires bucket",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				StorageType: "s3",
				S3Region:    "us-east-1",
			},
			wantErr: true,
			errMsg:  "s3_bucket is required when storage_type is s3",
		},
		{
			name: "s3 storage requires region",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				StorageType: "s3",
				S3Bucket:    "my-bucket",
			},
			wantErr: true,
			errMsg:  "s3_region is required when storage_type is s3",
		},
		{
			name: "valid s3 storage config",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				StorageType: "s3",
				S3Bucket:    "my-bucket",
				S3Region:    "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "external database requires host",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				DatabaseType: "external",
				DatabasePort: 5432,
				DatabaseName: "harbor",
				DatabaseUser: "harbor",
			},
			wantErr: true,
			errMsg:  "database_host is required when database_type is external",
		},
		{
			name: "external database requires port",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				DatabaseType: "external",
				DatabaseHost: "postgres.example.com",
				DatabaseName: "harbor",
				DatabaseUser: "harbor",
			},
			wantErr: true,
			errMsg:  "database_port must be positive when database_type is external",
		},
		{
			name: "external database requires name",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				DatabaseType: "external",
				DatabaseHost: "postgres.example.com",
				DatabasePort: 5432,
				DatabaseUser: "harbor",
			},
			wantErr: true,
			errMsg:  "database_name is required when database_type is external",
		},
		{
			name: "external database requires user",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				DatabaseType: "external",
				DatabaseHost: "postgres.example.com",
				DatabasePort: 5432,
				DatabaseName: "harbor",
			},
			wantErr: true,
			errMsg:  "database_user is required when database_type is external",
		},
		{
			name: "valid external database config",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				DatabaseType: "external",
				DatabaseHost: "postgres.example.com",
				DatabasePort: 5432,
				DatabaseName: "harbor",
				DatabaseUser: "harbor",
			},
			wantErr: false,
		},
		{
			name: "negative registry volume size",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				RegistryVolumeSize: -10,
			},
			wantErr: true,
			errMsg:  "registry_volume_size must be non-negative",
		},
		{
			name: "disabled service skips validation",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: false,
				},
				ExternalURL:        "invalid",
				RegistryVolumeSize: -10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.Validate(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestHarborPlugin_Validate_InvalidType(t *testing.T) {
	plugin := NewHarborPlugin()

	err := plugin.Validate("invalid-type")
	if err == nil {
		t.Error("Validate() should return error for invalid config type")
	}

	// Error is wrapped, so check if it contains the expected message
	expectedMsg := "invalid config type for harbor: expected *HarborConfig"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Validate() error = %v, want error containing %v", err.Error(), expectedMsg)
	}
}

func TestHarborPlugin_Render(t *testing.T) {
	plugin := NewHarborPlugin()

	config := &services.HarborConfig{
		BaseConfig: services.BaseConfig{
			Enabled: true,
		},
	}

	// Render should not return error (template system handles rendering)
	err := plugin.Render(context.Background(), config, nil)
	if err != nil {
		t.Errorf("Render() unexpected error = %v", err)
	}
}

func TestHarborPlugin_Status(t *testing.T) {
	plugin := NewHarborPlugin()

	tests := []struct {
		name         string
		config       *services.HarborConfig
		wantState    string
		wantMsg      string
		checkDetails bool
	}{
		{
			name: "enabled service with details",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				ExternalURL:        "https://harbor.example.com",
				StorageType:        "s3",
				DatabaseType:       "external",
				RegistryVolumeSize: 100,
			},
			wantState:    "pending",
			wantMsg:      "Harbor container registry",
			checkDetails: true,
		},
		{
			name: "disabled service",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: false,
				},
			},
			wantState: "disabled",
			wantMsg:   "Service is disabled",
		},
		{
			name: "enabled service without status defaults to pending",
			config: &services.HarborConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
			},
			wantState: "pending",
			wantMsg:   "Harbor container registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := plugin.Status(tt.config)

			if status.State != tt.wantState {
				t.Errorf("Status() state = %v, want %v", status.State, tt.wantState)
			}

			if status.Message != tt.wantMsg {
				t.Errorf("Status() message = %v, want %v", status.Message, tt.wantMsg)
			}

			if tt.checkDetails {
				if status.Details == nil {
					t.Error("Status() details should not be nil")
				} else {
					if status.Details["external_url"] != tt.config.ExternalURL {
						t.Errorf("Status() details[external_url] = %v, want %v",
							status.Details["external_url"], tt.config.ExternalURL)
					}
					if status.Details["storage_type"] != tt.config.StorageType {
						t.Errorf("Status() details[storage_type] = %v, want %v",
							status.Details["storage_type"], tt.config.StorageType)
					}
					if status.Details["database_type"] != tt.config.DatabaseType {
						t.Errorf("Status() details[database_type] = %v, want %v",
							status.Details["database_type"], tt.config.DatabaseType)
					}
					if status.Details["volume_size_gb"] != tt.config.RegistryVolumeSize {
						t.Errorf("Status() details[volume_size_gb] = %v, want %v",
							status.Details["volume_size_gb"], tt.config.RegistryVolumeSize)
					}
				}
			}
		})
	}
}

func TestHarborPlugin_Status_InvalidType(t *testing.T) {
	plugin := NewHarborPlugin()

	status := plugin.Status("invalid-type")

	if status.State != "failed" {
		t.Errorf("Status() state = %v, want failed", status.State)
	}

	if status.Message != "Invalid configuration type" {
		t.Errorf("Status() message = %v, want 'Invalid configuration type'", status.Message)
	}
}
