# Deprecated Service Fields Removal

## Overview

As part of the v2.0.0 release, deprecated service configuration fields have been removed from test configurations. The struct definitions still contain these fields marked as deprecated for backward compatibility during migration, but they will be fully removed in a future release.

## Removed Fields from Test Configs

### VSphere CSI Deprecated Fields

The following fields have been removed from VSphere CSI service configurations:

- `datastore_name` - Use `storage_classes[].name` instead
- `datastoreurl` - Use `storage_classes[].datastore_url` instead  
- `delete_datastore_uuid` - No longer supported
- `retain_datastore_name` - Use `storage_classes[].reclaim_policy: Retain` instead
- `retain_datastore_uuid` - No longer supported

**Migration Example:**

```yaml
# Old (deprecated)
services:
  vsphere-csi:
    enabled: true
    datastore_name: "my-datastore"
    datastoreurl: "ds:///vmfs/volumes/12345"
    retain_datastore_name: "retain-datastore"

# New (v2.0.0)
services:
  vsphere-csi:
    enabled: true
    storage_classes:
      - name: my-datastore-delete
        datastore_url: "ds:///vmfs/volumes/12345"
        reclaim_policy: Delete
        allow_expansion: true
      - name: my-datastore-retain
        datastore_url: "ds:///vmfs/volumes/12345"
        reclaim_policy: Retain
        allow_expansion: true
```

### Loki Swift Authentication Deprecated Fields

The following Swift authentication fields have been removed from Loki service configurations:

- `swift_username` - Use `swift_application_credential_id` instead
- `swift_project_name` - Use `swift_application_credential_id` instead
- `swift_password` (in secrets) - Use `swift_application_credential_secret` instead

**Migration Example:**

```yaml
# Old (deprecated - username/password authentication)
services:
  loki:
    enabled: true
    swift_auth_url: "https://keystone.api.example.com/v3/"
    swift_username: "my-user"
    swift_project_name: "my-project"
    swift_region: "ORD1"

secrets:
  loki:
    swift_password: "my-password"

# New (v2.0.0 - application credentials)
services:
  loki:
    enabled: true
    swift_auth_url: "https://keystone.api.example.com/v3/"
    swift_application_credential_id: "abc123def456"
    swift_region: "ORD1"
    swift_container_name: "loki-logs"

secrets:
  loki:
    swift_application_credential_secret: "secret-credential-value"
```

## Why Application Credentials?

OpenStack application credentials provide several advantages over username/password authentication:

1. **Scoped Access**: Can be limited to specific operations
2. **No Password Exposure**: Credentials are separate from user passwords
3. **Revocable**: Can be revoked without changing user password
4. **Auditable**: Separate audit trail for application access
5. **Best Practice**: Recommended by OpenStack for application authentication

## Creating Application Credentials

To create OpenStack application credentials:

```bash
# Using OpenStack CLI
openstack application credential create \
  --description "Loki storage access" \
  --role member \
  loki-storage

# Output will include:
# - id: Use for swift_application_credential_id
# - secret: Use for swift_application_credential_secret
```

## Struct Definitions

The deprecated fields are still present in struct definitions for backward compatibility:

```go
// internal/config/types_services.go
type SimplifiedOpenCenter struct {
    // ... other fields ...
    
    // Legacy Swift fields (deprecated)
    SwiftUsername    string `yaml:"swift_username,omitempty" json:"swift_username,omitempty" jsonschema:"description=Swift username (deprecated)"`
    SwiftProjectName string `yaml:"swift_project_name,omitempty" json:"swift_project_name,omitempty" jsonschema:"description=Swift project name (deprecated)"`
}

// internal/config/types_secrets.go
type LokiSecrets struct {
    // Swift secrets
    SwiftPassword                    string `yaml:"swift_password" json:"swift_password" jsonschema:"secret=true,description=Swift storage password (deprecated: use application credentials)"`
    SwiftApplicationCredentialSecret string `yaml:"swift_application_credential_secret" json:"swift_application_credential_secret" jsonschema:"secret=true,description=Swift application credential secret"`
}
```

These will be fully removed in a future release after the migration period.

## Validation

The validation logic still accepts both old and new authentication methods during the transition:

```go
// internal/config/enhanced_validator.go
if hasSwiftConfig {
    hasSwiftPassword := config.Secrets.Loki.SwiftPassword != ""
    hasSwiftAppCred := config.Secrets.Loki.SwiftApplicationCredentialSecret != ""
    
    if !hasSwiftPassword && !hasSwiftAppCred {
        // Error: need one or the other
    }
}
```

## Timeline

- **v2.0.0**: Deprecated fields removed from test configs, marked as deprecated in structs
- **v2.1.0** (planned): Deprecated fields will be fully removed from struct definitions
- **v2.0.0+**: New configurations should use the new field structure

## Related Documentation

- [VSphere CSI Configuration](../providers/vsphere-csi.md)
- [Loki Configuration](../services/loki.md)
- [OpenStack Application Credentials](https://docs.openstack.org/keystone/latest/user/application_credentials.html)
- [v2.0.0 Migration Guide](../migration/v1-to-v2.md)

## Script for Automated Cleanup

A script is provided to remove deprecated fields from existing configurations:

```bash
./hack/remove-deprecated-service-fields.sh
```

This script:
1. Finds all YAML config files in `cmd/clusters/`
2. Removes deprecated VSphere CSI fields
3. Removes deprecated Loki Swift authentication fields
4. Preserves all other configuration

**Note**: Always backup your configurations before running automated cleanup scripts.
