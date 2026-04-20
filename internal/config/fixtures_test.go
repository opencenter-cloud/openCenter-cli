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

// testModeConfig returns a Config pre-populated with test values.
// This replaces the test mode logic that was previously in defaultConfig().
//
// Use this function in tests that need a config with populated test credentials
// and infrastructure settings.
func testModeConfig(name string) Config {
	cfg := NewDefault(name)

	// Populate test infrastructure credentials
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "admin"
	cfg.OpenCenter.Secrets.Barbican.AuthURL = "https://identity.example.com/v3"

	// Populate infrastructure-level AWS credentials for OpenTofu S3 backend tests
	cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "test-aws-access-key"
	cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "test-aws-secret-key"

	return cfg
}

// minimalConfig returns a minimal Config with only required fields populated.
// Use this for tests that don't need full configuration.
func minimalConfig(name string) Config {
	cfg := Config{
		SchemaVersion: SchemaVersion,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         name,
				Organization: "opencenter",
			},
			Cluster: ClusterConfig{
				ClusterName: name,
			},
			GitOps: GitOpsConfig{
				Repository: GitOpsRepository{
					LocalDir: "./testdata/test-git-repo-" + name,
				},
			},
		},
		OpenTofu: SimplifiedOpenTofu{
			Enabled: true,
		},
		Secrets: Secrets{
			SSHKey: SSHKey{
				Private: "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name,
				Public:  "./testdata/test-git-repo-" + name + "/" + name + "/secrets/ssh/" + name + ".pub",
			},
		},
		Metadata: NewConfigMetadata(),
	}

	return cfg
}

// openstackConfig returns a Config with OpenStack provider settings populated.
// Use this for tests that need OpenStack-specific configuration.
func openstackConfig(name string) Config {
	cfg := NewDefault(name)

	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "test-app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "test-app-cred-secret"

	return cfg
}

// awsConfig returns a Config with AWS provider settings populated.
// Use this for tests that need AWS-specific configuration.
func awsConfig(name string) Config {
	cfg := NewDefault(name)

	cfg.OpenCenter.Infrastructure.Provider = "aws"
	cfg.OpenCenter.Infrastructure.Cloud.AWS.Profile = "test-profile"
	cfg.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
	cfg.OpenCenter.Infrastructure.Cloud.AWS.VPCID = "vpc-12345678"
	cfg.OpenCenter.Infrastructure.Cloud.AWS.PrivateSubnets = []string{"subnet-1", "subnet-2"}
	cfg.OpenCenter.Infrastructure.Cloud.AWS.PublicSubnets = []string{"subnet-3", "subnet-4"}

	return cfg
}

// s3BackendConfig returns a Config with S3 backend configured.
// Use this for tests that need S3 backend configuration.
func s3BackendConfig(name string) Config {
	cfg := NewDefault(name)

	cfg.OpenTofu.Backend.Type = "s3"
	cfg.OpenTofu.Backend.S3.Bucket = "test-bucket"
	cfg.OpenTofu.Backend.S3.Key = "terraform.tfstate"
	cfg.OpenTofu.Backend.S3.Region = "us-east-1"

	// Populate AWS credentials for S3 backend
	cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "test-aws-access-key"
	cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "test-aws-secret-key"

	return cfg
}
