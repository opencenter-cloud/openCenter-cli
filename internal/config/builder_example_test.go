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

package config_test

import (
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// ExampleConfigBuilder demonstrates the fluent API for building cluster configurations.
func ExampleConfigBuilder() {
	// Create a new configuration using the fluent builder API
	builder := config.NewConfigBuilder("production-cluster").
		WithOrganization("acme-corp").
		WithProvider("openstack").
		WithRegion("us-east-1").
		WithEnvironment("production").
		WithKubernetesVersion("1.33.7").
		WithNodeCounts(3, 5).
		WithSubnetNodes("10.0.0.0/24").
		WithSubnetPods("10.42.0.0/16").
		WithSubnetServices("10.43.0.0/16").
		WithSSHUser("ubuntu").
		WithBaseDomain("acme.com").
		WithAdminEmail("admin@acme.com").
		WithK8sHardening(true).
		WithOSHardening(true).
		WithTag("environment", "production").
		WithTag("team", "platform").
		WithAnnotation("created-by", "platform-team").
		WithOpenStackConfig(config.SimplifiedOpenStackCloud{
			AuthURL:    "https://identity.example.com/v3",
			Region:     "us-east-1",
			TenantName: "acme-corp",
		})

	// Build the configuration
	cfg, err := builder.Build()
	if err != nil {
		fmt.Printf("Error building configuration: %v\n", err)
		return
	}

	// Use the configuration
	fmt.Printf("Cluster: %s\n", cfg.OpenCenter.Meta.Name)
	fmt.Printf("Organization: %s\n", cfg.OpenCenter.Meta.Organization)
	fmt.Printf("Provider: %s\n", cfg.OpenCenter.Infrastructure.Provider)
	fmt.Printf("Masters: %d, Workers: %d\n", 
		cfg.OpenCenter.Cluster.Kubernetes.MasterCount,
		cfg.OpenCenter.Cluster.Kubernetes.WorkerCount)
	
	// Output:
	// Cluster: production-cluster
	// Organization: acme-corp
	// Provider: openstack
	// Masters: 3, Workers: 5
}

// ExampleConfigBuilder_aws demonstrates building an AWS cluster configuration.
func ExampleConfigBuilder_aws() {
	builder := config.NewConfigBuilder("aws-cluster").
		WithOrganization("acme-corp").
		WithProvider("aws").
		WithRegion("us-west-2").
		WithKubernetesVersion("1.33.7").
		WithNodeCounts(3, 3).
		WithAWSConfig(config.SimplifiedAWSCloud{
			Region:         "us-west-2",
			VPCID:          "vpc-12345678",
			PrivateSubnets: []string{"subnet-1", "subnet-2"},
			PublicSubnets:  []string{"subnet-3", "subnet-4"},
		})

	cfg, err := builder.Build()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("AWS Cluster: %s in %s\n", 
		cfg.OpenCenter.Meta.Name,
		cfg.OpenCenter.Infrastructure.Cloud.AWS.Region)
	
	// Output:
	// AWS Cluster: aws-cluster in us-west-2
}

// ExampleConfigBuilder_modifyExisting demonstrates modifying an existing configuration.
func ExampleConfigBuilder_modifyExisting() {
	// Load an existing configuration
	existingConfig := config.NewDefault("existing-cluster")
	existingConfig.OpenCenter.Meta.Organization = "acme-corp"
	existingConfig.OpenCenter.Infrastructure.Provider = "aws"
	existingConfig.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"

	// Create a builder from the existing config and modify it
	builder := config.NewConfigBuilderFromConfig(existingConfig).
		WithEnvironment("staging").
		WithWorkerCount(10).
		WithTag("modified", "true")

	cfg, err := builder.Build()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Modified cluster: %s with %d workers\n",
		cfg.OpenCenter.Meta.Name,
		cfg.OpenCenter.Cluster.Kubernetes.WorkerCount)
	
	// Output:
	// Modified cluster: existing-cluster with 10 workers
}
