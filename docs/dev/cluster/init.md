# `openCenter cluster init` - Developer Documentation


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Implementation Details](#implementation-details)
- [Dynamic Flag Parsing](#dynamic-flag-parsing)
- [Reflection-Based Field Setting](#reflection-based-field-setting)
- [Key Generation](#key-generation)
- [SOPS Configuration](#sops-configuration)
- [Organization Structure](#organization-structure)
- [Testing](#testing)
- [Error Handling](#error-handling)
- [Performance Considerations](#performance-considerations)
- [Security Considerations](#security-considerations)
- [See Also](#see-also)
## Synopsis

```bash
openCenter cluster init [name] [OPTIONS]
```

## Description

The `init` command creates a new cluster configuration with schema-based defaults and organization structure. It handles dynamic flag parsing, key generation, and directory structure creation.

## Implementation Details

### File Location
`cmd/cluster_init.go`

### Key Functions

#### `newClusterInitCmd()`
Creates and configures the Cobra command with dynamic flag parsing.

#### `setField(obj any, path string, value string)`
Uses reflection to set configuration fields from dot-notation paths.

#### `generateOrganizationSOPSKey()`
Generates Age encryption keys for SOPS in organization structure.

#### `createOrganizationGitignore()`
Creates .gitignore file at organization level.

### Command Flow

1. **Parse Arguments**
   - Resolve cluster name from args or active cluster
   - Initialize ConfigManager and PathResolver

2. **Generate Defaults**
   - Call `config.GenerateDefaultFromSchema(name)`
   - Unmarshal to both map and struct

3. **Apply Overrides**
   - Parse `os.Args` for dynamic flags
   - Apply to both struct (validation) and map (output)
   - Handle `--org` and `--type` flags specially

4. **Determine Organization**
   - Priority: `--org` flag > config > cluster name
   - Validate organization name

5. **Handle Force Flag**
   - Check if cluster exists
   - Cleanup if `--force` is set

6. **Validate (if --strict)**
   - Run `config.Validate(cfg)`
   - Fail if validation errors

7. **Create Structure**
   - Create organization directories
   - Create cluster directories
   - Create .gitignore

8. **Generate Keys (unless --no-keygen)**
   - Generate SOPS Age key pair
   - Generate SSH key pair (ed25519 default)
   - Update configuration with key paths

9. **Update Paths**
   - Set GitOps directory (organization-based)
   - Set SSH key paths
   - Set SOPS key path

10. **Write Configuration**
    - Marshal final config to YAML
    - Write to organization directory
    - Set file permissions (0600)

## Dynamic Flag Parsing

### Cobra Configuration

```go
FParseErrWhitelist: cobra.FParseErrWhitelist{
    UnknownFlags: true,
},
```

This allows unknown flags to be ignored by Cobra.

### Manual Parsing

```go
for _, arg := range os.Args {
    if strings.HasPrefix(arg, "--") {
        parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
        if len(parts) == 2 {
            key, value := parts[0], parts[1]
            // Skip known flags
            if cmd.Flags().Lookup(key) != nil {
                continue
            }
            // Apply to configuration
            if err := setField(&cfg, key, value); err != nil {
                return err
            }
        }
    }
}
```

## Reflection-Based Field Setting

### setField Function

Uses reflection to traverse struct hierarchy and set values:

```go
func setField(obj any, path string, value string) error {
    v := reflect.ValueOf(obj).Elem()
    parts := strings.Split(path, ".")
    
    for i, part := range parts {
        // Find field by YAML tag
        field := util.FindField(v, part)
        
        if !field.IsValid() {
            // Handle maps
            if v.Kind() == reflect.Map {
                // Set map value
            }
            return fmt.Errorf("field not found: '%s'", part)
        }
        
        // Last part: set value
        if i == len(parts)-1 {
            return setFieldValue(field, value)
        }
        
        // Traverse deeper
        // Handle struct, pointer, or map
    }
    return nil
}
```

### Type Conversion

```go
func setReflectValue(field reflect.Value, value string) error {
    switch field.Kind() {
    case reflect.String:
        field.SetString(value)
    case reflect.Int, reflect.Int64:
        i, _ := strconv.ParseInt(value, 10, 64)
        field.SetInt(i)
    case reflect.Bool:
        b, _ := strconv.ParseBool(value)
        field.SetBool(b)
    case reflect.Interface:
        // Auto-detect type
        if b, err := strconv.ParseBool(value); err == nil {
            field.Set(reflect.ValueOf(b))
        } else if i, err := strconv.ParseInt(value, 10, 64); err == nil {
            field.Set(reflect.ValueOf(i))
        } else {
            field.Set(reflect.ValueOf(value))
        }
    }
    return nil
}
```

## Key Generation

### SOPS Age Key

```go
func generateOrganizationSOPSKey(cluster, organization string, cfg *config.Config, pathResolver *config.PathResolver) error {
    clusterPaths := pathResolver.ResolveClusterPaths(cluster, organization)
    
    // Create secrets directory
    secretsKeyDir := filepath.Dir(clusterPaths.SOPSKeyPath)
    os.MkdirAll(secretsKeyDir, 0o755)
    
    // Generate key pair
    km := sops.NewKeyManager(secretsKeyDir)
    keyPair, err := km.GenerateAgeKey()
    
    // Save key
    km.SaveAgeKey(keyPair, cluster)
    
    // Create SOPS config
    createOrganizationSOPSConfig(clusterPaths.SOPSConfigPath, keyPair.PublicKey, cluster)
    
    cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
    return nil
}
```

### SSH Key Pair

```go
// Generate SSH key with comment
sshKeyComment := fmt.Sprintf("%s-%s-%s", organization, name, region)
keyPair, err := crypto.GenerateSSHKeyWithComment(cfg.Secrets.SSHKey.Cypher, sshKeyComment)

// Write private key (0600)
os.WriteFile(secretsSSHKeyPath, keyPair.PrivateKey, 0o600)

// Write public key (0644)
os.WriteFile(secretsSSHPubKeyPath, keyPair.PublicKey, 0o644)
```

## SOPS Configuration

### Organization-Wide Config

```yaml
# SOPS configuration for organization
# Each cluster's key encrypts only its specific directories
creation_rules:
  - path_regex: (applications/overlays/my-cluster/.*|infrastructure/clusters/my-cluster/.*)\.ya?ml$
    age: >-
      age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Path-Based Encryption

Each cluster's key only encrypts files in:
- `applications/overlays/<cluster>/`
- `infrastructure/clusters/<cluster>/`

## Organization Structure

### Directory Creation

```go
func (pr *PathResolver) CreateOrganizationStructure(organization string) error {
    orgDir := filepath.Join(pr.configManager.ClustersDir(), organization)
    
    // Create organization directory
    os.MkdirAll(orgDir, 0o755)
    
    // Create secrets subdirectories
    os.MkdirAll(filepath.Join(orgDir, "secrets", "age", "keys"), 0o755)
    os.MkdirAll(filepath.Join(orgDir, "secrets", "ssh"), 0o700)
    
    // Create GitOps directory
    os.MkdirAll(filepath.Join(orgDir, "gitops"), 0o755)
    
    return nil
}
```

## Testing

### Unit Tests

```go
func TestClusterInit(t *testing.T) {
    // Test basic initialization
    // Test with organization
    // Test with custom values
    // Test force overwrite
    // Test strict validation
}

func TestSetField(t *testing.T) {
    // Test field setting with reflection
    // Test nested fields
    // Test type conversion
}
```

### BDD Tests

```gherkin
Feature: Cluster Initialization
  Scenario: Initialize cluster with defaults
    When I run "openCenter cluster init test-cluster"
    Then the cluster configuration should exist
    And the organization directory should be created
    And SOPS keys should be generated
    And SSH keys should be generated
```

## Error Handling

### Common Errors

1. **Cluster Already Exists**
   - Check if directory exists
   - Require `--force` flag
   - Cleanup and recreate

2. **Invalid Organization Name**
   - Validate name format
   - Check for special characters
   - Ensure safe for filesystem

3. **Key Generation Failure**
   - Check directory permissions
   - Verify crypto libraries
   - Fallback to manual key setup

4. **Validation Failure (--strict)**
   - Display all validation errors
   - Exit with error code
   - Suggest fixes

## Performance Considerations

- Schema generation is cached
- Reflection is used sparingly
- File operations are batched
- Directory creation is idempotent

## Security Considerations

- Config files: 0600 permissions
- SOPS keys: 0600 permissions
- SSH private keys: 0600 permissions
- SSH public keys: 0644 permissions
- Directories: 0755 permissions
- Secrets directory: 0700 permissions

## See Also

- [Configuration Schema](../../reference/configuration.md)
- [Path Resolution](path_resolver.md)
- [SOPS Integration](../../explanation/sops.md)
