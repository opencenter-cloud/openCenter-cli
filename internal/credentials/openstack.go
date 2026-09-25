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

// ToEnvVarsForShell converts OpenStack credentials to shell-specific environment variable export statements
func (c *OpenStackCredentials) ToEnvVarsForShell(shell string) string {
	var output strings.Builder

	for _, env := range c.shellEnvVars() {
		switch shell {
		case "fish":
			fmt.Fprintf(&output, "set -gx %s \"%s\"\n", env.name, env.value)
		case "powershell":
			fmt.Fprintf(&output, "$env:%s = \"%s\"\n", env.name, env.value)
		default:
			fmt.Fprintf(&output, "export %s=\"%s\"\n", env.name, env.value)
		}
	}

	return output.String()
}

type envVar struct {
	name  string
	value string
}

func (c *OpenStackCredentials) shellEnvVars() []envVar {
	env := make([]envVar, 0, 13)
	add := func(name, value string) {
		if value != "" {
			env = append(env, envVar{name: name, value: value})
		}
	}

	add("OS_AUTH_URL", c.AuthURL)
	add("OS_REGION_NAME", c.Region)
	add("OS_APPLICATION_CREDENTIAL_ID", c.ApplicationCredentialID)
	add("OS_APPLICATION_CREDENTIAL_SECRET", c.ApplicationCredentialSecret)
	add("OS_USERNAME", c.Username)
	add("OS_PASSWORD", c.Password)
	add("OS_PROJECT_NAME", firstNonEmpty(c.ProjectName, c.TenantName))
	add("OS_USER_DOMAIN_NAME", c.UserDomainName)
	add("OS_PROJECT_DOMAIN_NAME", c.ProjectDomainName)
	if c.Domain != "" && c.UserDomainName == "" && c.ProjectDomainName == "" {
		add("OS_USER_DOMAIN_NAME", c.Domain)
		add("OS_PROJECT_DOMAIN_NAME", c.Domain)
	}
	add("OS_INTERFACE", "public")
	add("OS_IDENTITY_API_VERSION", "3")

	return env
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ToEnvMap converts OpenStack credentials to process environment variables.
func (c *OpenStackCredentials) ToEnvMap() map[string]string {
	result := map[string]string{
		"OS_INTERFACE":            "public",
		"OS_IDENTITY_API_VERSION": "3",
	}

	if c.AuthURL != "" {
		result["OS_AUTH_URL"] = c.AuthURL
	}
	if c.Insecure {
		result["OS_INSECURE"] = "true"
	}
	if c.Region != "" {
		result["OS_REGION_NAME"] = c.Region
	}
	if c.ApplicationCredentialID != "" {
		result["OS_APPLICATION_CREDENTIAL_ID"] = c.ApplicationCredentialID
	}
	if c.ApplicationCredentialSecret != "" {
		result["OS_APPLICATION_CREDENTIAL_SECRET"] = c.ApplicationCredentialSecret
	}
	if c.Username != "" {
		result["OS_USERNAME"] = c.Username
	}
	if c.Password != "" {
		result["OS_PASSWORD"] = c.Password
	}
	if c.ProjectName != "" {
		result["OS_PROJECT_NAME"] = c.ProjectName
	} else if c.TenantName != "" {
		result["OS_PROJECT_NAME"] = c.TenantName
	}
	if c.UserDomainName != "" {
		result["OS_USER_DOMAIN_NAME"] = c.UserDomainName
	}
	if c.ProjectDomainName != "" {
		result["OS_PROJECT_DOMAIN_NAME"] = c.ProjectDomainName
	}
	if c.Domain != "" {
		if _, ok := result["OS_USER_DOMAIN_NAME"]; !ok {
			result["OS_USER_DOMAIN_NAME"] = c.Domain
		}
		if _, ok := result["OS_PROJECT_DOMAIN_NAME"]; !ok {
			result["OS_PROJECT_DOMAIN_NAME"] = c.Domain
		}
	}

	return result
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
		fmt.Fprintf(&output, "  auth_url    = \"%s\"\n", c.AuthURL)
	}
	if c.Region != "" {
		fmt.Fprintf(&output, "  region      = \"%s\"\n", c.Region)
	}

	// Application credentials (preferred)
	if c.ApplicationCredentialID != "" {
		fmt.Fprintf(&output, "  application_credential_id     = \"%s\"\n", c.ApplicationCredentialID)
	}
	if c.ApplicationCredentialSecret != "" {
		fmt.Fprintf(&output, "  application_credential_secret = \"%s\"\n", c.ApplicationCredentialSecret)
	}

	// Username/password authentication (fallback)
	if c.Username != "" {
		fmt.Fprintf(&output, "  user_name   = \"%s\"\n", c.Username)
	}
	if c.Password != "" {
		fmt.Fprintf(&output, "  password    = \"%s\"\n", c.Password)
	}
	if c.TenantName != "" {
		fmt.Fprintf(&output, "  tenant_name = \"%s\"\n", c.TenantName)
	}
	if c.Domain != "" {
		fmt.Fprintf(&output, "  domain_name = \"%s\"\n", c.Domain)
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
		fmt.Fprintf(&output, "      auth_url: \"%s\"\n", c.AuthURL)
	}

	// Application credentials (preferred)
	if c.ApplicationCredentialID != "" {
		fmt.Fprintf(&output, "      application_credential_id: \"%s\"\n", c.ApplicationCredentialID)
	}
	if c.ApplicationCredentialSecret != "" {
		fmt.Fprintf(&output, "      application_credential_secret: \"%s\"\n", c.ApplicationCredentialSecret)
	}

	// Username/password authentication (fallback)
	if c.Username != "" {
		fmt.Fprintf(&output, "      username: \"%s\"\n", c.Username)
	}
	if c.Password != "" {
		fmt.Fprintf(&output, "      password: \"%s\"\n", c.Password)
	}
	if c.ProjectName != "" {
		fmt.Fprintf(&output, "      project_name: \"%s\"\n", c.ProjectName)
	} else if c.TenantName != "" {
		fmt.Fprintf(&output, "      project_name: \"%s\"\n", c.TenantName)
	}
	if c.UserDomainName != "" {
		fmt.Fprintf(&output, "      user_domain_name: \"%s\"\n", c.UserDomainName)
	} else if c.Domain != "" {
		fmt.Fprintf(&output, "      user_domain_name: \"%s\"\n", c.Domain)
	}
	if c.ProjectDomainName != "" {
		fmt.Fprintf(&output, "      project_domain_name: \"%s\"\n", c.ProjectDomainName)
	} else if c.Domain != "" {
		fmt.Fprintf(&output, "      project_domain_name: \"%s\"\n", c.Domain)
	}

	if c.Region != "" {
		fmt.Fprintf(&output, "    region_name: \"%s\"\n", c.Region)
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
