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
	"bytes"
	"context"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestDestroyService_SupportsInfraDestroy(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		tofu     bool
		want     bool
	}{
		{
			name:     "openstack with tofu enabled",
			provider: "openstack",
			tofu:     true,
			want:     true,
		},
		{
			name:     "openstack with tofu disabled",
			provider: "openstack",
			tofu:     false,
			want:     false,
		},
		{
			name:     "vmware with tofu enabled",
			provider: "vmware",
			tofu:     true,
			want:     true,
		},
		{
			name:     "kind provider",
			provider: "kind",
			tofu:     false,
			want:     false,
		},
		{
			name:     "kind with tofu enabled (unusual)",
			provider: "kind",
			tofu:     true,
			want:     false,
		},
		{
			name:     "magnum with tofu disabled",
			provider: "magnum",
			tofu:     false,
			want:     true,
		},
		{
			name:     "magnum with tofu enabled",
			provider: "magnum",
			tofu:     true,
			want:     true,
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			tofu:     true,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &v2.Config{}
			cfg.OpenCenter.Infrastructure.Provider = tt.provider
			cfg.OpenTofu.Enabled = tt.tofu
			if tt.provider == "magnum" {
				cfg.OpenCenter.Cluster.ClusterName = "magnum-cluster"
				cfg.OpenCenter.Infrastructure.Cloud.Magnum = &v2.MagnumCloudConfig{
					AuthURL:                     "https://keystone.example.test/v3/",
					Region:                      "RegionOne",
					ProjectID:                   "project-id",
					ApplicationCredentialID:     "application-id",
					ApplicationCredentialSecret: "application-secret",
					ClusterTemplate:             "kubernetes-template",
				}
			}

			svc := NewDestroyService(&bytes.Buffer{})
			got := svc.SupportsInfraDestroy(cfg)

			if got != tt.want {
				t.Errorf("SupportsInfraDestroy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDestroyService_SupportsInfraDestroy_NilConfig(t *testing.T) {
	svc := NewDestroyService(&bytes.Buffer{})
	if svc.SupportsInfraDestroy(nil) {
		t.Error("SupportsInfraDestroy(nil) should return false")
	}
}

func TestDestroyService_MagnumInvalidConfigIsNotSilentlyUnsupported(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Infrastructure.Provider = "magnum"
	svc := NewDestroyService(&bytes.Buffer{})
	if !svc.SupportsInfraDestroy(cfg) {
		t.Fatal("Magnum provider selection should report destroy support before config validation")
	}
	if _, err := svc.DestroyInfrastructure(context.TODO(), cfg, nil); err == nil {
		t.Fatal("DestroyInfrastructure() should reject invalid Magnum configuration")
	}
}

func TestDestroyService_GetDestroyProvider(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		wantError bool
	}{
		{
			name:      "openstack provider",
			provider:  "openstack",
			wantError: false,
		},
		{
			name:      "vmware provider",
			provider:  "vmware",
			wantError: false,
		},
		{
			name:      "kind provider",
			provider:  "kind",
			wantError: true,
		},
		{
			name:      "magnum provider",
			provider:  "magnum",
			wantError: false,
		},
		{
			name:      "unknown provider",
			provider:  "unknown",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &v2.Config{}
			cfg.OpenCenter.Infrastructure.Provider = tt.provider
			if tt.provider == "magnum" {
				cfg.OpenCenter.Cluster.ClusterName = "magnum-cluster"
				cfg.OpenCenter.Infrastructure.Cloud.Magnum = &v2.MagnumCloudConfig{
					AuthURL:                     "https://keystone.example.test/v3/",
					Region:                      "RegionOne",
					ProjectID:                   "project-id",
					ApplicationCredentialID:     "application-id",
					ApplicationCredentialSecret: "application-secret",
					ClusterTemplate:             "kubernetes-template",
				}
			}

			svc := NewDestroyService(&bytes.Buffer{})
			provider, err := svc.getDestroyProvider(cfg)

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantError && tt.provider == "magnum" {
				steps, buildErr := provider.BuildSteps(cfg, nil)
				if buildErr != nil {
					t.Fatalf("BuildSteps() error: %v", buildErr)
				}
				if len(steps) != 1 || steps[0].ID != "magnum-delete" {
					t.Fatalf("Magnum destroy steps = %#v, want only magnum-delete", steps)
				}
			}
		})
	}
}
