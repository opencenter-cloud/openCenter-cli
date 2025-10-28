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

// GenerateSchema returns a JSON schema (Draft 2020-12) describing the current
// cluster configuration structure. The schema mirrors the structure emitted by
// defaultConfig / cluster init so IDE integrations stay in sync with runtime.
func GenerateSchema(pretty bool) ([]byte, error) {
	serviceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enabled":              map[string]any{"type": "boolean"},
			"email":                map[string]any{"type": "string"},
			"region":               map[string]any{"type": "string"},
			"s3_host":              map[string]any{"type": "string"},
			"s3_region":            map[string]any{"type": "string"},
			"aws_access_key":       map[string]any{"type": "string"},
			"aws_secret_access_key": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}

	infrastructure := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"provider": map[string]any{"type": "string"},
			"cloud": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"aws": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"profile":         map[string]any{"type": "string"},
							"region":          map[string]any{"type": "string"},
							"vpc_id":          map[string]any{"type": "string"},
							"private_subnets": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"public_subnets":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						},
					},
					"openstack": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"auth_url":                      map[string]any{"type": "string"},
							"insecure":                      map[string]any{"type": "boolean"},
							"region":                        map[string]any{"type": "string"},
							"application_credential_id":     map[string]any{"type": "string"},
							"application_credential_secret": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	networkPlugin := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calico": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled":                     map[string]any{"type": "boolean"},
					"cni_iface":                   map[string]any{"type": "string"},
					"calico_interface_autodetect": map[string]any{"type": "string"},
				},
			},
			"cilium": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled":              map[string]any{"type": "boolean"},
					"operator_enabled":     map[string]any{"type": "boolean"},
					"kubeProxyReplacement": map[string]any{"type": "boolean"},
				},
			},
			"kube-ovn": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled":            map[string]any{"type": "boolean"},
					"cilium_integration": map[string]any{"type": "boolean"},
				},
			},
		},
	}

	cluster := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cluster_name":          map[string]any{"type": "string"},
			"aws_access_key":        map[string]any{"type": "string"},
			"aws_secret_access_key": map[string]any{"type": "string"},
			"k8s_api_port_acl":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"ssh_authorized_keys":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"kubernetes": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"version":               map[string]any{"type": "string"},
					"flavor_bastion":        map[string]any{"type": "string"},
					"flavor_master":         map[string]any{"type": "string"},
					"flavor_worker":         map[string]any{"type": "string"},
					"subnet_pods":           map[string]any{"type": "string"},
					"subnet_services":       map[string]any{"type": "string"},
					"loadbalancer_provider": map[string]any{"type": "string"},
					"dns_zone_name":         map[string]any{"type": "string"},
					"master_count":          map[string]any{"type": "integer"},
					"worker_count":          map[string]any{"type": "integer"},
					"worker_count_windows":  map[string]any{"type": "integer"},
					"network_plugin":        networkPlugin,
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
		"type": "object",
		"properties": map[string]any{
			"git_branch":  map[string]any{"type": "string"},
			"git_dir":     map[string]any{"type": "string"},
			"git_ssh_key": map[string]any{"type": "string"},
			"git_ssh_pub": map[string]any{"type": "string"},
			"git_url":     map[string]any{"type": "string"},
			"flux": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"interval": map[string]any{"type": "string"},
					"prune":    map[string]any{"type": "boolean"},
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
			"calico":                serviceSchema,
			"cert-manager":          serviceSchema,
			"etcd-backup":           serviceSchema,
			"external-snapshotter":  serviceSchema,
			"fluxcd":                serviceSchema,
			"gateway":               serviceSchema,
			"gateway-api":           serviceSchema,
			"headlamp":              serviceSchema,
			"keycloak":              serviceSchema,
			"kube-prometheus-stack": serviceSchema,
			"kyverno":               serviceSchema,
			"olm":                   serviceSchema,
			"openstack-ccm":         serviceSchema,
			"openstack-csi":         serviceSchema,
			"postgres-operator":     serviceSchema,
			"rbac-manager":          serviceSchema,
			"sources":               serviceSchema,
			"velero":                serviceSchema,
			"weave-gitops":          serviceSchema,
		},
		"additionalProperties": serviceSchema,
	}

	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   "openCenter Cluster Configuration",
		"type":    "object",
		"properties": map[string]any{
			"opencenter": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"infrastructure":  infrastructure,
					"cluster":         cluster,
					"gitops":          gitops,
					"managed-service": managedService,
					"services":        services,
				},
			},
			"opentofu": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
					"path":    map[string]any{"type": "string"},
					"backend": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type": map[string]any{"type": "string"},
							"local": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"path": map[string]any{"type": "string"},
								},
							},
							"s3": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"bucket":   map[string]any{"type": "string"},
									"key":      map[string]any{"type": "string"},
									"region":   map[string]any{"type": "string"},
									"endpoint": map[string]any{"type": "string"},
									"profile":  map[string]any{"type": "string"},
									"encrypt":  map[string]any{"type": "boolean"},
								},
							},
						},
					},
				},
			},
			"secrets": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sops_age_key_file": map[string]any{"type": "string"},
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
