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
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestExtractAWS(t *testing.T) {
	cfg := config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Infrastructure: config.Infrastructure{
				Cloud: config.CloudConfig{
					AWS: config.SimplifiedAWSCloud{
						Profile: "test-profile",
						Region:  "us-west-2",
						VPCID:   "vpc-12345",
					},
				},
			},
			Cluster: config.ClusterConfig{
				AWSAccessKey:       "AKIATEST123",
				AWSSecretAccessKey: "test-secret-key",
			},
		},
		Secrets: config.Secrets{
			AWS: config.AWSSecrets{
				AccessKey:       "AKIATEST456",
				SecretAccessKey: "test-secret-key-2",
				Region:          "us-east-1",
			},
		},
	}

	extractor := NewExtractor(cfg)
	creds, err := extractor.ExtractAWS()

	if err != nil {
		t.Fatalf("ExtractAWS failed: %v", err)
	}

	// Should prefer secrets over cluster-level credentials
	if creds.AccessKeyID != "AKIATEST456" {
		t.Errorf("Expected AccessKeyID 'AKIATEST456', got '%s'", creds.AccessKeyID)
	}

	if creds.SecretAccessKey != "test-secret-key-2" {
		t.Errorf("Expected SecretAccessKey 'test-secret-key-2', got '%s'", creds.SecretAccessKey)
	}

	// Should prefer region from secrets
	if creds.Region != "us-east-1" {
		t.Errorf("Expected Region 'us-east-1', got '%s'", creds.Region)
	}

	if creds.Profile != "test-profile" {
		t.Errorf("Expected Profile 'test-profile', got '%s'", creds.Profile)
	}

	if creds.VPCID != "vpc-12345" {
		t.Errorf("Expected VPCID 'vpc-12345', got '%s'", creds.VPCID)
	}
}

func TestExtractOpenStack(t *testing.T) {
	cfg := config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Infrastructure: config.Infrastructure{
				Cloud: config.CloudConfig{
					OpenStack: config.SimplifiedOpenStackCloud{
						AuthURL:                     "https://keystone.example.com/v3",
						Region:                      "RegionOne",
						ApplicationCredentialID:     "test-app-cred-id",
						ApplicationCredentialSecret: "test-app-cred-secret",
						Domain:                      "Default",
						TenantName:                  "test-tenant",
						FloatingNetworkId:           "net-12345",
						SubnetId:                    "subnet-67890",
						Insecure:                    true,
					},
				},
			},
		},
	}

	extractor := NewExtractor(cfg)
	creds, err := extractor.ExtractOpenStack()

	if err != nil {
		t.Fatalf("ExtractOpenStack failed: %v", err)
	}

	if creds.AuthURL != "https://keystone.example.com/v3" {
		t.Errorf("Expected AuthURL 'https://keystone.example.com/v3', got '%s'", creds.AuthURL)
	}

	if creds.Region != "RegionOne" {
		t.Errorf("Expected Region 'RegionOne', got '%s'", creds.Region)
	}

	if creds.ApplicationCredentialID != "test-app-cred-id" {
		t.Errorf("Expected ApplicationCredentialID 'test-app-cred-id', got '%s'", creds.ApplicationCredentialID)
	}

	if creds.ApplicationCredentialSecret != "test-app-cred-secret" {
		t.Errorf("Expected ApplicationCredentialSecret 'test-app-cred-secret', got '%s'", creds.ApplicationCredentialSecret)
	}

	if creds.Domain != "Default" {
		t.Errorf("Expected Domain 'Default', got '%s'", creds.Domain)
	}

	if creds.TenantName != "test-tenant" {
		t.Errorf("Expected TenantName 'test-tenant', got '%s'", creds.TenantName)
	}

	if creds.FloatingNetworkID != "net-12345" {
		t.Errorf("Expected FloatingNetworkID 'net-12345', got '%s'", creds.FloatingNetworkID)
	}

	if creds.SubnetID != "subnet-67890" {
		t.Errorf("Expected SubnetID 'subnet-67890', got '%s'", creds.SubnetID)
	}

	if !creds.Insecure {
		t.Errorf("Expected Insecure to be true, got false")
	}
}

func TestAWSCredentialsToEnvVars(t *testing.T) {
	creds := &AWSCredentials{
		AccessKeyID:     "AKIATEST123",
		SecretAccessKey: "test-secret-key",
		Region:          "us-west-2",
		Profile:         "test-profile",
		SessionToken:    "test-session-token",
	}

	envVars := creds.ToEnvVars()

	expectedVars := []string{
		"export AWS_ACCESS_KEY_ID=\"AKIATEST123\"",
		"export AWS_SECRET_ACCESS_KEY=\"test-secret-key\"",
		"export AWS_DEFAULT_REGION=\"us-west-2\"",
		"export AWS_PROFILE=\"test-profile\"",
		"export AWS_SESSION_TOKEN=\"test-session-token\"",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(envVars, expected) {
			t.Errorf("Expected env vars to contain '%s', got:\n%s", expected, envVars)
		}
	}
}

func TestOpenStackCredentialsToEnvVars(t *testing.T) {
	creds := &OpenStackCredentials{
		AuthURL:                     "https://keystone.example.com/v3",
		Region:                      "RegionOne",
		ApplicationCredentialID:     "test-app-cred-id",
		ApplicationCredentialSecret: "test-app-cred-secret",
		Domain:                      "Default",
	}

	envVars := creds.ToEnvVars()

	expectedVars := []string{
		"export OS_AUTH_URL=\"https://keystone.example.com/v3\"",
		"export OS_REGION_NAME=\"RegionOne\"",
		"export OS_APPLICATION_CREDENTIAL_ID=\"test-app-cred-id\"",
		"export OS_APPLICATION_CREDENTIAL_SECRET=\"test-app-cred-secret\"",
		"export OS_USER_DOMAIN_NAME=\"Default\"",
		"export OS_PROJECT_DOMAIN_NAME=\"Default\"",
		"export OS_INTERFACE=\"public\"",
		"export OS_IDENTITY_API_VERSION=\"3\"",
	}

	for _, expected := range expectedVars {
		if !strings.Contains(envVars, expected) {
			t.Errorf("Expected env vars to contain '%s', got:\n%s", expected, envVars)
		}
	}
}

func TestAWSCredentialsIsEmpty(t *testing.T) {
	// Empty credentials
	emptyCreds := &AWSCredentials{}
	if !emptyCreds.IsEmpty() {
		t.Error("Expected empty credentials to return true for IsEmpty()")
	}

	// Credentials with access key
	credsWithKey := &AWSCredentials{AccessKeyID: "AKIATEST123"}
	if credsWithKey.IsEmpty() {
		t.Error("Expected credentials with access key to return false for IsEmpty()")
	}

	// Credentials with profile
	credsWithProfile := &AWSCredentials{Profile: "test-profile"}
	if credsWithProfile.IsEmpty() {
		t.Error("Expected credentials with profile to return false for IsEmpty()")
	}
}

func TestOpenStackCredentialsIsEmpty(t *testing.T) {
	// Empty credentials
	emptyCreds := &OpenStackCredentials{}
	if !emptyCreds.IsEmpty() {
		t.Error("Expected empty credentials to return true for IsEmpty()")
	}

	// Credentials without auth URL
	credsNoAuth := &OpenStackCredentials{ApplicationCredentialID: "test-id"}
	if !credsNoAuth.IsEmpty() {
		t.Error("Expected credentials without auth URL to return true for IsEmpty()")
	}

	// Valid application credentials
	validAppCreds := &OpenStackCredentials{
		AuthURL:                     "https://keystone.example.com/v3",
		ApplicationCredentialID:     "test-id",
		ApplicationCredentialSecret: "test-secret",
	}
	if validAppCreds.IsEmpty() {
		t.Error("Expected valid application credentials to return false for IsEmpty()")
	}

	// Valid username/password credentials
	validUserCreds := &OpenStackCredentials{
		AuthURL:  "https://keystone.example.com/v3",
		Username: "test-user",
		Password: "test-password",
	}
	if validUserCreds.IsEmpty() {
		t.Error("Expected valid username/password credentials to return false for IsEmpty()")
	}
}