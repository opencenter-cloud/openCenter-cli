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

package migration

import (
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

func TestV1ToV2Migration(t *testing.T) {
	migration := V1ToV2Migration()

	if migration == nil {
		t.Fatal("V1ToV2Migration returned nil")
	}

	if migration.From != "1.0" {
		t.Errorf("From = %s, want 1.0", migration.From)
	}

	if migration.To != "2.0" {
		t.Errorf("To = %s, want 2.0", migration.To)
	}

	if migration.Migrate == nil {
		t.Error("Migrate function is nil")
	}

	if migration.Description == "" {
		t.Error("Description is empty")
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "valid v1 config",
			cfg: &config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name:   "test-cluster",
						Region: "us-east-1",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
						Cloud: config.CloudConfig{
							OpenStack: config.SimplifiedOpenStackCloud{
								AuthURL:    "https://identity.example.com/v3",
								TenantName: "test-project",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrateV1ToV2(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify schema version was updated
			if result.SchemaVersion != "2.0" {
				t.Errorf("SchemaVersion = %s, want 2.0", result.SchemaVersion)
			}
		})
	}
}

func TestMigrateV1ToV2_ProviderValidation(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		cfg      config.Config
		wantErr  bool
	}{
		{
			name:     "valid openstack",
			provider: "openstack",
			cfg: config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
						Cloud: config.CloudConfig{
							OpenStack: config.SimplifiedOpenStackCloud{
								AuthURL:    "https://identity.example.com/v3",
								TenantName: "test-project",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid openstack - missing auth_url",
			provider: "openstack",
			cfg: config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
						Cloud: config.CloudConfig{
							OpenStack: config.SimplifiedOpenStackCloud{
								TenantName: "test-project",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "valid aws",
			provider: "aws",
			cfg: config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "aws",
						Cloud: config.CloudConfig{
							AWS: config.SimplifiedAWSCloud{
								Region: "us-east-1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid aws - missing region",
			provider: "aws",
			cfg: config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Infrastructure: config.Infrastructure{
						Provider: "aws",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := migrateV1ToV2(&tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
