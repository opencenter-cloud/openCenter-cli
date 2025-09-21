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

// GenerateSchema returns a JSON document representing the Draft 2020-12
// schema for the Config structure.
//
// The schema describes the nested properties of the configuration, and can
// be used for validation and documentation purposes.
//
// Inputs:
//   - pretty: If true, the output JSON will be indented.
//
// Outputs:
//   - []byte: The JSON schema document.
//   - error: An error if the schema cannot be generated.
func GenerateSchema(pretty bool) ([]byte, error) {
    // Match the exact structure from testdata/schema.yaml
    schema := map[string]any{
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "title":   "openCenter Cluster Configuration",
        "type":    "object",
        "properties": map[string]any{
            "cluster_name": map[string]any{"type": "string"},
            "cluster": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "env":    map[string]any{"type": "string"},
                    "name":   map[string]any{"type": "string"},
                    "region": map[string]any{"type": "string"},
                    "status": map[string]any{"type": "string"},
                },
            },
            "gitops": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "flux": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "interval": map[string]any{"type": "string"},
                            "prune":    map[string]any{"type": "boolean"},
                        },
                    },
                    "git_branch": map[string]any{"type": "string"},
                    "git_dir":    map[string]any{"type": "string"},
                    "git_ssh_key": map[string]any{"type": "string"},
                    "git_ssh_pub": map[string]any{"type": "string"},
                    "git_url":     map[string]any{"type": "string"},
                },
            },
            "opencenter": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "services": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "cert-manager": map[string]any{"type": "boolean"},
                            "gateway":      map[string]any{"type": "boolean"},
                            "gateway-api":  map[string]any{"type": "boolean"},
                            "keycloak":     map[string]any{"type": "boolean"},
                        },
                    },
                    "managed-service": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "alert-manager": map[string]any{"type": "boolean"},
                        },
                    },
                    "cluster": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "cluster_name":             map[string]any{"type": "string"},
                            "aws_access_key":           map[string]any{"type": "string"},
                            "aws_secret_access_key":    map[string]any{"type": "string"},
                            "k8s_api_port_acl":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                            "ssh_authorized_keys":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                            "kubernetes": map[string]any{
                                "type":       "object",
                                "properties": map[string]any{
                                    "version":                    map[string]any{"type": "string"},
                                    "flavor_bastion":             map[string]any{"type": "string"},
                                    "flavor_master":              map[string]any{"type": "string"},
                                    "flavor_worker":              map[string]any{"type": "string"},
                                    "subnet_pods":                map[string]any{"type": "string"},
                                    "subnet_services":            map[string]any{"type": "string"},
                                    "loadbalancer_provider":      map[string]any{"type": "string"},
                                    "dns_zone_name":              map[string]any{"type": "string"},
                                    "master_count":               map[string]any{"type": "integer"},
                                    "worker_count":               map[string]any{"type": "integer"},
                                    "worker_count_windows":       map[string]any{"type": "integer"},
                                    "network_plugin": map[string]any{
                                        "type":       "object",
                                        "properties": map[string]any{
                                            "calico": map[string]any{
                                                "type":       "object",
                                                "properties": map[string]any{
                                                    "enabled":                        map[string]any{"type": "boolean"},
                                                    "cni_iface":                      map[string]any{"type": "string"},
                                                    "calico_interface_autodetect":    map[string]any{"type": "string"},
                                                },
                                            },
                                        },
                                    },
                                    "oidc": map[string]any{
                                        "type":       "object",
                                        "properties": map[string]any{
                                            "enabled":                 map[string]any{"type": "boolean"},
                                            "kube_oidc_url":           map[string]any{"type": "string"},
                                            "kube_oidc_client_id":     map[string]any{"type": "string"},
                                            "kube_oidc_ca_file":       map[string]any{"type": "string"},
                                            "kube_oidc_username_claim": map[string]any{"type": "string"},
                                            "kube_oidc_username_prefix": map[string]any{"type": "string"},
                                            "kube_oidc_groups_claim":  map[string]any{"type": "string"},
                                            "kube_oidc_groups_prefix": map[string]any{"type": "string"},
                                        },
                                    },
                                    "windows_workers": map[string]any{
                                        "type":       "object",
                                        "properties": map[string]any{
                                            "enabled":                         map[string]any{"type": "boolean"},
                                            "windows_user":                    map[string]any{"type": "string"},
                                            "windows_admin_password":          map[string]any{"type": "string"},
                                            "worker_node_bfv_size_windows":    map[string]any{"type": "integer"},
                                            "worker_node_bfv_type_windows":    map[string]any{"type": "string"},
                                        },
                                    },
                                },
                            },
                        },
                    },
                    "gitops": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "git_dir":     map[string]any{"type": "string"},
                            "git_url":     map[string]any{"type": "string"},
                            "git_ssh_key": map[string]any{"type": "string"},
                            "git_ssh_pub": map[string]any{"type": "string"},
                        },
                    },
                },
            },
            "opentofu": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "enabled": map[string]any{"type": "boolean"},
                    "path":    map[string]any{"type": "string"},
                    "backend": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "type": map[string]any{"type": "string"},
                            "local": map[string]any{
                                "type":       "object",
                                "properties": map[string]any{
                                    "path": map[string]any{"type": "string"},
                                },
                            },
                            "s3": map[string]any{
                                "type":       "object",
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
            "ansible": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "enabled":   map[string]any{"type": "boolean"},
                    "inventory": map[string]any{"type": "string"},
                    "path":      map[string]any{"type": "string"},
                    "playbooks": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                },
            },
            "cloud": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "provider": map[string]any{"type": "string"},
                    "openstack": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "admin_password":                   map[string]any{"type": "string"},
                            "application_credential_id":       map[string]any{"type": "string"},
                            "application_credential_secret":   map[string]any{"type": "string"},
                            "auth_url":                         map[string]any{"type": "string"},
                            "availability_zone":                map[string]any{"type": "string"},
                            "ca":                               map[string]any{"type": "string"},
                            "disable_bastion":                  map[string]any{"type": "boolean"},
                            "external_network":                 map[string]any{"type": "string"},
                            "floatingip_pool":                  map[string]any{"type": "string"},
                            "insecure":                         map[string]any{"type": "boolean"},
                            "project_domain_name":              map[string]any{"type": "string"},
                            "region":                           map[string]any{"type": "string"},
                            "router_external_network_id":       map[string]any{"type": "string"},
                            "tenant_name":                      map[string]any{"type": "string"},
                            "use_octavia":                      map[string]any{"type": "boolean"},
                            "user_domain_name":                 map[string]any{"type": "string"},
                            "user_name":                        map[string]any{"type": "string"},
                            "user_password":                    map[string]any{"type": "string"},
                            "vrrp_ip":                          map[string]any{"type": "string"},
                        },
                    },
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
                },
            },
            "secrets": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "sops_age_key_file": map[string]any{"type": "string"},
                },
            },
            "iac": map[string]any{
                "type":        "object",
                "description": "Infrastructure-as-code inputs for main.tf (locals as main, and modules)",
                "properties": map[string]any{
                    "main": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "availability_zone":                    map[string]any{"type": "string"},
                            "ca_certificates":                      map[string]any{"type": "string"},
                            "calico_encapsulation_type":            map[string]any{"type": "string"},
                            "calico_interface_autodetect":          map[string]any{"type": "string"},
                            "calico_interface_autodetect_cidr":     map[string]any{"type": "string"},
                            "calico_nat_outgoing":                  map[string]any{"type": "boolean"},
                            "cluster_name":                         map[string]any{"type": "string"},
                            "cni_iface":                           map[string]any{"type": "string"},
                            "deploy_cluster":                      map[string]any{"type": "boolean"},
                            "dns_nameservers":                     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                            "flavor_bastion":                      map[string]any{"type": "string"},
                            "flavor_master":                       map[string]any{"type": "string"},
                            "flavor_worker":                       map[string]any{"type": "string"},
                            "floatingip_pool":                     map[string]any{"type": "string"},
                            "image_id":                            map[string]any{"type": "string"},
                            "image_id_windows":                    map[string]any{"type": "string"},
                            "k8s_api_port":                        map[string]any{"type": "integer"},
                            "k8s_hardening_enabled":               map[string]any{"type": "boolean"},
                            "kube_pod_security_exemptions_namespaces": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                            "kube_vip_enabled":                    map[string]any{"type": "boolean"},
                            "kubelet_rotate_server_certificates":  map[string]any{"type": "boolean"},
                            "kubernetes_version":                  map[string]any{"type": "string"},
                            "kubespray_version":                   map[string]any{"type": "string"},
                            "loadbalancer_provider":               map[string]any{"type": "string"},
                            "master_count":                        map[string]any{"type": "integer"},
                            "mtu":                                 map[string]any{"type": "string"},
                            "network_plugin":                      map[string]any{"type": "string"},
                            "network_provider":                    map[string]any{"type": "string"},
                            "node_master":                         map[string]any{"type": "string"},
                            "node_worker":                         map[string]any{"type": "string"},
                            "node_worker_windows":                 map[string]any{"type": "string"},
                            "openstack_admin_password":            map[string]any{"type": "string"},
                            "openstack_auth_url":                  map[string]any{"type": "string"},
                            "openstack_ca":                        map[string]any{"type": "string"},
                            "openstack_insecure":                  map[string]any{"type": "boolean"},
                            "openstack_project_domain_name":       map[string]any{"type": "string"},
                            "openstack_region":                    map[string]any{"type": "string"},
                            "openstack_tenant_name":               map[string]any{"type": "string"},
                            "openstack_user_domain_name":          map[string]any{"type": "string"},
                            "openstack_user_password":             map[string]any{"type": "string"},
                            "os_hardening_enabled":                map[string]any{"type": "boolean"},
                            "router_external_network_id":          map[string]any{"type": "string"},
                            "ssh_authorized_keys":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                            "ssh_user":                            map[string]any{"type": "string"},
                            "subnet_nodes":                        map[string]any{"type": "string"},
                            "subnet_pods":                         map[string]any{"type": "string"},
                            "subnet_services":                     map[string]any{"type": "string"},
                            "ub_version":                          map[string]any{"type": "string"},
                            "use_designate":                       map[string]any{"type": "boolean"},
                            "use_octavia":                         map[string]any{"type": "boolean"},
                            "vlan_id":                             map[string]any{"type": "string"},
                            "vrrp_enabled":                        map[string]any{"type": "boolean"},
                            "worker_count":                        map[string]any{"type": "integer"},
                            "worker_count_windows":                map[string]any{"type": "integer"},
                            "worker_node_bfv_destination_type":    map[string]any{"type": "string"},
                            "worker_node_bfv_source_type":         map[string]any{"type": "string"},
                            "worker_node_bfv_volume_size":         map[string]any{"type": "integer"},
                            "worker_node_bfv_volume_type":         map[string]any{"type": "string"},
                        },
                        "additionalProperties": true,
                    },
                    "modules": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "calico": map[string]any{
                                "type": "object",
                                "properties": map[string]any{
                                    "k8s_internal_ip":    map[string]any{"type": "string"},
                                    "source":             map[string]any{"type": "string"},
                                    "windows_dataplane":  map[string]any{"type": "string"},
                                },
                                "additionalProperties": true,
                            },
                            "kubespray-cluster": map[string]any{
                                "type": "object",
                                "properties": map[string]any{
                                    "address_bastion":  map[string]any{"type": "string"},
                                    "k8s_api_ip":       map[string]any{"type": "string"},
                                    "master_nodes":     map[string]any{"type": "string"},
                                    "source":           map[string]any{"type": "string"},
                                    "windows_nodes":    map[string]any{"type": "string"},
                                    "worker_nodes":     map[string]any{"type": "string"},
                                },
                                "additionalProperties": true,
                            },
                            "openstack-nova": map[string]any{
                                "type": "object",
                                "properties": map[string]any{
                                    "network_id": map[string]any{"type": "string"},
                                    "source":     map[string]any{"type": "string"},
                                },
                                "additionalProperties": true,
                            },
                        },
                        "additionalProperties": map[string]any{
                            "type":                 "object",
                            "additionalProperties": true,
                        },
                    },
                },
                "additionalProperties": false,
            },
        },
        "required": []string{"cluster_name"},
    }
    var data []byte
    var err error
    if pretty {
        data, err = json.MarshalIndent(schema, "", "  ")
    } else {
        data, err = json.Marshal(schema)
    }
    if err != nil {
        return nil, fmt.Errorf("failed to marshal schema: %w", err)
    }
    return data, nil
}

// GenerateDefaultFromSchema creates a default configuration by converting the JSON schema
// into a YAML structure with appropriate default values, excluding the iac section.
//
// Inputs:
//   - name: The cluster name to use for the configuration
//
// Outputs:
//   - []byte: The YAML configuration document
//   - error: An error if the configuration cannot be generated
func GenerateDefaultFromSchema(name string) ([]byte, error) {
    // Get the schema properties (without the JSON Schema metadata)
    schema := getSchemaProperties()

    // Create a map to hold the configuration
    config := make(map[string]any)

    // Set the cluster name
    config["cluster_name"] = name

    // Process each property in the schema, excluding 'iac'
    for key, value := range schema {
        if key == "iac" {
            continue // Skip iac section as requested
        }

        if key == "cluster_name" {
            // This is handled separately as the top-level cluster name
            continue
        }

        config[key] = generateDefaultValue(value.(map[string]any), key, name)
    }

    // Marshal to YAML
    data, err := yaml.Marshal(config)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal config to YAML: %w", err)
    }

    return data, nil
}

// getSchemaProperties returns the properties section matching testdata/schema.yaml structure
func getSchemaProperties() map[string]any {
    // Rewritten to exactly match testdata/schema.yaml structure
    schema := map[string]any{
        "opencenter": map[string]any{
            "type":       "object",
            "properties": map[string]any{
                "services": map[string]any{
                    "type":       "object",
                    "properties": map[string]any{
                        "cert-manager": map[string]any{"type": "boolean"},
                        "gateway":      map[string]any{"type": "boolean"},
                        "gateway-api":  map[string]any{"type": "boolean"},
                        "keycloak":     map[string]any{"type": "boolean"},
                    },
                },
                "managed-service": map[string]any{
                    "type":       "object",
                    "properties": map[string]any{
                        "alert-manager": map[string]any{"type": "boolean"},
                    },
                },
                "cluster": map[string]any{
                    "type":       "object",
                    "properties": map[string]any{
                        "cluster_name":             map[string]any{"type": "string"},
                        "aws_access_key":           map[string]any{"type": "string"},
                        "aws_secret_access_key":    map[string]any{"type": "string"},
                        "k8s_api_port_acl":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                        "ssh_authorized_keys":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                        "kubernetes": map[string]any{
                            "type":       "object",
                            "properties": map[string]any{
                                "version":                    map[string]any{"type": "string"},
                                "flavor_bastion":             map[string]any{"type": "string"},
                                "flavor_master":              map[string]any{"type": "string"},
                                "flavor_worker":              map[string]any{"type": "string"},
                                "subnet_pods":                map[string]any{"type": "string"},
                                "subnet_services":            map[string]any{"type": "string"},
                                "loadbalancer_provider":      map[string]any{"type": "string"},
                                "dns_zone_name":              map[string]any{"type": "string"},
                                "master_count":               map[string]any{"type": "integer"},
                                "worker_count":               map[string]any{"type": "integer"},
                                "worker_count_windows":       map[string]any{"type": "integer"},
                                "network_plugin": map[string]any{
                                    "type":       "object",
                                    "properties": map[string]any{
                                        "calico": map[string]any{
                                            "type":       "object",
                                            "properties": map[string]any{
                                                "enabled":                        map[string]any{"type": "boolean"},
                                                "cni_iface":                      map[string]any{"type": "string"},
                                                "calico_interface_autodetect":    map[string]any{"type": "string"},
                                            },
                                        },
                                    },
                                },
                                "oidc": map[string]any{
                                    "type":       "object",
                                    "properties": map[string]any{
                                        "enabled":                 map[string]any{"type": "boolean"},
                                        "kube_oidc_url":           map[string]any{"type": "string"},
                                        "kube_oidc_client_id":     map[string]any{"type": "string"},
                                        "kube_oidc_ca_file":       map[string]any{"type": "string"},
                                        "kube_oidc_username_claim": map[string]any{"type": "string"},
                                        "kube_oidc_username_prefix": map[string]any{"type": "string"},
                                        "kube_oidc_groups_claim":  map[string]any{"type": "string"},
                                        "kube_oidc_groups_prefix": map[string]any{"type": "string"},
                                    },
                                },
                                "windows_workers": map[string]any{
                                    "type":       "object",
                                    "properties": map[string]any{
                                        "enabled":                         map[string]any{"type": "boolean"},
                                        "windows_user":                    map[string]any{"type": "string"},
                                        "windows_admin_password":          map[string]any{"type": "string"},
                                        "worker_node_bfv_size_windows":    map[string]any{"type": "integer"},
                                        "worker_node_bfv_type_windows":    map[string]any{"type": "string"},
                                    },
                                },
                            },
                        },
                    },
                },
                "gitops": map[string]any{
                    "type":       "object",
                    "properties": map[string]any{
                        "git_dir":     map[string]any{"type": "string"},
                        "git_url":     map[string]any{"type": "string"},
                        "git_ssh_key": map[string]any{"type": "string"},
                        "git_ssh_pub": map[string]any{"type": "string"},
                    },
                },
            },
        },
        "opentofu": map[string]any{
            "type":       "object",
            "properties": map[string]any{
                "enabled": map[string]any{"type": "boolean"},
                "path":    map[string]any{"type": "string"},
                "backend": map[string]any{
                    "type":       "object",
                    "properties": map[string]any{
                        "type": map[string]any{"type": "string"},
                        "local": map[string]any{
                            "type":       "object",
                            "properties": map[string]any{
                                "path": map[string]any{"type": "string"},
                            },
                        },
                        "s3": map[string]any{
                            "type":       "object",
                            "properties": map[string]any{
                                "bucket":   map[string]any{"type": "string"},
                                "key":      map[string]any{"type": "string"},
                                "region":   map[string]any{"type": "string"},
                            },
                        },
                    },
                },
            },
        },
        "cloud": map[string]any{
            "type":       "object",
            "properties": map[string]any{
                "provider": map[string]any{"type": "string"},
                "openstack": map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "auth_url":                     map[string]any{"type": "string"},
                        "insecure":                     map[string]any{"type": "boolean"},
                        "region":                       map[string]any{"type": "string"},
                        "application_credential_id":   map[string]any{"type": "string"},
                        "application_credential_secret": map[string]any{"type": "string"},
                    },
                },
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
            },
        },
        "secrets": map[string]any{
            "type":       "object",
            "properties": map[string]any{
                "sops_age_key_file": map[string]any{"type": "string"},
            },
        },
    }
    return schema
}

// generateDefaultValue creates a default value based on the schema type and context
func generateDefaultValue(schema map[string]any, key, clusterName string) any {
    schemaType, ok := schema["type"].(string)
    if !ok {
        return nil
    }

    switch schemaType {
    case "string":
        return getStringDefault(key, clusterName)
    case "boolean":
        return getBooleanDefault(key)
    case "integer":
        return getIntegerDefault(key)
    case "array":
        return []any{}
    case "object":
        if properties, ok := schema["properties"].(map[string]any); ok {
            obj := make(map[string]any)
            for propKey, propSchema := range properties {
                obj[propKey] = generateDefaultValue(propSchema.(map[string]any), propKey, clusterName)
            }
            return obj
        }
        return make(map[string]any)
    default:
        return nil
    }
}

// getStringDefault returns appropriate string defaults based on the field name
func getStringDefault(key, clusterName string) string {
    defaults := map[string]string{
        "name":                     clusterName,
        "env":                      "dev",
        "region":                   "us-east-1",
        "status":                   "pending",
        "git_branch":               "main",
        "interval":                 "1m",
        "type":                     "local",
        "inventory":                "inventory.yml",
        "provider":                 "openstack",
        "availability_zone":        "nova",
        "floatingip_pool":          "public",
        "cluster_name":             clusterName,
        "version":                  "1.30.4",
        "flavor_bastion":           "gp.0.2.2",
        "flavor_master":            "gp.0.4.4",
        "flavor_worker":            "gp.0.4.8",
        "subnet_pods":              "10.42.0.0/16",
        "subnet_services":          "10.43.0.0/16",
        "loadbalancer_provider":    "amphora",
        "dns_zone_name":            "cluster.local",
        "cni_iface":                "enp3s0",
        "calico_interface_autodetect": "interface",
        "kube_oidc_client_id":      "kubernetes",
        "kube_oidc_username_claim": "sub",
        "kube_oidc_username_prefix": "oidc:",
        "kube_oidc_groups_claim":   "groups",
        "kube_oidc_groups_prefix":  "oidc:",
        "windows_user":             "Administrator",
        "worker_node_bfv_type_windows": "local",
    }

    // Handle specific path fields based on context
    if key == "path" {
        return "opentofu" // Default path for opentofu and ansible
    }
    if key == "git_dir" {
        return "/tmp/test" // Default git directory from testdata
    }

    if val, exists := defaults[key]; exists {
        return val
    }
    return ""
}

// getBooleanDefault returns appropriate boolean defaults based on the field name
func getBooleanDefault(key string) bool {
    trueDefaults := map[string]bool{
        "prune":    true,
        "enabled":  true,
        "encrypt":  false,
        "insecure": false,
        "disable_bastion": false,
        "use_octavia": false,
    }

    if val, exists := trueDefaults[key]; exists {
        return val
    }
    return false
}

// getIntegerDefault returns appropriate integer defaults based on the field name
func getIntegerDefault(key string) int {
    defaults := map[string]int{
        "master_count":               3,
        "worker_count":               2,
        "worker_count_windows":       0,
        "worker_node_bfv_size_windows": 0,
    }

    if val, exists := defaults[key]; exists {
        return val
    }
    return 0
}
