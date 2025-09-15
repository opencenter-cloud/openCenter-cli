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
    schema := map[string]any{
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "title":   "openCenter Cluster Configuration",
        "type":    "object",
        "properties": map[string]any{
            // Legacy top-levels retained
            "cluster_name": map[string]any{"type": "string"},
            "naming_prefix": map[string]any{"type": "string"},

            // New: cluster metadata (optional)
            "cluster": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "name":   map[string]any{"type": "string"},
                    "env":    map[string]any{"type": "string"},
                    "region": map[string]any{"type": "string"},
                    "status": map[string]any{"type": "string"},
                },
            },
            "gitops": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "git_dir":    map[string]any{"type": "string"},
                    "git_url":    map[string]any{"type": "string"},
                    "git_ssh_key": map[string]any{"type": "string"},
                    // New fields
                    "git_branch": map[string]any{"type": "string"},
                    "flux": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "interval": map[string]any{"type": "string"},
                            "prune":    map[string]any{"type": "boolean"},
                        },
                    },
                },
            },
            // New: opencenter meta for credentials used by OpenTofu
            "opencenter": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "aws_access_key":        map[string]any{"type": "string", "description": "AWS access key ID used by OpenTofu S3 backend", "writeOnly": true},
                    "aws_secret_access_key": map[string]any{"type": "string", "description": "AWS secret access key used by OpenTofu S3 backend", "writeOnly": true},
                },
            },
            // New: opentofu configuration
            "opentofu": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "enabled": map[string]any{"type": "boolean"},
                    "path":    map[string]any{"type": "string"},
                    "backend": map[string]any{
                        "type":       "object",
                        "properties": map[string]any{
                            "type": map[string]any{"type": "string", "enum": []any{"local", "s3"}},
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
                    "enabled": map[string]any{"type": "boolean"},
                    "path":    map[string]any{"type": "string"},
                    // New fields
                    "inventory": map[string]any{"type": "string"},
                    "playbooks": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
                },
            },
            // iac: structured fields for Terraform locals (main) and modules
            "iac": map[string]any{
                "type":        "object",
                "description": "Infrastructure-as-code inputs for main.tf (locals as main, and modules)",
                "properties": map[string]any{
                    "main": map[string]any{
                        "type": "object",
                        // Allow arbitrary keys corresponding to Terraform locals
                        "additionalProperties": true,
                    },
                    "modules": map[string]any{
                        "type": "object",
                        // Each module is an object with arbitrary attributes
                        "additionalProperties": map[string]any{
                            "type":                 "object",
                            "additionalProperties": true,
                        },
                    },
                    // Deprecated field retained for backwards compatibility
                    "main_tf": map[string]any{"type": "string"},
                },
                "additionalProperties": false,
            },
            "cloud": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "provider": map[string]any{"type": "string"},
                    "openstack": map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "auth_url":          map[string]any{"type": "string"},
                            "insecure":          map[string]any{"type": "boolean"},
                            "region":            map[string]any{"type": "string"},
                            "user_name":         map[string]any{"type": "string"},
                            "user_password":     map[string]any{"type": "string"},
                            "admin_password":    map[string]any{"type": "string"},
                            "project_domain_name": map[string]any{"type": "string"},
                            "user_domain_name":    map[string]any{"type": "string"},
                            "tenant_name":        map[string]any{"type": "string"},
                            "availability_zone":  map[string]any{"type": "string"},
                            "floatingip_pool":     map[string]any{"type": "string"},
                            "router_external_network_id": map[string]any{"type": "string"},
                            "disable_bastion":     map[string]any{"type": "boolean"},
                            "ca":                  map[string]any{"type": "string"},
                            // New convenience aliases
                            "external_network":    map[string]any{"type": "string"},
                            "use_octavia":         map[string]any{"type": "boolean"},
                            "vrrp_ip":             map[string]any{"type": "string"},
                        },
                    },
                    // New: AWS provider shape (optional)
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
            // New: secrets configuration
            "secrets": map[string]any{
                "type":       "object",
                "properties": map[string]any{
                    "sops_age_key_file": map[string]any{"type": "string"},
                },
            },
        },
        "required": []string{"cluster_name", "gitops.git_dir"},
        // Conditional requirements
        "allOf": []any{
            map[string]any{
                "if": map[string]any{
                    "properties": map[string]any{
                        "opentofu": map[string]any{
                            "type": "object",
                            "properties": map[string]any{
                                "backend": map[string]any{
                                    "type": "object",
                                    "properties": map[string]any{
                                        "type": map[string]any{"const": "s3"},
                                    },
                                    "required": []string{"type"},
                                },
                            },
                            "required": []string{"backend"},
                        },
                    },
                    "required": []string{"opentofu"},
                },
                "then": map[string]any{
                    "required": []string{"opencenter"},
                    "properties": map[string]any{
                        "opencenter": map[string]any{
                            "type": "object",
                            "required": []string{"aws_access_key", "aws_secret_access_key"},
                        },
                    },
                },
            },
        },
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
