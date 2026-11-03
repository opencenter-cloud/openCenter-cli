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
	"fmt"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// FluxServiceTarget describes the Flux resources (GitRepository sources,
// ordered Kustomizations, and HelmRelease) representing a service in Kubernetes.
type FluxServiceTarget struct {
	ServiceName        string
	TargetNamespace    string
	GitRepositories    []string // e.g. ["flux-system"] or additional sources
	KustomizationOrder []string // Ordered Kustomization names in flux-system
	HelmReleaseName    string   // Empty if service does not manage a HelmRelease
}

// knownDescriptorFluxTargets maps descriptor-backed services whose manifests
// are rendered outside the core RenderCatalog specs.
var knownDescriptorFluxTargets = map[string]FluxServiceTarget{
	"cert-manager": {
		ServiceName:        "cert-manager",
		TargetNamespace:    "cert-manager",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"cert-manager-override", "cert-manager-base"},
		HelmReleaseName:    "cert-manager",
	},
	"harbor": {
		ServiceName:        "harbor",
		TargetNamespace:    "harbor",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"harbor-namespace", "harbor-override", "harbor-base"},
		HelmReleaseName:    "harbor",
	},
	"keycloak": {
		ServiceName:        "keycloak",
		TargetNamespace:    "keycloak",
		GitRepositories:    []string{"flux-system", "opencenter-keycloak-config"},
		KustomizationOrder: []string{"keycloak-postgres", "keycloak-operator", "keycloak-cr", "oidc-security-policies"},
		HelmReleaseName:    "",
	},
	"olm": {
		ServiceName:        "olm",
		TargetNamespace:    "olm",
		GitRepositories:    []string{"flux-system", "opencenter-olm-config"},
		KustomizationOrder: []string{"olm"},
		HelmReleaseName:    "",
	},
	"calico": {
		ServiceName:        "calico",
		TargetNamespace:    "kube-system",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"calico"},
		HelmReleaseName:    "calico",
	},
	"strimzi-kafka-operator": {
		ServiceName:        "strimzi-kafka-operator",
		TargetNamespace:    "strimzi-kafka-operator",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"strimzi-kafka-operator"},
		HelmReleaseName:    "strimzi-kafka-operator",
	},
	"kafka-cluster": {
		ServiceName:        "kafka-cluster",
		TargetNamespace:    "kafka",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"kafka-cluster"},
		HelmReleaseName:    "",
	},
	"alert-proxy": {
		ServiceName:        "alert-proxy",
		TargetNamespace:    "observability",
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{"alert-proxy"},
		HelmReleaseName:    "alert-proxy",
	},
}

// ResolveFluxServiceTarget resolves the Flux GitRepository, Kustomizations, and
// HelmRelease for a given service name using the RenderCatalog and descriptor registry.
func ResolveFluxServiceTarget(cfg *v2.Config, serviceName string) (FluxServiceTarget, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return FluxServiceTarget{}, fmt.Errorf("service name cannot be empty")
	}

	// 1. Check known descriptor targets
	if target, ok := knownDescriptorFluxTargets[serviceName]; ok {
		return target, nil
	}

	// 2. Check RenderCatalog specs
	catalog := newBuiltInRenderCatalog()
	spec, ok := catalog.Lookup(serviceName)
	if ok {
		target := FluxServiceTarget{
			ServiceName:     spec.ServiceName,
			TargetNamespace: spec.DefaultNamespace,
			GitRepositories: []string{"flux-system"},
		}

		kn := spec.KustomizationName
		if kn == "" {
			kn = spec.ServiceName
		}

		if spec.NamespaceStage {
			target.KustomizationOrder = append(target.KustomizationOrder, spec.ServiceName+"-namespace")
		}

		if spec.SingleStage {
			target.KustomizationOrder = append(target.KustomizationOrder, kn)
			return target, nil
		}

		if spec.BaseOnly {
			target.KustomizationOrder = append(target.KustomizationOrder, kn+"-base")
			for _, stage := range spec.PostBaseStages {
				target.KustomizationOrder = append(target.KustomizationOrder, stage.Name)
			}
			return target, nil
		}

		// Two-stage service: override first (to decrypt SOPS values Secret), then base
		if spec.HasOverrideValues {
			target.KustomizationOrder = append(target.KustomizationOrder, kn+"-override")
		}
		target.KustomizationOrder = append(target.KustomizationOrder, kn+"-base")
		for _, stage := range spec.PostBaseStages {
			target.KustomizationOrder = append(target.KustomizationOrder, stage.Name)
		}

		if spec.HelmReleaseName != "" {
			target.HelmReleaseName = spec.HelmReleaseName
		} else if spec.HasOverrideValues {
			target.HelmReleaseName = spec.ServiceName
		}

		return target, nil
	}

	// 3. Fallback for custom or unlisted services
	return FluxServiceTarget{
		ServiceName:        serviceName,
		TargetNamespace:    serviceName,
		GitRepositories:    []string{"flux-system"},
		KustomizationOrder: []string{serviceName + "-override", serviceName + "-base"},
		HelmReleaseName:    serviceName,
	}, nil
}
