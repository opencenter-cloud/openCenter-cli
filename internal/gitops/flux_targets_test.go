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
	"reflect"
	"testing"
)

func TestResolveFluxServiceTarget(t *testing.T) {
	tests := []struct {
		name            string
		service         string
		wantNamespace   string
		wantSources     []string
		wantKustoms     []string
		wantHelmRelease string
	}{
		{
			name:            "openstack-ccm resolves namespace stage, override, base and cloud-controller-manager HR",
			service:         "openstack-ccm",
			wantNamespace:   "openstack-ccm",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"openstack-ccm-namespace", "openstack-ccm-override", "openstack-ccm-base"},
			wantHelmRelease: "openstack-cloud-controller-manager",
		},
		{
			name:            "openstack-csi resolves namespace stage, override, base and cinder-csi HR",
			service:         "openstack-csi",
			wantNamespace:   "openstack-csi",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"openstack-csi-namespace", "openstack-csi-override", "openstack-csi-base"},
			wantHelmRelease: "openstack-cinder-csi",
		},
		{
			name:            "headlamp resolves override, base and headlamp HR",
			service:         "headlamp",
			wantNamespace:   "headlamp",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"headlamp-override", "headlamp-base"},
			wantHelmRelease: "headlamp",
		},
		{
			name:            "gateway-api resolves envoy-gateway-api kustomizations and HR",
			service:         "gateway-api",
			wantNamespace:   "envoy-gateway-system",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"envoy-gateway-api-override", "envoy-gateway-api-base"},
			wantHelmRelease: "envoy-gateway-api",
		},
		{
			name:            "gateway resolves single-stage with no HelmRelease",
			service:         "gateway",
			wantNamespace:   "gateway",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"gateway"},
			wantHelmRelease: "",
		},
		{
			name:            "kube-prometheus-stack resolves observability namespace and override/base",
			service:         "kube-prometheus-stack",
			wantNamespace:   "observability",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"kube-prometheus-stack-override", "kube-prometheus-stack-base"},
			wantHelmRelease: "kube-prometheus-stack",
		},
		{
			name:            "kyverno resolves base-only and post-base stages",
			service:         "kyverno",
			wantNamespace:   "kyverno",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"kyverno-base", "kyverno-default-ruleset"},
			wantHelmRelease: "",
		},
		{
			name:            "cert-manager resolves descriptor targets",
			service:         "cert-manager",
			wantNamespace:   "cert-manager",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"cert-manager-override", "cert-manager-base"},
			wantHelmRelease: "cert-manager",
		},
		{
			name:            "keycloak resolves descriptor targets including extra GitRepository",
			service:         "keycloak",
			wantNamespace:   "keycloak",
			wantSources:     []string{"flux-system", "opencenter-keycloak-config"},
			wantKustoms:     []string{"keycloak-postgres", "keycloak-operator", "keycloak-cr", "oidc-security-policies"},
			wantHelmRelease: "",
		},
		{
			name:            "olm resolves descriptor target",
			service:         "olm",
			wantNamespace:   "olm",
			wantSources:     []string{"flux-system", "opencenter-olm-config"},
			wantKustoms:     []string{"olm"},
			wantHelmRelease: "",
		},
		{
			name:            "harbor resolves namespace, override, base and harbor HR",
			service:         "harbor",
			wantNamespace:   "harbor",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"harbor-namespace", "harbor-override", "harbor-base"},
			wantHelmRelease: "harbor",
		},
		{
			name:            "unknown custom service falls back gracefully",
			service:         "my-custom-app",
			wantNamespace:   "my-custom-app",
			wantSources:     []string{"flux-system"},
			wantKustoms:     []string{"my-custom-app-override", "my-custom-app-base"},
			wantHelmRelease: "my-custom-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ResolveFluxServiceTarget(nil, tt.service)
			if err != nil {
				t.Fatalf("ResolveFluxServiceTarget(%q) error = %v", tt.service, err)
			}
			if target.TargetNamespace != tt.wantNamespace {
				t.Errorf("TargetNamespace = %q, want %q", target.TargetNamespace, tt.wantNamespace)
			}
			if !reflect.DeepEqual(target.GitRepositories, tt.wantSources) {
				t.Errorf("GitRepositories = %v, want %v", target.GitRepositories, tt.wantSources)
			}
			if !reflect.DeepEqual(target.KustomizationOrder, tt.wantKustoms) {
				t.Errorf("KustomizationOrder = %v, want %v", target.KustomizationOrder, tt.wantKustoms)
			}
			if target.HelmReleaseName != tt.wantHelmRelease {
				t.Errorf("HelmReleaseName = %q, want %q", target.HelmReleaseName, tt.wantHelmRelease)
			}
		})
	}
}

func TestResolveFluxServiceTargetEmptyService(t *testing.T) {
	_, err := ResolveFluxServiceTarget(nil, "   ")
	if err == nil {
		t.Fatal("expected error for empty service name, got nil")
	}
}
