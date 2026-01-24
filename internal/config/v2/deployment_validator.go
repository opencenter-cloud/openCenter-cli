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

package v2

import (
	"fmt"
)

// DeploymentMethod interface defines deployment-method-specific validation.
// Requirements: 5.6, 5.7, 5.8, 10.9, 10.10, 10.11, 10.12
type DeploymentMethod interface {
	ValidateConfig(cfg *Config) error
	ValidateCompatibility(provider string) error
	GetMethodName() string
	RequiresMasterNodes() bool
}

// KubesprayDeployment implements deployment validation for Kubespray.
// Requirements: 5.6
type KubesprayDeployment struct{}

// ValidateConfig validates Kubespray deployment configuration.
func (d *KubesprayDeployment) ValidateConfig(cfg *Config) error {
	if cfg.OpenCenter.Infrastructure.Compute.MasterCount == 0 {
		return fmt.Errorf("kubespray requires master_count > 0")
	}
	return nil
}

// ValidateCompatibility validates Kubespray compatibility with infrastructure provider.
func (d *KubesprayDeployment) ValidateCompatibility(provider string) error {
	// Kubespray supports all providers
	validProviders := []string{"openstack", "aws", "gcp", "azure", "baremetal", "vsphere"}
	for _, p := range validProviders {
		if provider == p {
			return nil
		}
	}
	return fmt.Errorf("kubespray does not support provider: %s", provider)
}

// GetMethodName returns the deployment method name.
func (d *KubesprayDeployment) GetMethodName() string {
	return "kubespray"
}

// RequiresMasterNodes returns whether this deployment method requires master nodes.
func (d *KubesprayDeployment) RequiresMasterNodes() bool {
	return true
}

// TalosDeployment implements deployment validation for Talos.
// Requirements: 5.7
type TalosDeployment struct{}

// ValidateConfig validates Talos deployment configuration.
func (d *TalosDeployment) ValidateConfig(cfg *Config) error {
	if cfg.OpenCenter.Infrastructure.Compute.MasterCount == 0 {
		return fmt.Errorf("talos requires master_count > 0")
	}
	return nil
}

// ValidateCompatibility validates Talos compatibility with infrastructure provider.
func (d *TalosDeployment) ValidateCompatibility(provider string) error {
	// Talos supports most providers except baremetal
	validProviders := []string{"openstack", "aws", "gcp", "azure", "vsphere"}
	for _, p := range validProviders {
		if provider == p {
			return nil
		}
	}
	return fmt.Errorf("talos does not support provider: %s", provider)
}

// GetMethodName returns the deployment method name.
func (d *TalosDeployment) GetMethodName() string {
	return "talos"
}

// RequiresMasterNodes returns whether this deployment method requires master nodes.
func (d *TalosDeployment) RequiresMasterNodes() bool {
	return true
}

// KamajiDeployment implements deployment validation for Kamaji.
// Requirements: 5.8, 10.9, 10.10, 10.11, 10.12
type KamajiDeployment struct{}

// ValidateConfig validates Kamaji deployment configuration.
func (d *KamajiDeployment) ValidateConfig(cfg *Config) error {
	// Kamaji requires master_count to be zero
	if cfg.OpenCenter.Infrastructure.Compute.MasterCount != 0 {
		return fmt.Errorf("kamaji requires master_count to be 0 (control plane is hosted)")
	}

	// Kamaji requires vrrp_enabled to be false
	if cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled {
		return fmt.Errorf("kamaji requires vrrp_enabled to be false (no HA VIP needed)")
	}

	// Kamaji requires kube_vip_enabled to be false
	if cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled {
		return fmt.Errorf("kamaji requires kube_vip_enabled to be false (control plane is hosted)")
	}

	// Validate Kamaji-specific configuration
	// Note: In v2, Kamaji config would be in deployment.kamaji, but for now we validate the constraints
	
	return nil
}

// ValidateCompatibility validates Kamaji compatibility with infrastructure provider.
func (d *KamajiDeployment) ValidateCompatibility(provider string) error {
	// Kamaji supports OpenStack, AWS, GCP, Azure, vSphere
	validProviders := []string{"openstack", "aws", "gcp", "azure", "vsphere"}
	for _, p := range validProviders {
		if provider == p {
			return nil
		}
	}
	return fmt.Errorf("kamaji does not support provider: %s", provider)
}

// GetMethodName returns the deployment method name.
func (d *KamajiDeployment) GetMethodName() string {
	return "kamaji"
}

// RequiresMasterNodes returns whether this deployment method requires master nodes.
func (d *KamajiDeployment) RequiresMasterNodes() bool {
	return false
}

// ValidateKamajiControlPlane validates Kamaji control plane configuration.
// Requirements: 10.10, 10.11, 10.12
func ValidateKamajiControlPlane(cp *KamajiControlPlane) error {
	// Validate replicas is odd
	if cp.Replicas%2 == 0 {
		return fmt.Errorf("control_plane.replicas must be odd (1, 3, 5, 7), got %d", cp.Replicas)
	}

	// Validate datastore configuration
	switch cp.Datastore {
	case "etcd":
		if cp.Etcd == nil {
			return fmt.Errorf("control_plane.etcd is required when datastore is etcd")
		}
		if cp.Etcd.StorageClass == "" {
			return fmt.Errorf("control_plane.etcd.storage_class is required")
		}
		if cp.Etcd.StorageSize == "" {
			return fmt.Errorf("control_plane.etcd.storage_size is required")
		}
	case "postgresql":
		if cp.PostgreSQL == nil {
			return fmt.Errorf("control_plane.postgresql is required when datastore is postgresql")
		}
		if cp.PostgreSQL.Host == "" {
			return fmt.Errorf("control_plane.postgresql.host is required")
		}
		if cp.PostgreSQL.Database == "" {
			return fmt.Errorf("control_plane.postgresql.database is required")
		}
	case "mysql":
		if cp.MySQL == nil {
			return fmt.Errorf("control_plane.mysql is required when datastore is mysql")
		}
		if cp.MySQL.Host == "" {
			return fmt.Errorf("control_plane.mysql.host is required")
		}
		if cp.MySQL.Database == "" {
			return fmt.Errorf("control_plane.mysql.database is required")
		}
	default:
		return fmt.Errorf("invalid datastore: %s (must be etcd, postgresql, or mysql)", cp.Datastore)
	}

	return nil
}

// ValidateKamajiWorkerPool validates Kamaji worker pool configuration.
// Requirements: 10.11, 10.12
func ValidateKamajiWorkerPool(pool *KamajiWorkerPool) error {
	// Validate bootstrap provider matches OS
	switch pool.OS {
	case "ubuntu", "windows":
		if pool.BootstrapProvider != "kubeadm" {
			return fmt.Errorf("worker pool %s: bootstrap_provider must be 'kubeadm' for OS '%s'", pool.Name, pool.OS)
		}
	case "talos":
		if pool.BootstrapProvider != "talos" {
			return fmt.Errorf("worker pool %s: bootstrap_provider must be 'talos' for OS 'talos'", pool.Name)
		}
		if pool.TalosVersion == "" {
			return fmt.Errorf("worker pool %s: talos_version is required when OS is 'talos'", pool.Name)
		}
	default:
		return fmt.Errorf("worker pool %s: invalid OS '%s' (must be ubuntu, windows, or talos)", pool.Name, pool.OS)
	}

	// Validate autoscaling constraints
	if pool.Autoscaling.Enabled {
		if pool.Autoscaling.MinReplicas < 1 {
			return fmt.Errorf("worker pool %s: autoscaling.min_replicas must be >= 1", pool.Name)
		}
		if pool.Autoscaling.MaxReplicas < pool.Autoscaling.MinReplicas {
			return fmt.Errorf("worker pool %s: autoscaling.max_replicas must be >= min_replicas", pool.Name)
		}
		if pool.Count < pool.Autoscaling.MinReplicas || pool.Count > pool.Autoscaling.MaxReplicas {
			return fmt.Errorf("worker pool %s: count (%d) must be between min_replicas (%d) and max_replicas (%d)",
				pool.Name, pool.Count, pool.Autoscaling.MinReplicas, pool.Autoscaling.MaxReplicas)
		}
	}

	return nil
}

// ValidateClusterAPIProviders validates Cluster API provider configuration.
// Requirements: 10.9
func ValidateClusterAPIProviders(providers *ClusterAPIProviders, infraProvider string) error {
	// Validate infrastructure provider matches
	if providers.Infrastructure != infraProvider {
		return fmt.Errorf("cluster_api.providers.infrastructure (%s) must match infrastructure.provider (%s)",
			providers.Infrastructure, infraProvider)
	}

	return nil
}

// GetDeploymentMethod returns the appropriate deployment method validator.
func GetDeploymentMethod(methodName string) (DeploymentMethod, error) {
	switch methodName {
	case "kubespray":
		return &KubesprayDeployment{}, nil
	case "talos":
		return &TalosDeployment{}, nil
	case "kamaji":
		return &KamajiDeployment{}, nil
	default:
		return nil, fmt.Errorf("unsupported deployment method: %s", methodName)
	}
}
