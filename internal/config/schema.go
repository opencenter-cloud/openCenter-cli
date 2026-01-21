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

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// SchemaVersion represents the current schema version for backward compatibility tracking
const SchemaVersion = "1.0.0"

// GenerateSchema returns a JSON schema (Draft 2020-12) describing the current
// cluster configuration structure. The schema mirrors the structure emitted by
// defaultConfig / cluster init so IDE integrations stay in sync with runtime.
// It includes comprehensive validation rules, constraints, and versioning support.
func GenerateSchema(pretty bool) ([]byte, error) {
	// Base service schema for services that only need enabled flag
	baseServiceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "Enable or disable this service",
				"default":     false,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Service deployment status",
				"enum":        []string{"pending", "running", "success", "failed"},
			},
			"release": map[string]any{
				"type":        "string",
				"description": "Release version or tag for this service (mutually exclusive with branch)",
				"pattern":     "^[a-zA-Z0-9._-]+$",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch for this service (mutually exclusive with release)",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Git repository URI for this service",
				"pattern":     "^(https?://|git@|ssh://)",
			},
		},
		"additionalProperties": false,
	}

	// Cert-manager specific schema
	certManagerSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"aws_access_key": map[string]any{
				"type":        "string",
				"description": "AWS access key for cert-manager DNS validation",
				"minLength":   16,
			},
			"aws_secret_access_key": map[string]any{
				"type":        "string",
				"description": "AWS secret access key for cert-manager DNS validation",
				"minLength":   32,
			},
			"email": map[string]any{
				"type":        "string",
				"description": "Email address for Let's Encrypt certificate notifications",
				"format":      "email",
			},
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "Enable cert-manager for automatic TLS certificate management",
				"default":     false,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Service deployment status",
				"enum":        []string{"pending", "running", "success", "failed"},
			},
			"region": map[string]any{
				"type":        "string",
				"description": "AWS region for Route53 DNS validation",
				"pattern":     "^[a-z]{2}-[a-z]+-[0-9]{1}$",
			},
			"release": map[string]any{
				"type":        "string",
				"description": "Cert-manager release version (mutually exclusive with branch)",
				"pattern":     "^[a-zA-Z0-9._-]+$",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch for cert-manager (mutually exclusive with release)",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Git repository URI for cert-manager",
				"pattern":     "^(https?://|git@|ssh://)",
			},
		},
		"additionalProperties": false,
	}

	// Calico specific schema (for services section, not network plugin)
	calicoServiceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "Enable Calico CNI service",
				"default":     false,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Service deployment status",
				"enum":        []string{"pending", "running", "success", "failed"},
			},
			"release": map[string]any{
				"type":        "string",
				"description": "Calico release version (mutually exclusive with branch)",
				"pattern":     "^[a-zA-Z0-9._-]+$",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch for Calico (mutually exclusive with release)",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Git repository URI for Calico",
				"pattern":     "^(https?://|git@|ssh://)",
			},
		},
		"additionalProperties": false,
	}

	// Etcd-backup specific schema
	etcdBackupSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"aws_access_key": map[string]any{
				"type":        "string",
				"description": "AWS access key for S3 backup storage",
				"minLength":   16,
			},
			"aws_secret_access_key": map[string]any{
				"type":        "string",
				"description": "AWS secret access key for S3 backup storage",
				"minLength":   32,
			},
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "Enable automated etcd backups to S3",
				"default":     false,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Service deployment status",
				"enum":        []string{"pending", "running", "success", "failed"},
			},
			"release": map[string]any{
				"type":        "string",
				"description": "Etcd-backup release version (mutually exclusive with branch)",
				"pattern":     "^[a-zA-Z0-9._-]+$",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch for etcd-backup (mutually exclusive with release)",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Git repository URI for etcd-backup",
				"pattern":     "^(https?://|git@|ssh://)",
			},
			"s3_host": map[string]any{
				"type":        "string",
				"description": "S3-compatible storage endpoint URL",
				"format":      "uri",
			},
			"s3_region": map[string]any{
				"type":        "string",
				"description": "S3 region for backup storage",
				"minLength":   1,
			},
		},
		"additionalProperties": false,
	}

	infrastructure := map[string]any{
		"type":        "object",
		"description": "Infrastructure provider configuration",
		"required":    []string{"provider"},
		"properties": map[string]any{
			"provider": map[string]any{
				"type":        "string",
				"description": "Cloud provider type",
				"enum":        []string{"openstack", "aws", "vmware", "kind", "baremetal"},
				"default":     "openstack",
			},
			"bastion": map[string]any{
				"type":        "object",
				"description": "Bastion host configuration for baremetal deployments",
				"properties": map[string]any{
					"address": map[string]any{
						"type":        "string",
						"description": "Bastion host address (defaults to localhost for baremetal)",
						"default":     "localhost",
					},
				},
			},
			"k8s_api_ip": map[string]any{
				"type":        "string",
				"description": "Kubernetes API server IP address",
				"format":      "ipv4",
			},
			"cloud": map[string]any{
				"type":        "object",
				"description": "Cloud provider specific configuration",
				"properties": map[string]any{
					"aws": map[string]any{
						"type":        "object",
						"description": "AWS cloud provider configuration",
						"properties": map[string]any{
							"profile": map[string]any{
								"type":        "string",
								"description": "AWS CLI profile name",
								"minLength":   1,
							},
							"region": map[string]any{
								"type":        "string",
								"description": "AWS region",
								"pattern":     "^[a-z]{2}-[a-z]+-[0-9]{1}$",
								"examples":    []string{"us-east-1", "us-west-2", "eu-west-1"},
							},
							"vpc_id": map[string]any{
								"type":        "string",
								"description": "VPC ID for cluster deployment",
								"pattern":     "^vpc-[a-f0-9]{8,17}$",
							},
							"private_subnets": map[string]any{
								"type":        "array",
								"description": "List of private subnet IDs",
								"items": map[string]any{
									"type":    "string",
									"pattern": "^subnet-[a-f0-9]{8,17}$",
								},
								"minItems": 1,
							},
							"public_subnets": map[string]any{
								"type":        "array",
								"description": "List of public subnet IDs",
								"items": map[string]any{
									"type":    "string",
									"pattern": "^subnet-[a-f0-9]{8,17}$",
								},
								"minItems": 1,
							},
						},
					},
					"openstack": map[string]any{
						"type":        "object",
						"description": "OpenStack cloud provider configuration",
						"properties": map[string]any{
							"auth_url": map[string]any{
								"type":        "string",
								"description": "OpenStack Keystone authentication URL",
								"format":      "uri",
								"pattern":     "^https?://",
							},
							"insecure": map[string]any{
								"type":        "boolean",
								"description": "Skip TLS certificate verification (not recommended for production)",
								"default":     false,
							},
							"region": map[string]any{
								"type":        "string",
								"description": "OpenStack region name",
								"minLength":   1,
							},
							"application_credential_id": map[string]any{
								"type":        "string",
								"description": "OpenStack application credential ID",
								"minLength":   32,
							},
							"application_credential_secret": map[string]any{
								"type":        "string",
								"description": "OpenStack application credential secret",
								"minLength":   32,
							},
							"domain": map[string]any{
								"type":        "string",
								"description": "OpenStack domain name",
								"default":     "Default",
							},
							"tenant_name": map[string]any{
								"type":        "string",
								"description": "OpenStack project/tenant name or ID",
								"minLength":   1,
							},
							"floating_network_id": map[string]any{
								"type":        "string",
								"description": "External network ID for floating IPs",
								"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
							},
							"subnet_id": map[string]any{
								"type":        "string",
								"description": "Subnet ID for cluster network",
								"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
							},
						},
					},
				},
			},
		},
	}

	networkPlugin := map[string]any{
		"type":        "object",
		"description": "Kubernetes network plugin (CNI) configuration. Only one plugin should be enabled at a time.",
		"properties": map[string]any{
			"calico": map[string]any{
				"type":        "object",
				"description": "Calico CNI configuration",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable Calico as the CNI plugin",
						"default":     false,
					},
					"cni_iface": map[string]any{
						"type":        "string",
						"description": "Network interface name for Calico CNI",
						"pattern":     "^[a-zA-Z0-9]+$",
						"examples":    []string{"enp3s0", "eth0", "ens3"},
					},
					"calico_interface_autodetect": map[string]any{
						"type":        "string",
						"description": "Interface autodetection method",
						"enum":        []string{"interface", "can-reach", "skip-interface", "cidr"},
						"default":     "interface",
					},
				},
			},
			"cilium": map[string]any{
				"type":        "object",
				"description": "Cilium CNI configuration",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable Cilium as the CNI plugin",
						"default":     false,
					},
					"operator_enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable Cilium operator for advanced features",
						"default":     true,
					},
					"kubeProxyReplacement": map[string]any{
						"type":        "boolean",
						"description": "Replace kube-proxy with Cilium's eBPF implementation",
						"default":     true,
					},
				},
			},
			"kube-ovn": map[string]any{
				"type":        "object",
				"description": "Kube-OVN CNI configuration",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable Kube-OVN as the CNI plugin",
						"default":     false,
					},
					"cilium_integration": map[string]any{
						"type":        "boolean",
						"description": "Enable Cilium integration for advanced networking features",
						"default":     true,
					},
				},
			},
		},
		"oneOf": []map[string]any{
			{"properties": map[string]any{"calico": map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": true}}}}},
			{"properties": map[string]any{"cilium": map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": true}}}}},
			{"properties": map[string]any{"kube-ovn": map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": true}}}}},
			{"properties": map[string]any{
				"calico":   map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": false}}},
				"cilium":   map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": false}}},
				"kube-ovn": map[string]any{"properties": map[string]any{"enabled": map[string]any{"const": false}}},
			}},
		},
	}

	cluster := map[string]any{
		"type":        "object",
		"description": "Kubernetes cluster configuration",
		"required":    []string{"cluster_name", "kubernetes"},
		"properties": map[string]any{
			"cluster_name": map[string]any{
				"type":        "string",
				"description": "Unique cluster name (used for resource naming)",
				"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
				"minLength":   3,
				"maxLength":   63,
			},
			"base_domain": map[string]any{
				"type":        "string",
				"description": "Base domain for the cluster (e.g. k8s.opencenter.cloud)",
				"format":      "hostname",
				"default":     "k8s.opencenter.cloud",
			},
			"cluster_fqdn": map[string]any{
				"type":        "string",
				"description": "Fully qualified domain name for the cluster",
				"format":      "hostname",
			},
			"admin_email": map[string]any{
				"type":        "string",
				"description": "Administrator email address for certificates and notifications",
				"format":      "email",
				"default":     "admin@example.com",
			},
			"aws_access_key": map[string]any{
				"type":        "string",
				"description": "AWS access key for cluster resources",
				"minLength":   16,
			},
			"aws_secret_access_key": map[string]any{
				"type":        "string",
				"description": "AWS secret access key for cluster resources",
				"minLength":   32,
			},
			"k8s_api_port_acl": map[string]any{
				"type":        "array",
				"description": "CIDR blocks allowed to access Kubernetes API server",
				"items": map[string]any{
					"type":    "string",
					"pattern": "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
				},
				"default":  []string{"0.0.0.0/0"},
				"minItems": 1,
			},
			"ssh_authorized_keys": map[string]any{
				"type":        "array",
				"description": "SSH public keys for node access",
				"items": map[string]any{
					"type":      "string",
					"pattern":   "^(ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521) ",
					"minLength": 100,
				},
				"minItems": 1,
			},
			"kubernetes": map[string]any{
				"type":        "object",
				"description": "Kubernetes cluster settings",
				"required":    []string{"version", "master_count", "worker_count"},
				"properties": map[string]any{
					"version": map[string]any{
						"type":        "string",
						"description": "Kubernetes version",
						"pattern":     "^[0-9]+\\.[0-9]+\\.[0-9]+$",
						"examples":    []string{"1.31.4", "1.30.0", "1.29.2"},
					},
					"flavor_bastion": map[string]any{
						"type":        "string",
						"description": "Instance flavor/size for bastion host",
						"minLength":   1,
					},
					"flavor_master": map[string]any{
						"type":        "string",
						"description": "Instance flavor/size for control plane nodes",
						"minLength":   1,
					},
					"flavor_worker": map[string]any{
						"type":        "string",
						"description": "Instance flavor/size for worker nodes",
						"minLength":   1,
					},
					"subnet_pods": map[string]any{
						"type":        "string",
						"description": "CIDR block for pod network",
						"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
						"default":     "10.42.0.0/16",
					},
					"subnet_services": map[string]any{
						"type":        "string",
						"description": "CIDR block for service network",
						"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
						"default":     "10.43.0.0/16",
					},
					"loadbalancer_provider": map[string]any{
						"type":        "string",
						"description": "Load balancer provider",
						"enum":        []string{"ovn", "octavia", "metallb", "none"},
						"default":     "ovn",
					},
					"dns_zone_name": map[string]any{
						"type":        "string",
						"description": "DNS zone name for cluster services",
						"format":      "hostname",
					},
					"master_count": map[string]any{
						"type":        "integer",
						"description": "Number of control plane nodes",
						"minimum":     1,
						"maximum":     9,
						"default":     3,
					},
					"worker_count": map[string]any{
						"type":        "integer",
						"description": "Number of worker nodes",
						"minimum":     0,
						"maximum":     100,
						"default":     2,
					},
					"worker_count_windows": map[string]any{
						"type":        "integer",
						"description": "Number of Windows worker nodes",
						"minimum":     0,
						"maximum":     50,
						"default":     0,
					},
					"master_nodes": map[string]any{
						"type":        "array",
						"description": "Pre-configured master/control plane nodes for baremetal deployments",
						"items": map[string]any{
							"type":        "object",
							"description": "Baremetal node configuration",
							"required":    []string{"id", "name", "access_ip_v4"},
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "Unique identifier for the node",
									"minLength":   1,
								},
								"name": map[string]any{
									"type":        "string",
									"description": "Hostname or name of the node",
									"minLength":   1,
								},
								"access_ip_v4": map[string]any{
									"type":        "string",
									"description": "IPv4 address for accessing the node",
									"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}$",
								},
							},
						},
					},
					"worker_nodes": map[string]any{
						"type":        "array",
						"description": "Pre-configured worker nodes for baremetal deployments",
						"items": map[string]any{
							"type":        "object",
							"description": "Baremetal node configuration",
							"required":    []string{"id", "name", "access_ip_v4"},
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "Unique identifier for the node",
									"minLength":   1,
								},
								"name": map[string]any{
									"type":        "string",
									"description": "Hostname or name of the node",
									"minLength":   1,
								},
								"access_ip_v4": map[string]any{
									"type":        "string",
									"description": "IPv4 address for accessing the node",
									"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}$",
								},
							},
						},
					},
					"additional_server_pools_worker": map[string]any{
						"type":        "array",
						"description": "Additional worker node pools with custom configurations",
						"items": map[string]any{
							"type":                 "object",
							"description":          "Configuration for an additional worker node pool",
							"required":             []string{"name", "worker_count", "flavor_worker", "node_worker"},
							"additionalProperties": false,
							"properties": map[string]any{
								"name": map[string]any{
									"type":        "string",
									"description": "Unique name for this worker pool",
									"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
									"minLength":   1,
								},
								"worker_count": map[string]any{
									"type":        "integer",
									"description": "Number of worker nodes in this pool",
									"minimum":     0,
									"maximum":     100,
								},
								"flavor_worker": map[string]any{
									"type":        "string",
									"description": "Instance flavor/size for this worker pool",
									"minLength":   1,
								},
								"node_worker": map[string]any{
									"type":        "string",
									"description": "Node suffix identifier for this worker pool",
									"minLength":   1,
								},
								"server_group_affinity": map[string]any{
									"type":        "string",
									"description": "Server group affinity policy for this worker pool",
									"enum":        []string{"affinity", "anti-affinity", "soft-affinity", "soft-anti-affinity"},
								},
								"image_id": map[string]any{
									"type":        "string",
									"description": "OpenStack image ID for this worker pool",
									"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
								},
								"image_name": map[string]any{
									"type":        "string",
									"description": "OpenStack image name (alternative to image_id)",
								},
								"worker_node_bfv_volume_size": map[string]any{
									"type":        "integer",
									"description": "Boot volume size in GB",
									"default":     40,
									"minimum":     10,
									"maximum":     1000,
								},
								"worker_node_bfv_destination_type": map[string]any{
									"type":        "string",
									"description": "Boot volume destination type",
									"enum":        []string{"volume", "local"},
									"default":     "volume",
								},
								"worker_node_bfv_source_type": map[string]any{
									"type":        "string",
									"description": "Boot volume source type",
									"enum":        []string{"image", "volume", "snapshot"},
									"default":     "image",
								},
								"worker_node_bfv_volume_type": map[string]any{
									"type":        "string",
									"description": "Boot volume type (e.g., HA-Standard, HA-Performance)",
								},
								"worker_node_bfv_delete_on_termination": map[string]any{
									"type":        "boolean",
									"description": "Delete boot volume when instance is terminated",
									"default":     true,
								},
								"additional_block_devices_worker": map[string]any{
									"type":        "array",
									"description": "Additional block devices for this worker pool",
									"items": map[string]any{
										"type": "object",
									},
								},
								"pf9_onboard": map[string]any{
									"type":        "boolean",
									"description": "Enable Platform9 onboarding for this pool",
									"default":     false,
								},
								"subnet_id": map[string]any{
									"type":        "string",
									"description": "Specific subnet ID for this worker pool (optional)",
									"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
								},
							},
						},
					},
					"additional_server_pools_worker_windows": map[string]any{
						"type":        "array",
						"description": "Additional Windows worker node pools with custom configurations",
						"items": map[string]any{
							"type":                 "object",
							"description":          "Configuration for an additional Windows worker node pool",
							"required":             []string{"name", "worker_count", "flavor_worker", "node_worker"},
							"additionalProperties": false,
							"properties": map[string]any{
								"name": map[string]any{
									"type":        "string",
									"description": "Unique name for this Windows worker pool",
									"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
									"minLength":   1,
								},
								"worker_count": map[string]any{
									"type":        "integer",
									"description": "Number of Windows worker nodes in this pool",
									"minimum":     0,
									"maximum":     50,
								},
								"flavor_worker": map[string]any{
									"type":        "string",
									"description": "Instance flavor/size for this Windows worker pool",
									"minLength":   1,
								},
								"node_worker": map[string]any{
									"type":        "string",
									"description": "Node suffix identifier for this Windows worker pool",
									"minLength":   1,
								},
								"server_group_affinity": map[string]any{
									"type":        "string",
									"description": "Server group affinity policy for this Windows worker pool",
									"enum":        []string{"affinity", "anti-affinity", "soft-affinity", "soft-anti-affinity"},
								},
								"image_id": map[string]any{
									"type":        "string",
									"description": "OpenStack Windows image ID for this worker pool",
									"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
								},
							},
						},
					},
					"network_plugin": networkPlugin,
					"oidc": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"enabled":                   map[string]any{"type": "boolean"},
							"kube_oidc_url":             map[string]any{"type": "string"},
							"kube_oidc_client_id":       map[string]any{"type": "string"},
							"kube_oidc_ca_file":         map[string]any{"type": "string"},
							"kube_oidc_username_claim":  map[string]any{"type": "string"},
							"kube_oidc_username_prefix": map[string]any{"type": "string"},
							"kube_oidc_groups_claim":    map[string]any{"type": "string"},
							"kube_oidc_groups_prefix":   map[string]any{"type": "string"},
						},
					},
					"windows_workers": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"enabled":                      map[string]any{"type": "boolean"},
							"windows_user":                 map[string]any{"type": "string"},
							"windows_admin_password":       map[string]any{"type": "string"},
							"worker_node_bfv_size_windows": map[string]any{"type": "integer"},
							"worker_node_bfv_type_windows": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	gitops := map[string]any{
		"type":        "object",
		"description": "GitOps repository configuration for cluster manifests",
		"required":    []string{"git_dir"},
		"properties": map[string]any{
			"git_branch": map[string]any{
				"type":        "string",
				"description": "Git branch for GitOps repository",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
				"default":     "main",
			},
			"git_dir": map[string]any{
				"type":        "string",
				"description": "Local directory path for GitOps repository",
				"minLength":   1,
			},
			"git_ssh_key": map[string]any{
				"type":        "string",
				"description": "Path to SSH private key for Git authentication",
				"pattern":     "^[~./].*",
			},
			"git_ssh_pub": map[string]any{
				"type":        "string",
				"description": "Path to SSH public key for Git authentication",
				"pattern":     "^[~./].*",
			},
			"git_url": map[string]any{
				"type":        "string",
				"description": "Git repository URL (SSH or HTTPS)",
				"pattern":     "^(https?://|git@|ssh://)",
			},
			"gitops_base_repo": map[string]any{
				"type":        "string",
				"description": "URL of the GitOps base repository",
				"pattern":     "^(https?://|git@|ssh://)",
				"default":     "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
			},
			"gitops_base_release": map[string]any{
				"type":        "string",
				"description": "Release tag of the GitOps base repository",
				"pattern":     "^[a-zA-Z0-9._-]+$",
				"default":     "v0.1.0",
			},
			"gitops_branch": map[string]any{
				"type":        "string",
				"description": "Branch of the GitOps base repository",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
				"default":     "main",
			},
			"release": map[string]any{
				"type":        "string",
				"description": "GitOps base release version (mutually exclusive with branch)",
				"pattern":     "^[a-zA-Z0-9._-]+$",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch for GitOps base (mutually exclusive with release)",
				"pattern":     "^[a-zA-Z0-9/_-]+$",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Git repository URI for GitOps base",
				"pattern":     "^(https?://|git@|ssh://)",
			},
			"flux": map[string]any{
				"type":        "object",
				"description": "FluxCD reconciliation settings",
				"properties": map[string]any{
					"interval": map[string]any{
						"type":        "string",
						"description": "Reconciliation interval (e.g., 15m, 1h)",
						"pattern":     "^[0-9]+(s|m|h)$",
						"default":     "15m",
					},
					"prune": map[string]any{
						"type":        "boolean",
						"description": "Enable pruning of resources not in Git",
						"default":     true,
					},
				},
			},
		},
	}

	managedService := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"alert-proxy": baseServiceSchema, // Use baseServiceSchema instead of the removed serviceSchema
		},
		"additionalProperties": baseServiceSchema,
	}

	// Remove the generic serviceSchema that contains all fields from all services
	// This was causing schema bloat where every service got every possible field

	services := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calico":                calicoServiceSchema,
			"cert-manager":          certManagerSchema,
			"etcd-backup":           etcdBackupSchema,
			"external-snapshotter":  baseServiceSchema,
			"fluxcd":                baseServiceSchema,
			"gateway":               baseServiceSchema,
			"gateway-api":           baseServiceSchema,
			"headlamp":              baseServiceSchema,
			"keycloak":              baseServiceSchema,
			"kube-prometheus-stack": baseServiceSchema,
			"kyverno":               baseServiceSchema,
			"loki":                  baseServiceSchema, // TODO: Create specific loki schema when needed
			"olm":                   baseServiceSchema,
			"opencenter_release":    baseServiceSchema,
			"openstack-ccm":         baseServiceSchema,
			"openstack-csi":         baseServiceSchema,
			"postgres-operator":     baseServiceSchema,
			"rbac-manager":          baseServiceSchema,
			"sources":               baseServiceSchema,
			"velero":                baseServiceSchema, // TODO: Create specific velero schema when needed
			"vsphere-csi":           baseServiceSchema,
			"weave-gitops":          baseServiceSchema,
		},
		// Use baseServiceSchema for any additional services instead of the bloated serviceSchema
		"additionalProperties": baseServiceSchema,
	}

	schema := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://opencenter.cloud/schemas/cluster-config.json",
		"title":       "openCenter Cluster Configuration",
		"description": "Complete schema for openCenter cluster configuration with validation rules and constraints",
		"version":     SchemaVersion,
		"type":        "object",
		"required":    []string{"opencenter"},
		"properties": map[string]any{
			"opencenter": map[string]any{
				"type":        "object",
				"description": "Main openCenter configuration section",
				"required":    []string{"meta", "infrastructure", "cluster", "gitops"},
				"properties": map[string]any{
					"meta": map[string]any{
						"type":        "object",
						"description": "Cluster metadata and organizational information",
						"required":    []string{"name"},
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "Cluster name (must match cluster_name in cluster section)",
								"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
								"minLength":   3,
								"maxLength":   63,
							},
							"env": map[string]any{
								"type":        "string",
								"description": "Environment designation",
								"enum":        []string{"dev", "stage", "prod", "test", ""},
							},
							"region": map[string]any{
								"type":        "string",
								"description": "Deployment region",
								"minLength":   1,
							},
							"status": map[string]any{
								"type":        "string",
								"description": "Cluster status",
								"enum":        []string{"active", "inactive", "maintenance", ""},
							},
							"organization": map[string]any{
								"type":        "string",
								"description": "Organization name for multi-tenant deployments",
								"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
								"default":     "opencenter",
								"minLength":   1,
								"maxLength":   63,
							},
						},
					},
					"infrastructure": infrastructure,
					"cluster":        cluster,
					"gitops":         gitops,
					"storage": map[string]any{
						"type":        "object",
						"description": "Storage configuration for the cluster",
						"properties": map[string]any{
							"default_storage_class": map[string]any{
								"type":        "string",
								"description": "Default storage class for persistent volumes",
								"default":     "csi-cinder-sc-delete",
							},
						},
					},
					"talos": map[string]any{
						"type":        "object",
						"description": "Talos Linux provider configuration for immutable Kubernetes clusters",
						"properties": map[string]any{
							"enabled": map[string]any{
								"type":        "boolean",
								"description": "Enable Talos Linux provider",
								"default":     false,
							},
							"version": map[string]any{
								"type":        "string",
								"description": "Talos Linux version",
								"pattern":     "^v[0-9]+\\.[0-9]+\\.[0-9]+$",
								"examples":    []string{"v1.8.0", "v1.7.0"},
							},
							"image_url": map[string]any{
								"type":        "string",
								"description": "URL to Talos Linux image",
								"format":      "uri",
							},
							"image_signature": map[string]any{
								"type":        "string",
								"description": "Cryptographic signature of Talos image",
								"minLength":   64,
							},
							"machine_config": map[string]any{
								"type":        "object",
								"description": "Talos machine configuration settings",
								"properties": map[string]any{
									"apparmor_enabled": map[string]any{
										"type":        "boolean",
										"description": "Enable AppArmor security profiles",
										"default":     true,
									},
									"seccomp_enabled": map[string]any{
										"type":        "boolean",
										"description": "Enable Seccomp security profiles",
										"default":     true,
									},
									"disk_encryption": map[string]any{
										"type":        "boolean",
										"description": "Enable disk encryption with LUKS",
										"default":     true,
									},
									"kubeprism_enabled": map[string]any{
										"type":        "boolean",
										"description": "Enable KubePrism for internal load balancing",
										"default":     true,
									},
									"system_extensions": map[string]any{
										"type":        "array",
										"description": "List of Talos system extensions to install",
										"items": map[string]any{
											"type": "string",
										},
									},
									"log_destination": map[string]any{
										"type":        "string",
										"description": "Destination for Talos system logs",
										"format":      "uri",
									},
								},
							},
							"network_config": map[string]any{
								"type":        "object",
								"description": "Network topology settings",
								"properties": map[string]any{
									"management_subnet": map[string]any{
										"type":        "string",
										"description": "CIDR for management network",
										"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
										"default":     "10.0.1.0/24",
									},
									"control_subnet": map[string]any{
										"type":        "string",
										"description": "CIDR for control plane network",
										"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
										"default":     "10.0.2.0/24",
									},
									"data_subnet": map[string]any{
										"type":        "string",
										"description": "CIDR for data plane network",
										"pattern":     "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
										"default":     "10.0.3.0/24",
									},
									"wireguard_port": map[string]any{
										"type":        "integer",
										"description": "UDP port for WireGuard VPN",
										"minimum":     1024,
										"maximum":     65535,
										"default":     51820,
									},
									"talos_api_port": map[string]any{
										"type":        "integer",
										"description": "TCP port for Talos API",
										"minimum":     1024,
										"maximum":     65535,
										"default":     50000,
									},
									"allowed_cidrs": map[string]any{
										"type":        "array",
										"description": "List of CIDRs allowed to access cluster",
										"items": map[string]any{
											"type":    "string",
											"pattern": "^([0-9]{1,3}\\.){3}[0-9]{1,3}/[0-9]{1,2}$",
										},
									},
								},
							},
							"security_config": map[string]any{
								"type":        "object",
								"description": "Security-related settings",
								"properties": map[string]any{
									"vtpm_enabled": map[string]any{
										"type":        "boolean",
										"description": "Enable vTPM for hardware-backed encryption",
										"default":     true,
									},
									"barbican_key_id": map[string]any{
										"type":        "string",
										"description": "Barbican key ID for encryption",
										"pattern":     "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
									},
									"image_verification": map[string]any{
										"type":        "boolean",
										"description": "Enable cryptographic image verification",
										"default":     true,
									},
									"mfa_required": map[string]any{
										"type":        "boolean",
										"description": "Require MFA for administrative access",
										"default":     true,
									},
									"audit_log_enabled": map[string]any{
										"type":        "boolean",
										"description": "Enable audit logging",
										"default":     true,
									},
								},
							},
							"pulumi_config": map[string]any{
								"type":        "object",
								"description": "Pulumi-specific settings",
								"properties": map[string]any{
									"stack_name": map[string]any{
										"type":        "string",
										"description": "Pulumi stack name",
										"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
									},
									"swift_container": map[string]any{
										"type":        "string",
										"description": "Swift container for Pulumi state",
										"pattern":     "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
									},
									"swift_prefix": map[string]any{
										"type":        "string",
										"description": "Swift prefix for state isolation",
										"pattern":     "^[a-z0-9][a-z0-9-/]*[a-z0-9]$",
									},
									"secrets_passphrase": map[string]any{
										"type":        "string",
										"description": "Passphrase for Pulumi secrets provider (should be SOPS encrypted)",
										"minLength":   32,
									},
								},
							},
						},
					},
					"managed-service": managedService,
					"services":        services,
				},
			},
			"opentofu": map[string]any{
				"type":        "object",
				"description": "OpenTofu/Terraform infrastructure-as-code configuration",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable OpenTofu for infrastructure provisioning",
						"default":     true,
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Path to OpenTofu binary or working directory",
						"default":     "opentofu",
					},
					"backend": map[string]any{
						"type":        "object",
						"description": "OpenTofu state backend configuration",
						"required":    []string{"type"},
						"properties": map[string]any{
							"type": map[string]any{
								"type":        "string",
								"description": "Backend type",
								"enum":        []string{"local", "s3", "azurerm", "gcs"},
								"default":     "local",
							},
							"local": map[string]any{
								"type":        "object",
								"description": "Local backend configuration",
								"properties": map[string]any{
									"path": map[string]any{
										"type":        "string",
										"description": "Path to state file",
										"default":     "terraform.tfstate",
									},
								},
							},
							"s3": map[string]any{
								"type":        "object",
								"description": "S3 backend configuration",
								"required":    []string{"bucket", "key", "region"},
								"properties": map[string]any{
									"bucket": map[string]any{
										"type":        "string",
										"description": "S3 bucket name for state storage",
										"minLength":   3,
									},
									"key": map[string]any{
										"type":        "string",
										"description": "S3 object key for state file",
										"minLength":   1,
									},
									"region": map[string]any{
										"type":        "string",
										"description": "AWS region for S3 bucket",
										"pattern":     "^[a-z]{2}-[a-z]+-[0-9]{1}$",
									},
									"endpoint": map[string]any{
										"type":        "string",
										"description": "Custom S3 endpoint URL",
										"format":      "uri",
									},
									"profile": map[string]any{
										"type":        "string",
										"description": "AWS CLI profile name",
									},
									"encrypt": map[string]any{
										"type":        "boolean",
										"description": "Enable server-side encryption",
										"default":     true,
									},
								},
							},
						},
					},
				},
			},
			"secrets": map[string]any{
				"type":        "object",
				"description": "Secrets management configuration",
				"properties": map[string]any{
					"sops_age_key_file": map[string]any{
						"type":        "string",
						"description": "Path to SOPS Age encryption key file",
						"pattern":     "^[~./].*",
					},
					"ssh_key": map[string]any{
						"type":        "object",
						"description": "SSH key configuration for cluster access",
						"properties": map[string]any{
							"private": map[string]any{
								"type":        "string",
								"description": "Path to SSH private key file",
								"pattern":     "^[~./].*",
							},
							"public": map[string]any{
								"type":        "string",
								"description": "Path to SSH public key file",
								"pattern":     "^[~./].*\\.pub$",
							},
							"cypher": map[string]any{
								"type":        "string",
								"description": "SSH key encryption algorithm",
								"enum":        []string{"ed25519", "rsa", "ecdsa"},
								"default":     "ed25519",
							},
						},
					},
					"cert_manager": map[string]any{
						"type":        "object",
						"description": "Cert-manager secret values",
						"properties": map[string]any{
							"aws_access_key": map[string]any{
								"type":        "string",
								"description": "AWS access key for Route53 DNS validation",
								"secret":      true,
							},
							"aws_secret_access_key": map[string]any{
								"type":        "string",
								"description": "AWS secret access key for Route53 DNS validation",
								"secret":      true,
							},
						},
					},
					"loki": map[string]any{
						"type":        "object",
						"description": "Loki secret values",
						"properties": map[string]any{
							"swift_password": map[string]any{
								"type":        "string",
								"description": "Swift storage password (deprecated: use application credentials)",
								"secret":      true,
							},
							"swift_application_credential_secret": map[string]any{
								"type":        "string",
								"description": "Swift application credential secret (recommended)",
								"secret":      true,
							},
							"s3_access_key_id": map[string]any{
								"type":        "string",
								"description": "S3 access key ID",
								"secret":      true,
							},
							"s3_secret_access_key": map[string]any{
								"type":        "string",
								"description": "S3 secret access key",
								"secret":      true,
							},
						},
					},
					"keycloak": map[string]any{
						"type":        "object",
						"description": "Keycloak secret values",
						"properties": map[string]any{
							"client_secret": map[string]any{
								"type":        "string",
								"description": "Keycloak OIDC client secret",
								"secret":      true,
							},
							"admin_password": map[string]any{
								"type":        "string",
								"description": "Keycloak admin user password",
								"secret":      true,
							},
						},
					},
					"headlamp": map[string]any{
						"type":        "object",
						"description": "Headlamp secret values",
						"properties": map[string]any{
							"oidc_client_secret": map[string]any{
								"type":        "string",
								"description": "Headlamp OIDC client secret",
								"secret":      true,
							},
						},
					},
					"weave_gitops": map[string]any{
						"type":        "object",
						"description": "Weave GitOps secret values",
						"properties": map[string]any{
							"password": map[string]any{
								"type":        "string",
								"description": "Weave GitOps admin password",
								"secret":      true,
							},
							"password_hash": map[string]any{
								"type":        "string",
								"description": "Weave GitOps admin password hash (bcrypt)",
								"secret":      true,
							},
						},
					},
					"grafana": map[string]any{
						"type":        "object",
						"description": "Grafana secret values",
						"properties": map[string]any{
							"admin_password": map[string]any{
								"type":        "string",
								"description": "Grafana admin password",
								"secret":      true,
							},
						},
					},
					"tempo": map[string]any{
						"type":        "object",
						"description": "Tempo secret values",
						"properties": map[string]any{
							"access_key": map[string]any{
								"type":        "string",
								"description": "Tempo S3 access key",
								"secret":      true,
							},
							"secret_key": map[string]any{
								"type":        "string",
								"description": "Tempo S3 secret key",
								"secret":      true,
							},
						},
					},
					"alert_proxy": map[string]any{
						"type":        "object",
						"description": "Alert proxy secret values",
						"properties": map[string]any{
							"core_device_id": map[string]any{
								"type":        "string",
								"description": "Alert proxy core device ID",
								"secret":      true,
							},
							"account_service_token": map[string]any{
								"type":        "string",
								"description": "Alert proxy account service token",
								"secret":      true,
							},
							"core_account_number": map[string]any{
								"type":        "string",
								"description": "Alert proxy core account number",
								"secret":      true,
							},
						},
					},
				},
			},
			"overrides": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Configuration metadata for tracking lifecycle and provenance",
				"properties": map[string]any{
					"created_at": map[string]any{
						"type":        "string",
						"description": "Timestamp when the configuration was created",
						"format":      "date-time",
					},
					"updated_at": map[string]any{
						"type":        "string",
						"description": "Timestamp when the configuration was last updated",
						"format":      "date-time",
					},
					"created_by": map[string]any{
						"type":        "string",
						"description": "User or system that created the configuration",
						"minLength":   1,
					},
					"tags": map[string]any{
						"type":                 "object",
						"description":          "Custom tags for categorization and filtering",
						"additionalProperties": map[string]any{"type": "string"},
					},
					"annotations": map[string]any{
						"type":                 "object",
						"description":          "Custom annotations for additional metadata",
						"additionalProperties": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	if pretty {
		return json.MarshalIndent(schema, "", "  ")
	}
	return json.Marshal(schema)
}

// GenerateDefaultFromSchema returns the YAML defaults used by cluster init.
func GenerateDefaultFromSchema(name string) ([]byte, error) {
	cfg := defaultConfig(name)
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default config: %w", err)
	}
	return out, nil
}

// GenerateFullSchemaDefaults returns a YAML configuration with all available fields
// including examples of Terraform local value references for advanced users.
// This is used when the --full-schema flag is specified during cluster init.
func GenerateFullSchemaDefaults(name string) ([]byte, error) {
	cfg := defaultConfig(name)

	// Convert to map for easier manipulation
	cfgYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var configMap map[string]any
	if err := yaml.Unmarshal(cfgYAML, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config to map: %w", err)
	}

	// Add iac section with local value examples
	configMap["iac"] = map[string]any{
		"main": map[string]any{
			"local": map[string]any{
				"cluster_name":                       fmt.Sprintf("local.cluster_name = \"%s\"", name),
				"region":                             "local.region = \"sjc3\"",
				"environment":                        "local.environment = \"dev\"",
				"kubelet_rotate_server_certificates": "local.kubelet_rotate_server_certificates = false",
				"example_comment":                    "# Terraform local values can be referenced in your infrastructure code",
			},
		},
	}

	// Marshal back to YAML with the iac section
	out, err := yaml.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal full schema config: %w", err)
	}

	return out, nil
}

// GetSchemaVersion returns the current schema version for backward compatibility tracking.
func GetSchemaVersion() string {
	return SchemaVersion
}

// ValidateSchemaVersion checks if a given schema version is compatible with the current version.
// Returns true if compatible, false otherwise.
func ValidateSchemaVersion(version string) bool {
	// For now, we only support exact version match
	// In the future, this could implement semantic versioning compatibility checks
	return version == SchemaVersion
}

// DetectSchemaMigrationNeeded checks if a configuration needs schema migration.
// It compares the config's schema version with the current schema version.
//
// Inputs:
//   - config: The configuration to check
//
// Outputs:
//   - bool: true if migration is needed, false otherwise
//   - string: The current schema version of the config (empty if not set)
//   - error: An error if version comparison fails
func DetectSchemaMigrationNeeded(config Config) (bool, string, error) {
	configVersion := config.SchemaVersion

	// If no schema version is set, assume it's an old config that needs migration
	if configVersion == "" {
		return true, "", nil
	}

	// If versions match, no migration needed
	if configVersion == SchemaVersion {
		return false, configVersion, nil
	}

	// Versions differ - migration may be needed
	// In the future, this could use semantic versioning to determine
	// if the difference requires migration or is backward compatible
	return true, configVersion, nil
}

// GetMigrationPath determines the migration path from one schema version to another.
// Returns a list of intermediate versions that need to be migrated through.
//
// Inputs:
//   - fromVersion: The starting schema version
//   - toVersion: The target schema version
//
// Outputs:
//   - []string: List of versions to migrate through (empty if no migration needed)
//   - error: An error if the migration path is invalid
func GetMigrationPath(fromVersion, toVersion string) ([]string, error) {
	// If versions are the same, no migration needed
	if fromVersion == toVersion {
		return []string{}, nil
	}

	// If fromVersion is empty, treat as pre-versioned config
	if fromVersion == "" {
		fromVersion = "0.0.0"
	}

	// For now, we only support direct migration to current version
	// In the future, this could support multi-step migrations
	if toVersion != SchemaVersion {
		return nil, fmt.Errorf("can only migrate to current schema version %s, requested %s", SchemaVersion, toVersion)
	}

	// Return single-step migration path
	return []string{toVersion}, nil
}

// NeedsMigration is a convenience function that checks if a config needs migration
// to the current schema version.
//
// Inputs:
//   - config: The configuration to check
//
// Outputs:
//   - bool: true if migration is needed, false otherwise
func NeedsMigration(config Config) bool {
	needed, _, _ := DetectSchemaMigrationNeeded(config)
	return needed
}

// SetSchemaVersion updates the schema version field in a configuration.
// This should be called after successful migration.
//
// Inputs:
//   - config: Pointer to the configuration to update
//   - version: The new schema version to set
func SetSchemaVersion(config *Config, version string) {
	config.SchemaVersion = version
}
