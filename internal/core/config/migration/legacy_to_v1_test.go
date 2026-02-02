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

func TestLegacyToV1Migration(t *testing.T) {
	migration := LegacyToV1Migration()

	if migration == nil {
		t.Fatal("LegacyToV1Migration returned nil")
	}

	if migration.From != "legacy" {
		t.Errorf("From = %s, want legacy", migration.From)
	}

	if migration.To != "1.0" {
		t.Errorf("To = %s, want 1.0", migration.To)
	}

	if migration.Migrate == nil {
		t.Error("Migrate function is nil")
	}

	if migration.Description == "" {
		t.Error("Description is empty")
	}
}

func TestMigrateLegacyToV1(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "valid legacy config",
			cfg: &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name:   "test-cluster",
						Region: "us-east-1",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
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
			result, err := migrateLegacyToV1(tt.cfg)

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

			// Verify schema version was set
			if result.SchemaVersion != "1.0" {
				t.Errorf("SchemaVersion = %s, want 1.0", result.SchemaVersion)
			}

			// Verify metadata was created
			if result.Metadata.CreatedAt.IsZero() {
				t.Error("CreatedAt should be set")
			}

			if result.Metadata.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should be set")
			}

			// Verify cluster name was preserved
			if tt.cfg != nil && tt.cfg.OpenCenter.Meta.Name != "" {
				if result.OpenCenter.Meta.Name != tt.cfg.OpenCenter.Meta.Name {
					t.Errorf("cluster name = %s, want %s",
						result.OpenCenter.Meta.Name, tt.cfg.OpenCenter.Meta.Name)
				}
			}
		})
	}
}

func TestMigrateLegacyToV1_PreservesData(t *testing.T) {
	source := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Env:          "production",
				Region:       "us-east-1",
				Organization: "test-org",
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
		Metadata: config.ConfigMetadata{
			Tags: map[string]string{
				"environment": "production",
				"team":        "platform",
			},
		},
	}

	result, err := migrateLegacyToV1(source)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify all metadata was preserved
	if result.OpenCenter.Meta.Name != source.OpenCenter.Meta.Name {
		t.Error("cluster name not preserved")
	}
	if result.OpenCenter.Meta.Env != source.OpenCenter.Meta.Env {
		t.Error("environment not preserved")
	}
	if result.OpenCenter.Meta.Region != source.OpenCenter.Meta.Region {
		t.Error("region not preserved")
	}
	if result.OpenCenter.Meta.Organization != source.OpenCenter.Meta.Organization {
		t.Error("organization not preserved")
	}

	// Verify tags were preserved
	if len(result.Metadata.Tags) != len(source.Metadata.Tags) {
		t.Errorf("tags count = %d, want %d", len(result.Metadata.Tags), len(source.Metadata.Tags))
	}

	// Verify infrastructure was preserved
	if result.OpenCenter.Infrastructure.Provider != source.OpenCenter.Infrastructure.Provider {
		t.Error("provider not preserved")
	}
	if result.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != source.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL {
		t.Error("OpenStack auth_url not preserved")
	}
}
