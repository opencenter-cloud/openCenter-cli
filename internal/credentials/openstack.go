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

// OpenStackCredentials represents OpenStack credentials and configuration
type OpenStackCredentials struct {
	AuthURL                     string `json:"auth_url,omitempty"`
	Region                      string `json:"region,omitempty"`
	ApplicationCredentialID     string `json:"application_credential_id,omitempty"`
	ApplicationCredentialSecret string `json:"application_credential_secret,omitempty"`
	Domain                      string `json:"domain,omitempty"`
	TenantName                  string `json:"tenant_name,omitempty"`
	FloatingNetworkID           string `json:"floating_network_id,omitempty"`
	SubnetID                    string `json:"subnet_id,omitempty"`
	Insecure                    bool   `json:"insecure,omitempty"`

	// Legacy username/password authentication (if available)
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	ProjectName       string `json:"project_name,omitempty"`
	UserDomainName    string `json:"user_domain_name,omitempty"`
	ProjectDomainName string `json:"project_domain_name,omitempty"`
}

// IsEmpty returns true if the credentials are empty or incomplete
func (c *OpenStackCredentials) IsEmpty() bool {
	// Consider credentials empty if we don't have auth URL or any authentication method
	if c.AuthURL == "" {
		return true
	}

	// Check for application credentials
	hasAppCreds := c.ApplicationCredentialID != "" && c.ApplicationCredentialSecret != ""

	// Check for username/password authentication
	hasUserPass := c.Username != "" && c.Password != ""

	return !hasAppCreds && !hasUserPass
}

// ToEnvVars converts OpenStack credentials to environment variable export statements
// Deprecated: Use ToEnvVarsForShell instead
func (c *OpenStackCredentials) ToEnvVars() string {
	return c.ToEnvVarsForShell("bash")
}

// ToEnvVarsForShell converts OpenStack credentials to shell-specific environment variable export statements
func (c *OpenStackCredentials) ToEnvVarsForShell(shell string) string {
	var output strings.Builder

	switch shell {
	case "fish":
		if c.AuthURL != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_AUTH_URL \"%s\"\n", c.AuthURL))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_REGION_NAME \"%s\"\n", c.Region))
		}
		if c.ApplicationCredentialID != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_APPLICATION_CREDENTIAL_ID \"%s\"\n", c.ApplicationCredentialID))
		}
		if c.ApplicationCredentialSecret != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_APPLICATION_CREDENTIAL_SECRET \"%s\"\n", c.ApplicationCredentialSecret))
		}
		if c.Username != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_USERNAME \"%s\"\n", c.Username))
		}
		if c.Password != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_PASSWORD \"%s\"\n", c.Password))
		}
		if c.ProjectName != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_PROJECT_NAME \"%s\"\n", c.ProjectName))
		}
		if c.TenantName != "" && c.ProjectName == "" {
			output.WriteString(fmt.Sprintf("set -gx OS_PROJECT_NAME \"%s\"\n", c.TenantName))
		}
		if c.UserDomainName != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_USER_DOMAIN_NAME \"%s\"\n", c.UserDomainName))
		}
		if c.ProjectDomainName != "" {
			output.WriteString(fmt.Sprintf("set -gx OS_PROJECT_DOMAIN_NAME \"%s\"\n", c.ProjectDomainName))
		}
		if c.Domain != "" && c.UserDomainName == "" && c.ProjectDomainName == "" {
			output.WriteString(fmt.Sprintf("set -gx OS_USER_DOMAIN_NAME \"%s\"\n", c.Domain))
			output.WriteString(fmt.Sprintf("set -gx OS_PROJECT_DOMAIN_NAME \"%s\"\n", c.Domain))
		}
		output.WriteString("set -gx OS_INTERFACE \"public\"\n")
		output.WriteString("set -gx OS_IDENTITY_API_VERSION \"3\"\n")

	case "powershell":
		if c.AuthURL != "" {
			output.WriteString(fmt.Sprintf("$env:OS_AUTH_URL = \"%s\"\n", c.AuthURL))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("$env:OS_REGION_NAME = \"%s\"\n", c.Region))
		}
		if c.ApplicationCredentialID != "" {
			output.WriteString(fmt.Sprintf("$env:OS_APPLICATION_CREDENTIAL_ID = \"%s\"\n", c.ApplicationCredentialID))
		}
		if c.ApplicationCredentialSecret != "" {
			output.WriteString(fmt.Sprintf("$env:OS_APPLICATION_CREDENTIAL_SECRET = \"%s\"\n", c.ApplicationCredentialSecret))
		}
		if c.Username != "" {
			output.WriteString(fmt.Sprintf("$env:OS_USERNAME = \"%s\"\n", c.Username))
		}
		if c.Password != "" {
			output.WriteString(fmt.Sprintf("$env:OS_PASSWORD = \"%s\"\n", c.Password))
		}
		if c.ProjectName != "" {
			output.WriteString(fmt.Sprintf("$env:OS_PROJECT_NAME = \"%s\"\n", c.ProjectName))
		}
		if c.TenantName != "" && c.ProjectName == "" {
			output.WriteString(fmt.Sprintf("$env:OS_PROJECT_NAME = \"%s\"\n", c.TenantName))
		}
		if c.UserDomainName != "" {
			output.WriteString(fmt.Sprintf("$env:OS_USER_DOMAIN_NAME = \"%s\"\n", c.UserDomainName))
		}
		if c.ProjectDomainName != "" {
			output.WriteString(fmt.Sprintf("$env:OS_PROJECT_DOMAIN_NAME = \"%s\"\n", c.ProjectDomainName))
		}
		if c.Domain != "" && c.UserDomainName == "" && c.ProjectDomainName == "" {
			output.WriteString(fmt.Sprintf("$env:OS_USER_DOMAIN_NAME = \"%s\"\n", c.Domain))
			output.WriteString(fmt.Sprintf("$env:OS_PROJECT_DOMAIN_NAME = \"%s\"\n", c.Domain))
		}
		output.WriteString("$env:OS_INTERFACE = \"public\"\n")
		output.WriteString("$env:OS_IDENTITY_API_VERSION = \"3\"\n")

	default:
		// Bash/Zsh syntax
		if c.AuthURL != "" {
			output.WriteString(fmt.Sprintf("export OS_AUTH_URL=\"%s\"\n", c.AuthURL))
		}
		if c.Region != "" {
			output.WriteString(fmt.Sprintf("export OS_REGION_NAME=\"%s\"\n", c.Region))
		}
		if c.ApplicationCredentialID != "" {
			output.WriteString(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_ID=\"%s\"\n", c.ApplicationCredentialID))
		}
		if c.ApplicationCredentialSecret != "" {
			output.WriteString(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_SECRET=\"%s\"\n", c.ApplicationCredentialSecret))
		}
		if c.Username != "" {
			output.WriteString(fmt.Sprintf("export OS_USERNAME=\"%s\"\n", c.Username))
		}
		if c.Password != "" {
			output.WriteString(fmt.Sprintf("export OS_PASSWORD=\"%s\"\n", c.Password))
		}
		if c.ProjectName != "" {
			output.WriteString(fmt.Sprintf("export OS_PROJECT_NAME=\"%s\"\n", c.ProjectName))
		}
		if c.TenantName != "" && c.ProjectName == "" {
			output.WriteString(fmt.Sprintf("export OS_PROJECT_NAME=\"%s\"\n", c.TenantName))
		}
		if c.UserDomainName != "" {
			output.WriteString(fmt.Sprintf("export OS_USER_DOMAIN_NAME=\"%s\"\n", c.UserDomainName))
		}
		if c.ProjectDomainName != "" {
			output.WriteString(fmt.Sprintf("export OS_PROJECT_DOMAIN_NAME=\"%s\"\n", c.ProjectDomainName))
		}
		if c.Domain != "" && c.UserDomainName == "" && c.ProjectDomainName == "" {
			output.WriteString(fmt.Sprintf("export OS_USER_DOMAIN_NAME=\"%s\"\n", c.Domain))
			output.WriteString(fmt.Sprintf("export OS_PROJECT_DOMAIN_NAME=\"%s\"\n", c.Domain))
		}
		output.WriteString("export OS_INTERFACE=\"public\"\n")
		output.WriteString("export OS_IDENTITY_API_VERSION=\"3\"\n")
	}

	return output.String()
}

// ToMap converts OpenStack credentials to a map for JSON serialization
func (c *OpenStackCredentials) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if c.AuthURL != "" {
		result["auth_url"] = c.AuthURL
	}
	if c.Region != "" {
		result["region"] = c.Region
	}
	if c.ApplicationCredentialID != "" {
		result["application_credential_id"] = c.ApplicationCredentialID
	}
	if c.ApplicationCredentialSecret != "" {
		result["application_credential_secret"] = c.ApplicationCredentialSecret
	}
	if c.Domain != "" {
		result["domain"] = c.Domain
	}
	if c.TenantName != "" {
		result["tenant_name"] = c.TenantName
	}
	if c.Username != "" {
		result["username"] = c.Username
	}
	if c.Password != "" {
		result["password"] = c.Password
	}
	if c.ProjectName != "" {
		result["project_name"] = c.ProjectName
	}
	if c.UserDomainName != "" {
		result["user_domain_name"] = c.UserDomainName
	}
	if c.ProjectDomainName != "" {
		result["project_domain_name"] = c.ProjectDomainName
	}
	if c.FloatingNetworkID != "" {
		result["floating_network_id"] = c.FloatingNetworkID
	}
	if c.SubnetID != "" {
		result["subnet_id"] = c.SubnetID
	}
	if c.Insecure {
		result["insecure"] = c.Insecure
	}

	return result
}

// ToTerraform converts OpenStack credentials to Terraform provider configuration
func (c *OpenStackCredentials) ToTerraform() string {
	var output strings.Builder

	output.WriteString("provider \"openstack\" {\n")

	if c.AuthURL != "" {
		output.WriteString(fmt.Sprintf("  auth_url    = \"%s\"\n", c.AuthURL))
	}
	if c.Region != "" {
		output.WriteString(fmt.Sprintf("  region      = \"%s\"\n", c.Region))
	}

	// Application credentials (preferred)
	if c.ApplicationCredentialID != "" {
		output.WriteString(fmt.Sprintf("  application_credential_id     = \"%s\"\n", c.ApplicationCredentialID))
	}
	if c.ApplicationCredentialSecret != "" {
		output.WriteString(fmt.Sprintf("  application_credential_secret = \"%s\"\n", c.ApplicationCredentialSecret))
	}

	// Username/password authentication (fallback)
	if c.Username != "" {
		output.WriteString(fmt.Sprintf("  user_name   = \"%s\"\n", c.Username))
	}
	if c.Password != "" {
		output.WriteString(fmt.Sprintf("  password    = \"%s\"\n", c.Password))
	}
	if c.TenantName != "" {
		output.WriteString(fmt.Sprintf("  tenant_name = \"%s\"\n", c.TenantName))
	}
	if c.Domain != "" {
		output.WriteString(fmt.Sprintf("  domain_name = \"%s\"\n", c.Domain))
	}

	if c.Insecure {
		output.WriteString("  insecure    = true\n")
	}

	output.WriteString("}")

	return output.String()
}

// ToCloudsYAML converts OpenStack credentials to clouds.yaml format
func (c *OpenStackCredentials) ToCloudsYAML() string {
	var output strings.Builder

	output.WriteString("clouds:\n")
	output.WriteString("  openstack:\n")
	output.WriteString("    auth:\n")

	if c.AuthURL != "" {
		output.WriteString(fmt.Sprintf("      auth_url: \"%s\"\n", c.AuthURL))
	}

	// Application credentials (preferred)
	if c.ApplicationCredentialID != "" {
		output.WriteString(fmt.Sprintf("      application_credential_id: \"%s\"\n", c.ApplicationCredentialID))
	}
	if c.ApplicationCredentialSecret != "" {
		output.WriteString(fmt.Sprintf("      application_credential_secret: \"%s\"\n", c.ApplicationCredentialSecret))
	}

	// Username/password authentication (fallback)
	if c.Username != "" {
		output.WriteString(fmt.Sprintf("      username: \"%s\"\n", c.Username))
	}
	if c.Password != "" {
		output.WriteString(fmt.Sprintf("      password: \"%s\"\n", c.Password))
	}
	if c.ProjectName != "" {
		output.WriteString(fmt.Sprintf("      project_name: \"%s\"\n", c.ProjectName))
	} else if c.TenantName != "" {
		output.WriteString(fmt.Sprintf("      project_name: \"%s\"\n", c.TenantName))
	}
	if c.UserDomainName != "" {
		output.WriteString(fmt.Sprintf("      user_domain_name: \"%s\"\n", c.UserDomainName))
	} else if c.Domain != "" {
		output.WriteString(fmt.Sprintf("      user_domain_name: \"%s\"\n", c.Domain))
	}
	if c.ProjectDomainName != "" {
		output.WriteString(fmt.Sprintf("      project_domain_name: \"%s\"\n", c.ProjectDomainName))
	} else if c.Domain != "" {
		output.WriteString(fmt.Sprintf("      project_domain_name: \"%s\"\n", c.Domain))
	}

	if c.Region != "" {
		output.WriteString(fmt.Sprintf("    region_name: \"%s\"\n", c.Region))
	}

	output.WriteString("    interface: public\n")
	output.WriteString("    identity_api_version: 3\n")

	if c.Insecure {
		output.WriteString("    verify: false\n")
	}

	return output.String()
}

// GetOpenStackEnvVars returns the list of OpenStack environment variables
func GetOpenStackEnvVars() []string {
	return []string{
		"OS_AUTH_URL",
		"OS_USERNAME",
		"OS_PASSWORD",
		"OS_PROJECT_NAME",
		"OS_USER_DOMAIN_NAME",
		"OS_PROJECT_DOMAIN_NAME",
		"OS_APPLICATION_CREDENTIAL_ID",
		"OS_APPLICATION_CREDENTIAL_SECRET",
		"OS_REGION_NAME",
		"OS_INTERFACE",
		"OS_IDENTITY_API_VERSION",
	}
}
