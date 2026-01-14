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

// TypedConfigPath represents a type-safe path to a configuration field.
// It provides compile-time validation of configuration paths through Go's type system.
type TypedConfigPath[T any] struct {
	path string
}

// Path returns the string representation of the configuration path.
func (p TypedConfigPath[T]) Path() string {
	return p.path
}

// TypedConfigPaths provides type-safe accessors for all configuration paths.
// This enables compile-time validation of configuration paths and prevents
// runtime errors from invalid path strings.
var TypedConfigPaths = struct {
	// Meta paths
	Organization TypedConfigPath[string]
	ClusterName  TypedConfigPath[string]
	Environment  TypedConfigPath[string]
	Region       TypedConfigPath[string]
	
	// Infrastructure paths
	Provider TypedConfigPath[string]
	SSHUser  TypedConfigPath[string]
	
	// Kubernetes paths
	KubernetesVersion    TypedConfigPath[string]
	MasterCount          TypedConfigPath[int]
	WorkerCount          TypedConfigPath[int]
	WindowsWorkerCount   TypedConfigPath[int]
	SubnetPods           TypedConfigPath[string]
	SubnetServices       TypedConfigPath[string]
	
	// Networking paths
	SubnetNodes      TypedConfigPath[string]
	DNSNameservers   TypedConfigPath[[]string]
	NTPServers       TypedConfigPath[[]string]
	
	// Cluster paths
	BaseDomain       TypedConfigPath[string]
	AdminEmail       TypedConfigPath[string]
	SSHAuthorizedKeys TypedConfigPath[[]string]
	
	// Security paths
	K8sHardening TypedConfigPath[bool]
	OSHardening  TypedConfigPath[bool]
	
	// Storage paths
	DefaultStorageClass TypedConfigPath[string]
	
	// GitOps paths
	GitURL    TypedConfigPath[string]
	GitBranch TypedConfigPath[string]
	
	// Secrets paths
	SecretsBackend TypedConfigPath[string]
	
	// OpenStack paths
	OpenStackAuthURL    TypedConfigPath[string]
	OpenStackRegion     TypedConfigPath[string]
	OpenStackTenantName TypedConfigPath[string]
	
	// AWS paths
	AWSRegion TypedConfigPath[string]
}{
	// Meta paths
	Organization: TypedConfigPath[string]{path: "opencenter.meta.organization"},
	ClusterName:  TypedConfigPath[string]{path: "opencenter.meta.name"},
	Environment:  TypedConfigPath[string]{path: "opencenter.meta.env"},
	Region:       TypedConfigPath[string]{path: "opencenter.meta.region"},
	
	// Infrastructure paths
	Provider: TypedConfigPath[string]{path: "opencenter.infrastructure.provider"},
	SSHUser:  TypedConfigPath[string]{path: "opencenter.infrastructure.ssh_user"},
	
	// Kubernetes paths
	KubernetesVersion:  TypedConfigPath[string]{path: "opencenter.cluster.kubernetes.version"},
	MasterCount:        TypedConfigPath[int]{path: "opencenter.cluster.kubernetes.master_count"},
	WorkerCount:        TypedConfigPath[int]{path: "opencenter.cluster.kubernetes.worker_count"},
	WindowsWorkerCount: TypedConfigPath[int]{path: "opencenter.cluster.kubernetes.worker_count_windows"},
	SubnetPods:         TypedConfigPath[string]{path: "opencenter.cluster.kubernetes.subnet_pods"},
	SubnetServices:     TypedConfigPath[string]{path: "opencenter.cluster.kubernetes.subnet_services"},
	
	// Networking paths
	SubnetNodes:    TypedConfigPath[string]{path: "networking.subnet_nodes"},
	DNSNameservers: TypedConfigPath[[]string]{path: "networking.dns_nameservers"},
	NTPServers:     TypedConfigPath[[]string]{path: "networking.ntp_servers"},
	
	// Cluster paths
	BaseDomain:        TypedConfigPath[string]{path: "opencenter.cluster.base_domain"},
	AdminEmail:        TypedConfigPath[string]{path: "opencenter.cluster.admin_email"},
	SSHAuthorizedKeys: TypedConfigPath[[]string]{path: "opencenter.cluster.ssh_authorized_keys"},
	
	// Security paths
	K8sHardening: TypedConfigPath[bool]{path: "security.k8s_hardening"},
	OSHardening:  TypedConfigPath[bool]{path: "security.os_hardening"},
	
	// Storage paths
	DefaultStorageClass: TypedConfigPath[string]{path: "opencenter.storage.default_storage_class"},
	
	// GitOps paths
	GitURL:    TypedConfigPath[string]{path: "opencenter.gitops.git_url"},
	GitBranch: TypedConfigPath[string]{path: "opencenter.gitops.git_branch"},
	
	// Secrets paths
	SecretsBackend: TypedConfigPath[string]{path: "opencenter.secrets.backend"},
	
	// OpenStack paths
	OpenStackAuthURL:    TypedConfigPath[string]{path: "opencenter.infrastructure.cloud.openstack.auth_url"},
	OpenStackRegion:     TypedConfigPath[string]{path: "opencenter.infrastructure.cloud.openstack.region"},
	OpenStackTenantName: TypedConfigPath[string]{path: "opencenter.infrastructure.cloud.openstack.tenant_name"},
	
	// AWS paths
	AWSRegion: TypedConfigPath[string]{path: "opencenter.infrastructure.cloud.aws.region"},
}
