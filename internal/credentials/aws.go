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
	"fmt"
	"strings"
)

// AWSCredentials represents AWS credentials and configuration
type AWSCredentials struct {
	AccessKeyID     string   `json:"access_key_id,omitempty"`
	SecretAccessKey string   `json:"secret_access_key,omitempty"`
	Region          string   `json:"region,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	SessionToken    string   `json:"session_token,omitempty"`
	VPCID           string   `json:"vpc_id,omitempty"`
	PrivateSubnets  []string `json:"private_subnets,omitempty"`
	PublicSubnets   []string `json:"public_subnets,omitempty"`
}

// IsEmpty returns true if the credentials are empty or incomplete
func (c *AWSCredentials) IsEmpty() bool {
	// Consider credentials empty if we don't have access key or profile
	return c.AccessKeyID == "" && c.Profile == ""
}

// ToEnvVars converts AWS credentials to environment variable export statements
// Deprecated: Use ToEnvVarsForShell instead
func (c *AWSCredentials) ToEnvVars() string {
	return c.ToEnvVarsForShell("bash")
}

// ToEnvVarsForShell converts AWS credentials to shell-specific environment variable export statements
func (c *AWSCredentials) ToEnvVarsForShell(shell string) string {
	var output strings.Builder

	switch shell {
	case "fish":
		if c.AccessKeyID != "" {
			output.WriteString(fmt.Sprintf("set -gx AWS_ACCESS_KEY_ID \"%s\"\n", c.AccessKeyID))
		}
		if c.SecretAccessKey != "" {
			output.WriteString(fmt.Sprintf("set -gx AWS_SECRET_ACCESS_KEY \"%s\"\n", c.SecretAccessKey))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("set -gx AWS_DEFAULT_REGION \"%s\"\n", c.Region))
		}
		if c.Profile != "" {
			output.WriteString(fmt.Sprintf("set -gx AWS_PROFILE \"%s\"\n", c.Profile))
		}
		if c.SessionToken != "" {
			output.WriteString(fmt.Sprintf("set -gx AWS_SESSION_TOKEN \"%s\"\n", c.SessionToken))
		}

	case "powershell":
		if c.AccessKeyID != "" {
			output.WriteString(fmt.Sprintf("$env:AWS_ACCESS_KEY_ID = \"%s\"\n", c.AccessKeyID))
		}
		if c.SecretAccessKey != "" {
			output.WriteString(fmt.Sprintf("$env:AWS_SECRET_ACCESS_KEY = \"%s\"\n", c.SecretAccessKey))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("$env:AWS_DEFAULT_REGION = \"%s\"\n", c.Region))
		}
		if c.Profile != "" {
			output.WriteString(fmt.Sprintf("$env:AWS_PROFILE = \"%s\"\n", c.Profile))
		}
		if c.SessionToken != "" {
			output.WriteString(fmt.Sprintf("$env:AWS_SESSION_TOKEN = \"%s\"\n", c.SessionToken))
		}

	default:
		// Bash/Zsh syntax
		if c.AccessKeyID != "" {
			output.WriteString(fmt.Sprintf("export AWS_ACCESS_KEY_ID=\"%s\"\n", c.AccessKeyID))
		}
		if c.SecretAccessKey != "" {
			output.WriteString(fmt.Sprintf("export AWS_SECRET_ACCESS_KEY=\"%s\"\n", c.SecretAccessKey))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("export AWS_DEFAULT_REGION=\"%s\"\n", c.Region))
		}
		if c.Profile != "" {
			output.WriteString(fmt.Sprintf("export AWS_PROFILE=\"%s\"\n", c.Profile))
		}
		if c.SessionToken != "" {
			output.WriteString(fmt.Sprintf("export AWS_SESSION_TOKEN=\"%s\"\n", c.SessionToken))
		}
	}

	return output.String()
}

// ToMap converts AWS credentials to a map for JSON serialization
func (c *AWSCredentials) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if c.AccessKeyID != "" {
		result["access_key_id"] = c.AccessKeyID
	}
	if c.SecretAccessKey != "" {
		result["secret_access_key"] = c.SecretAccessKey
	}
	if c.Region != "" {
		result["region"] = c.Region
	}
	if c.Profile != "" {
		result["profile"] = c.Profile
	}
	if c.SessionToken != "" {
		result["session_token"] = c.SessionToken
	}
	if c.VPCID != "" {
		result["vpc_id"] = c.VPCID
	}
	if len(c.PrivateSubnets) > 0 {
		result["private_subnets"] = c.PrivateSubnets
	}
	if len(c.PublicSubnets) > 0 {
		result["public_subnets"] = c.PublicSubnets
	}

	return result
}

// ToTerraform converts AWS credentials to Terraform provider configuration
func (c *AWSCredentials) ToTerraform() string {
	var output strings.Builder

	output.WriteString("provider \"aws\" {\n")

	if c.AccessKeyID != "" {
		output.WriteString(fmt.Sprintf("  access_key = \"%s\"\n", c.AccessKeyID))
	}
	if c.SecretAccessKey != "" {
		output.WriteString(fmt.Sprintf("  secret_key = \"%s\"\n", c.SecretAccessKey))
	}
	if c.Region != "" {
		output.WriteString(fmt.Sprintf("  region     = \"%s\"\n", c.Region))
	}
	if c.Profile != "" {
		output.WriteString(fmt.Sprintf("  profile    = \"%s\"\n", c.Profile))
	}
	if c.SessionToken != "" {
		output.WriteString(fmt.Sprintf("  token      = \"%s\"\n", c.SessionToken))
	}

	output.WriteString("}")

	return output.String()
}

// GetAWSEnvVars returns the list of AWS environment variables
func GetAWSEnvVars() []string {
	return []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_DEFAULT_REGION",
		"AWS_PROFILE",
		"AWS_SESSION_TOKEN",
	}
}
