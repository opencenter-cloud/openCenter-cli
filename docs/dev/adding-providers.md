---
doc_type: how-to
title: "Adding New Infrastructure Providers"
audience: "developers"
---

# Adding New Infrastructure Providers

**Purpose:** For developers, shows how to add support for new infrastructure providers (cloud platforms, bare metal, etc.).

## Prerequisites

Before adding a provider, you need:
- Development environment set up (see [Development Setup](development-setup.md))
- Understanding of the target provider's API
- Provider credentials for testing

## Provider Architecture

Providers in openCenter-cli consist of:

1. **Configuration defaults** - Default values for provider-specific settings
2. **Validation logic** - Provider-specific validation rules
3. **Preflight checks** - Connectivity and credential validation
4. **Templates** - Infrastructure-as-code templates (Terraform/Pulumi)
5. **Documentation** - Provider-specific guides

## Step 1: Add Provider Configuration

Add provider defaults to `internal/config/defaults.go`:

```go
// Add provider constant
const (
    ProviderOpenStack = "openstack"
    ProviderAWS       = "aws"
    ProviderVMware    = "vmware"
    ProviderMyCloud   = "mycloud"  // New provider
)

// Add to getProviderDefaults function
func getProviderDefaults(provider string) map[string]interface{} {
    switch provider {
    case ProviderMyCloud:
        return map[string]interface{}{
            "opencenter": map[string]interface{}{
                "infrastructure": map[string]interface{}{
                    "cloud": map[string]interface{}{
                        "mycloud": map[string]interface{}{
                            "api_endpoint": "https://api.mycloud.com",
                            "region":       "us-east-1",
                            "project_id":   "",
                            "api_key":      "",
                        },
                    },
                },
            },
        }
    // ... other providers
    }
}
```


## Step 2: Update Configuration Schema

Add provider schema to `internal/config/schema.go`:

```go
// Add to schema generation
func (Config) JSONSchema() *jsonschema.Schema {
    schema := &jsonschema.Schema{
        // ... existing schema
    }
    
    // Add mycloud provider schema
    mycloudSchema := &jsonschema.Schema{
        Type: "object",
        Properties: map[string]*jsonschema.Schema{
            "api_endpoint": {
                Type:        "string",
                Description: "MyCloud API endpoint URL",
                Format:      "uri",
            },
            "region": {
                Type:        "string",
                Description: "MyCloud region",
                Enum:        []interface{}{"us-east-1", "us-west-1", "eu-west-1"},
            },
            "project_id": {
                Type:        "string",
                Description: "MyCloud project ID",
            },
            "api_key": {
                Type:        "string",
                Description: "MyCloud API key (encrypted with SOPS)",
            },
        },
        Required: []string{"api_endpoint", "region", "project_id"},
    }
    
    // Add to cloud providers
    schema.Properties["opencenter"].Properties["infrastructure"].
        Properties["cloud"].Properties["mycloud"] = mycloudSchema
    
    return schema
}
```

## Step 3: Implement Preflight Checks

Create `internal/cloud/mycloud/preflight.go`:

```go
package mycloud

import (
    "fmt"
    "net/http"
    "time"
)

// Preflight validates MyCloud credentials and connectivity
func Preflight(config map[string]any) []string {
    var errors []string
    
    // Extract configuration
    apiEndpoint, ok := config["api_endpoint"].(string)
    if !ok || apiEndpoint == "" {
        errors = append(errors, "api_endpoint is required")
        return errors
    }
    
    region, ok := config["region"].(string)
    if !ok || region == "" {
        errors = append(errors, "region is required")
    }
    
    projectID, ok := config["project_id"].(string)
    if !ok || projectID == "" {
        errors = append(errors, "project_id is required")
    }
    
    apiKey, ok := config["api_key"].(string)
    if !ok || apiKey == "" {
        errors = append(errors, "api_key is required")
    }
    
    // Return early if required fields missing
    if len(errors) > 0 {
        return errors
    }
    
    // Test API connectivity
    client := &http.Client{Timeout: 10 * time.Second}
    req, err := http.NewRequest("GET", apiEndpoint+"/v1/health", nil)
    if err != nil {
        errors = append(errors, fmt.Sprintf("failed to create request: %v", err))
        return errors
    }
    
    req.Header.Set("Authorization", "Bearer "+apiKey)
    
    resp, err := client.Do(req)
    if err != nil {
        errors = append(errors, fmt.Sprintf("failed to connect to MyCloud API: %v", err))
        return errors
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 401 {
        errors = append(errors, "authentication failed: invalid API key")
    } else if resp.StatusCode != 200 {
        errors = append(errors, fmt.Sprintf("API returned status %d", resp.StatusCode))
    }
    
    // Validate region
    if err := validateRegion(client, apiEndpoint, apiKey, region); err != nil {
        errors = append(errors, fmt.Sprintf("invalid region: %v", err))
    }
    
    // Validate project access
    if err := validateProject(client, apiEndpoint, apiKey, projectID); err != nil {
        errors = append(errors, fmt.Sprintf("project access denied: %v", err))
    }
    
    return errors
}

func validateRegion(client *http.Client, endpoint, apiKey, region string) error {
    // Implementation to validate region exists
    return nil
}

func validateProject(client *http.Client, endpoint, apiKey, projectID string) error {
    // Implementation to validate project access
    return nil
}
```

## Step 4: Register Preflight Check

Update `cmd/cluster_preflight.go` to call your preflight function:

```go
func runPreflight(cfg *config.Config) error {
    provider := cfg.OpenCenter.Infrastructure.Provider
    
    var errors []string
    
    switch provider {
    case "mycloud":
        mycloudConfig := cfg.OpenCenter.Infrastructure.Cloud.MyCloud
        errors = mycloud.Preflight(mycloudConfig)
    case "openstack":
        // ... existing providers
    }
    
    if len(errors) > 0 {
        for _, err := range errors {
            fmt.Fprintf(os.Stderr, "❌ %s\n", err)
        }
        return fmt.Errorf("preflight checks failed")
    }
    
    return nil
}
```


## Step 5: Add Infrastructure Templates

Create Terraform templates in `internal/gitops/gitops-base-dir/infrastructure/clusters/mycloud/`:

```hcl
# main.tf.tmpl
terraform {
  required_providers {
    mycloud = {
      source  = "mycloud/mycloud"
      version = "~> 1.0"
    }
  }
}

provider "mycloud" {
  api_endpoint = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.APIEndpoint }}"
  region       = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.Region }}"
  project_id   = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.ProjectID }}"
  api_key      = var.mycloud_api_key
}

# Create VPC
resource "mycloud_vpc" "cluster" {
  name        = "{{ .OpenCenter.Meta.ClusterName }}-vpc"
  cidr_block  = "{{ .OpenCenter.Infrastructure.Network.NodeSubnet }}"
  region      = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.Region }}"
}

# Create control plane instances
resource "mycloud_instance" "control_plane" {
  count         = {{ .OpenCenter.Cluster.MasterCount }}
  name          = "{{ .OpenCenter.Meta.ClusterName }}-master-${count.index + 1}"
  instance_type = "{{ .OpenCenter.Cluster.MasterFlavor }}"
  image_id      = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.ImageID }}"
  vpc_id        = mycloud_vpc.cluster.id
  
  tags = {
    cluster = "{{ .OpenCenter.Meta.ClusterName }}"
    role    = "control-plane"
  }
}

# Create worker instances
resource "mycloud_instance" "worker" {
  count         = {{ .OpenCenter.Cluster.WorkerCount }}
  name          = "{{ .OpenCenter.Meta.ClusterName }}-worker-${count.index + 1}"
  instance_type = "{{ .OpenCenter.Cluster.WorkerFlavor }}"
  image_id      = "{{ .OpenCenter.Infrastructure.Cloud.MyCloud.ImageID }}"
  vpc_id        = mycloud_vpc.cluster.id
  
  tags = {
    cluster = "{{ .OpenCenter.Meta.ClusterName }}"
    role    = "worker"
  }
}
```

```hcl
# variables.tf.tmpl
variable "mycloud_api_key" {
  description = "MyCloud API key"
  type        = string
  sensitive   = true
}
```

```hcl
# outputs.tf.tmpl
output "control_plane_ips" {
  value = mycloud_instance.control_plane[*].private_ip
}

output "worker_ips" {
  value = mycloud_instance.worker[*].private_ip
}

output "vpc_id" {
  value = mycloud_vpc.cluster.id
}
```

## Step 6: Add Provider Validation

Create `internal/config/mycloud_validator.go`:

```go
package config

import "fmt"

// ValidateMyCloudConfig validates MyCloud-specific configuration
func ValidateMyCloudConfig(cfg *Config) []error {
    var errors []error
    
    mycloud := cfg.OpenCenter.Infrastructure.Cloud.MyCloud
    
    // Validate API endpoint
    if mycloud.APIEndpoint == "" {
        errors = append(errors, fmt.Errorf("mycloud.api_endpoint is required"))
    }
    
    // Validate region
    validRegions := []string{"us-east-1", "us-west-1", "eu-west-1"}
    if !contains(validRegions, mycloud.Region) {
        errors = append(errors, fmt.Errorf("invalid region: %s", mycloud.Region))
    }
    
    // Validate instance types
    if err := validateInstanceType(mycloud.MasterFlavor); err != nil {
        errors = append(errors, fmt.Errorf("invalid master flavor: %w", err))
    }
    
    if err := validateInstanceType(mycloud.WorkerFlavor); err != nil {
        errors = append(errors, fmt.Errorf("invalid worker flavor: %w", err))
    }
    
    return errors
}

func validateInstanceType(flavor string) error {
    validFlavors := []string{"small", "medium", "large", "xlarge"}
    if !contains(validFlavors, flavor) {
        return fmt.Errorf("invalid instance type: %s", flavor)
    }
    return nil
}
```

Register validator in `internal/config/validator.go`:

```go
func (v *Validator) Validate(cfg *Config) error {
    // ... existing validation
    
    // Provider-specific validation
    switch cfg.OpenCenter.Infrastructure.Provider {
    case "mycloud":
        if errs := ValidateMyCloudConfig(cfg); len(errs) > 0 {
            return fmt.Errorf("mycloud validation failed: %v", errs)
        }
    // ... other providers
    }
    
    return nil
}
```

## Step 7: Write Tests

Create `internal/cloud/mycloud/preflight_test.go`:

```go
package mycloud

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestPreflight_MissingAPIEndpoint(t *testing.T) {
    config := map[string]any{
        "region":     "us-east-1",
        "project_id": "test-project",
        "api_key":    "test-key",
    }
    
    errors := Preflight(config)
    
    assert.Len(t, errors, 1)
    assert.Contains(t, errors[0], "api_endpoint is required")
}

func TestPreflight_InvalidRegion(t *testing.T) {
    config := map[string]any{
        "api_endpoint": "https://api.mycloud.com",
        "region":       "invalid-region",
        "project_id":   "test-project",
        "api_key":      "test-key",
    }
    
    errors := Preflight(config)
    
    assert.Contains(t, errors, "invalid region")
}
```

Create BDD test in `tests/features/mycloud_provider.feature`:

```gherkin
Feature: MyCloud Provider Support
  As a platform engineer
  I want to deploy clusters on MyCloud
  So that I can use MyCloud infrastructure

  Scenario: Initialize cluster with MyCloud provider
    When I run "opencenter cluster init test --org my-org --opencenter.provider=mycloud"
    Then the command should succeed
    And the configuration should have provider "mycloud"
    And the configuration should have mycloud.region "us-east-1"

  Scenario: Validate MyCloud credentials
    Given I have a cluster configuration with provider "mycloud"
    And I have set mycloud.api_key to "invalid-key"
    When I run "opencenter cluster preflight test"
    Then the command should fail
    And the error should contain "authentication failed"
```


## Step 8: Update Documentation

Add provider documentation to `docs/reference/providers.md`:

```markdown
### MyCloud

**Status:** Production Ready

MyCloud is a cloud platform providing compute, storage, and networking services.

**Requirements:**
- MyCloud account with API access
- Project ID
- API key with compute permissions

**Configuration:**
```yaml
opencenter:
  infrastructure:
    provider: mycloud
    cloud:
      mycloud:
        api_endpoint: https://api.mycloud.com
        region: us-east-1
        project_id: my-project-id
        api_key: ""  # Set via SOPS encryption
```

**Supported Regions:**
- us-east-1 (US East)
- us-west-1 (US West)
- eu-west-1 (Europe West)

**Instance Types:**
- small: 2 vCPU, 4 GB RAM
- medium: 4 vCPU, 8 GB RAM
- large: 8 vCPU, 16 GB RAM
- xlarge: 16 vCPU, 32 GB RAM
```

Create tutorial in `docs/tutorials/mycloud-deployment.md`:

```markdown
# Deploy Cluster on MyCloud

This tutorial shows how to deploy a production Kubernetes cluster on MyCloud.

## Prerequisites

- MyCloud account
- API key with compute permissions
- Project ID

## Step 1: Initialize Configuration

```bash
opencenter cluster init prod --org my-org --opencenter.provider=mycloud
```

## Step 2: Configure MyCloud Settings

```bash
opencenter cluster edit prod
```

Update MyCloud configuration:
```yaml
opencenter:
  infrastructure:
    cloud:
      mycloud:
        api_endpoint: https://api.mycloud.com
        region: us-east-1
        project_id: my-project-123
```

## Step 3: Encrypt API Key

```bash
# Set API key (will be encrypted with SOPS)
opencenter cluster update prod \
  --opencenter.infrastructure.cloud.mycloud.api_key="your-api-key"
```

## Step 4: Validate Configuration

```bash
opencenter cluster validate prod
opencenter cluster preflight prod
```

## Step 5: Deploy Cluster

```bash
opencenter cluster setup prod
opencenter cluster bootstrap prod
```
```

## Step 9: Test End-to-End

Test the complete workflow:

```bash
# Build CLI
mise run build

# Initialize cluster
./bin/opencenter cluster init mycloud-test --org test-org \
  --opencenter.provider=mycloud

# Validate
./bin/opencenter cluster validate mycloud-test

# Run preflight (requires real credentials)
export MYCLOUD_API_KEY="your-test-key"
./bin/opencenter cluster preflight mycloud-test

# Generate infrastructure
./bin/opencenter cluster setup mycloud-test --render

# Verify generated files
ls -la ~/.config/opencenter/clusters/test-org/mycloud-test/gitops/infrastructure/
```

## Step 10: Submit Pull Request

1. Run all tests:
   ```bash
   mise run test
   mise run godog
   ```

2. Update CHANGELOG.md:
   ```markdown
   ## [Unreleased]
   
   ### Added
   - MyCloud provider support with Terraform templates
   - MyCloud preflight checks for credential validation
   - MyCloud provider documentation and tutorial
   ```

3. Create pull request with:
   - Description of provider capabilities
   - Test results
   - Documentation updates
   - Example configuration

## Provider Checklist

Before submitting, verify:

- [ ] Configuration defaults added to `internal/config/defaults.go`
- [ ] Schema updated in `internal/config/schema.go`
- [ ] Preflight checks implemented in `internal/cloud/<provider>/preflight.go`
- [ ] Preflight registered in `cmd/cluster_preflight.go`
- [ ] Terraform templates created in `internal/gitops/gitops-base-dir/`
- [ ] Provider validation added to `internal/config/<provider>_validator.go`
- [ ] Unit tests written for preflight and validation
- [ ] BDD tests written for provider workflows
- [ ] Provider documented in `docs/reference/providers.md`
- [ ] Tutorial created in `docs/tutorials/<provider>-deployment.md`
- [ ] All tests pass (`mise run test && mise run godog`)
- [ ] Schema verification passes (`mise run schema-verify`)

## Common Issues

### Preflight checks fail in CI

**Problem:** Preflight tests fail because CI doesn't have provider credentials

**Solution:** Mock API calls in tests or skip preflight tests in CI:
```go
func TestPreflight(t *testing.T) {
    if os.Getenv("CI") == "true" {
        t.Skip("Skipping preflight test in CI")
    }
    // ... test implementation
}
```

### Template rendering fails

**Problem:** Template syntax errors or missing configuration fields

**Solution:** Test template rendering:
```bash
mise run build
./bin/opencenter cluster setup test --render
# Check generated files for errors
```

### Schema validation fails

**Problem:** Configuration doesn't match JSON schema

**Solution:** Regenerate schema and verify:
```bash
mise run schema-verify
```

---

## Evidence

This documentation is based on the following repository files:

- Provider defaults: `internal/config/defaults.go:27-451`
- Schema generation: `internal/config/schema.go`
- Preflight checks: `internal/cloud/openstack/preflight.go`, `internal/cloud/` directory
- Preflight command: `cmd/cluster_preflight.go`
- Templates: `internal/gitops/gitops-base-dir/` directory
- Provider validation: `internal/config/*_validator.go` files
- Existing providers: `docs/providers/README.md:1-20`
- Project structure: `.kiro/steering/structure.md:31-68`
