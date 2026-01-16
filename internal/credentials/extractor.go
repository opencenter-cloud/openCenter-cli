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

package credentials

import (
	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// Extractor extracts cloud provider credentials from cluster configuration
type Extractor struct {
	config config.Config
}

// NewExtractor creates a new credentials extractor
func NewExtractor(cfg config.Config) *Extractor {
	return &Extractor{
		config: cfg,
	}
}

// ExtractAWS extracts AWS credentials from the cluster configuration
func (e *Extractor) ExtractAWS() (*AWSCredentials, error) {
	creds := &AWSCredentials{}

	// Extract from infrastructure cloud configuration
	awsCloud := e.config.OpenCenter.Infrastructure.Cloud.AWS
	if awsCloud.Profile != "" {
		creds.Profile = awsCloud.Profile
	}
	if awsCloud.Region != "" {
		creds.Region = awsCloud.Region
	}
	if awsCloud.VPCID != "" {
		creds.VPCID = awsCloud.VPCID
	}
	creds.PrivateSubnets = awsCloud.PrivateSubnets
	creds.PublicSubnets = awsCloud.PublicSubnets

	// Extract from legacy cluster-level AWS credentials first (lower priority)
	if e.config.OpenCenter.Cluster.AWSAccessKey != "" {
		creds.AccessKeyID = e.config.OpenCenter.Cluster.AWSAccessKey
	}
	if e.config.OpenCenter.Cluster.AWSSecretAccessKey != "" {
		creds.SecretAccessKey = e.config.OpenCenter.Cluster.AWSSecretAccessKey
	}

	// Extract from global infrastructure AWS secrets (highest priority - overwrites cluster-level)
	infraSecrets := e.config.Secrets.Global.AWS.Infrastructure
	if infraSecrets.AccessKey != "" {
		creds.AccessKeyID = infraSecrets.AccessKey
	}
	if infraSecrets.SecretAccessKey != "" {
		creds.SecretAccessKey = infraSecrets.SecretAccessKey
	}
	if infraSecrets.Region != "" {
		creds.Region = infraSecrets.Region
	}

	return creds, nil
}

// ExtractOpenStack extracts OpenStack credentials from the cluster configuration
func (e *Extractor) ExtractOpenStack() (*OpenStackCredentials, error) {
	creds := &OpenStackCredentials{}

	// Extract from infrastructure cloud configuration
	osCloud := e.config.OpenCenter.Infrastructure.Cloud.OpenStack
	creds.AuthURL = osCloud.AuthURL
	creds.Region = osCloud.Region
	creds.ApplicationCredentialID = osCloud.ApplicationCredentialID
	creds.ApplicationCredentialSecret = osCloud.ApplicationCredentialSecret
	creds.Domain = osCloud.Domain
	creds.TenantName = osCloud.TenantName
	creds.FloatingNetworkID = osCloud.Networking.FloatingNetworkId
	creds.SubnetID = osCloud.Networking.SubnetId
	creds.Insecure = osCloud.Insecure

	return creds, nil
}
