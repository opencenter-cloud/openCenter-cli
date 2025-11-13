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

	// Generic service schema for services with all possible fields
	serviceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"aws_access_key":         map[string]any{"type": "string"},
			"aws_secret_access_key":  map[string]any{"type": "string"},
			"email":                  map[string]any{"type": "string"},
			"enabled":                map[string]any{"type": "boolean"},
			"region":                 map[string]any{"type": "string"},
			"release":                map[string]any{"type": "string", "description": "Release version or tag (mutually exclusive with branch)"},
			"branch":                 map[string]any{"type": "string", "description": "Git branch (mutually exclusive with release)"},
			"uri":                    map[string]any{"type": "string", "description": "Git repository URI"},
			"s3_host":                map[string]any{"type": "string"},
			"s3_region":              map[string]any{"type": "string"},
			"core_device_id":         map[string]any{"type": "string"},
			"account_service_token":  map[string]any{"type": "string"},
			"alert_manager_base_url": map[string]any{"type": "string"},
			"core_account_number":    map[string]any{"type": "string"},
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
			"alert-proxy": serviceSchema,
		},
		"additionalProperties": serviceSchema,
	}

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
			"olm":                   baseServiceSchema,
			"opencenter_release":    baseServiceSchema,
			"openstack-ccm":         baseServiceSchema,
			"openstack-csi":         baseServiceSchema,
			"postgres-operator":     baseServiceSchema,
			"rbac-manager":          baseServiceSchema,
			"sources":               baseServiceSchema,
			"velero":                baseServiceSchema,
			"weave-gitops":          baseServiceSchema,
		},
		"additionalProperties": serviceSchema,
	}

	schema := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://opencenter.cloud/schemas/cluster-config.json",
		"title":       "openCenter Cluster Configuration",
		"description": "Complete schema for openCenter cluster configuration with validation rules and constraints",
		"version":     SchemaVersion,
		"type":        "object",
		"required": []string{"opencenter"},
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
					"infrastructure":  infrastructure,
					"cluster":         cluster,
					"gitops":          gitops,
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
				},
			},
			"overrides": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
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
