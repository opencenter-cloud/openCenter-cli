# Baremetal Schema Comparison: v1 vs v2

## Overview

This document compares v1 and v2 schema structures for baremetal cluster configurations, highlighting key differences and migration paths.

## Quick Reference

| Aspect | v1 | v2 |
|--------|----|----|
| Schema Version Field | Optional (defaults to v1) | **Required**: `schema_version: "2.0"` |
| Networking Location | `opencenter.cluster.networking` | `opencenter.infrastructure.networking` |
| Storage Location | `opencenter.storage` | `opencenter.infrastructure.storage` |
| SSH Location | `opencenter.cluster.ssh` | `opencenter.infrastructure.ssh` |
| Deployment Location | `opencenter.deployment` | `deployment` (root level) |
| Meta Fields | Mixed in `cluster` | Dedicated `opencenter.meta` domain |
| CSI Plugin Selection | Not explicit | Explicit `storage_plugin` section |

## Key Structural Differences

### 1. Schema Version Declaration

**v1:**
```yaml
# No schema_version field (implicit v1)
opencenter:
  cluster:
    cluster_name: baremetal-cluster
```

**v2:**
```yaml
# Explicit schema version required
schema_version: "2.0"

opencenter:
  meta:
    name: baremetal-cluster
```

### 2. Meta Domain Separation

**v1:** Meta fields mixed with cluster configuration
```yaml
opencenter:
  cluster:
    cluster_name: baremetal-cluster
    organization: my-org
    env: dev
    region: datacenter1
    base_domain: example.com
```

**v2:** Dedicated meta domain for identity
```yaml
opencenter:
  meta:
    name: baremetal-cluster
    organization: my-org
    env: dev
    region: datacenter1
    status: active
  
  cluster:
    cluster_name: baremetal-cluster  # Must match meta.name
    base_domain: example.com
```

### 3. Networking Configuration

**v1:** Infrastructure networking in cluster domain
```yaml
opencenter:
  cluster:
    networking:
      subnet_nodes: "192.168.1.0/24"
      allocation_pool_start: "192.168.1.10"
      allocation_pool_end: "192.168.1.100"
      vrrp_ip: ""
      dns_nameservers:
        - "8.8.8.8"
      ntp_servers:
        - "time.google.com"
```

**v2:** Infrastructure networking in infrastructure domain
```yaml
opencenter:
  infrastructure:
    networking:
      subnet_nodes: "192.168.1.0/24"
      allocation_pool_start: "192.168.1.10"
      allocation_pool_end: "192.168.1.100"
      vrrp_ip: ""
      dns_nameservers:
        - "8.8.8.8"
      ntp_servers:
        - "time.google.com"
      
      security:
        firewall_enabled: false
        allowed_cidr_blocks:
          - "192.168.1.0/24"
      
      vlan:
        enabled: false
```

### 4. Storage Configuration

**v1:** Top-level storage domain
```yaml
opencenter:
  storage:
    default_storage_class: "local-path"
    worker_volume_size: 100
    worker_volume_destination_type: "local"
    worker_volume_source_type: "image"
    worker_volume_type: "local"
```

**v2:** Storage under infrastructure domain
```yaml
opencenter:
  infrastructure:
    storage:
      default_storage_class: "local-path"
      worker_volume_size: 100
      worker_volume_destination_type: "local"
      worker_volume_source_type: "image"
      worker_volume_type: "local"
      worker_volume_delete_on_termination: false
```

### 5. SSH Configuration

**v1:** SSH in cluster domain
```yaml
opencenter:
  cluster:
    ssh:
      user: ubuntu
      key_path: "~/.ssh/baremetal-cluster-key"
      authorized_keys:
        - "ssh-rsa AAAAB3..."
```

**v2:** SSH in infrastructure domain
```yaml
opencenter:
  infrastructure:
    ssh:
      user: ubuntu
      key_path: "~/.ssh/baremetal-cluster-key"
      authorized_keys:
        - "ssh-rsa AAAAB3..."
```

### 6. Deployment Configuration

**v1:** Deployment under opencenter
```yaml
opencenter:
  deployment:
    method: kubespray
    kubespray:
      version: "v2.29.1"
      modules:
        metallb:
          enabled: true
```

**v2:** Deployment at root level
```yaml
deployment:
  auto_deploy: false
  method: kubespray
  
  kubespray:
    version: "v2.29.1"
    modules:
      metallb:
        enabled: true
```

### 7. Storage Plugin Selection

**v1:** No explicit CSI plugin selection
```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
      # No storage_plugin section
```

**v2:** Explicit CSI plugin selection (similar to CNI)
```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
      
      storage_plugin:
        cinder_csi:
          enabled: false
        aws_ebs_csi:
          enabled: false
        vsphere_csi:
          enabled: false
```

### 8. Baremetal Node Inventory

**v1 & v2:** Same structure (no change)
```yaml
opencenter:
  infrastructure:
    cloud:
      baremetal:
        nodes:
          - name: master-01
            ip: "192.168.1.10"
            role: master
            mac_address: "00:11:22:33:44:55"
          - name: worker-01
            ip: "192.168.1.11"
            role: worker
            mac_address: "00:11:22:33:44:66"
```

## Complete Side-by-Side Comparison

### v1 Configuration Structure

```yaml
opencenter:
  cluster:                              # Mixed identity + Kubernetes config
    cluster_name: baremetal-cluster
    organization: my-org
    env: dev
    region: datacenter1
    base_domain: example.com
    
    ssh:                                # v1 location
      user: ubuntu
    
    networking:                         # v1 location
      subnet_nodes: "192.168.1.0/24"
    
    kubernetes:
      version: "1.31.4"
      network_plugin:
        calico:
          enabled: true
  
  infrastructure:
    provider: baremetal
    compute:
      master_count: 1
    cloud:
      baremetal:
        nodes: [...]
  
  storage:                              # v1 location
    default_storage_class: "local-path"
  
  deployment:                           # v1 location
    method: kubespray
  
  services:
    metallb:
      enabled: true
```

### v2 Configuration Structure

```yaml
schema_version: "2.0"                   # Required

opencenter:
  meta:                                 # Dedicated identity domain
    name: baremetal-cluster
    organization: my-org
    env: dev
    region: datacenter1
    status: active
  
  cluster:                              # Pure Kubernetes config
    cluster_name: baremetal-cluster
    base_domain: example.com
    
    kubernetes:
      version: "1.31.4"
      network_plugin:
        calico:
          enabled: true
      storage_plugin:                   # New in v2
        cinder_csi:
          enabled: false
  
  infrastructure:                       # All infrastructure concerns
    provider: baremetal
    
    ssh:                                # v2 location
      user: ubuntu
    
    networking:                         # v2 location
      subnet_nodes: "192.168.1.0/24"
      security:                         # New in v2
        firewall_enabled: false
      vlan:                             # New in v2
        enabled: false
    
    compute:
      master_count: 1
    
    storage:                            # v2 location
      default_storage_class: "local-path"
    
    cloud:
      baremetal:
        nodes: [...]
  
  services:
    metallb:
      enabled: true

deployment:                             # v2 location (root level)
  auto_deploy: false
  method: kubespray
```

## Migration Path

### Automated Migration

```bash
# Migrate v1 to v2
opencenter cluster migrate-config \
  --input baremetal-v1.yaml \
  --output baremetal-v2.yaml

# Validate migrated config
opencenter cluster validate --config baremetal-v2.yaml
```

### Manual Migration Steps

1. **Add schema version**
   ```yaml
   schema_version: "2.0"
   ```

2. **Create meta domain**
   - Move `cluster_name` → `meta.name`
   - Move `organization` → `meta.organization`
   - Move `env` → `meta.env`
   - Move `region` → `meta.region`
   - Add `status` field

3. **Relocate networking**
   - Move `cluster.networking.*` → `infrastructure.networking.*`
   - Add `security` and `vlan` sections if needed

4. **Relocate storage**
   - Move `opencenter.storage.*` → `infrastructure.storage.*`

5. **Relocate SSH**
   - Move `cluster.ssh.*` → `infrastructure.ssh.*`

6. **Relocate deployment**
   - Move `opencenter.deployment.*` → `deployment.*` (root level)
   - Add `auto_deploy` field

7. **Add storage plugin selection**
   ```yaml
   cluster:
     kubernetes:
       storage_plugin:
         cinder_csi:
           enabled: false
   ```

## Validation Differences

### v1 Validation

```bash
opencenter cluster validate --config baremetal-v1.yaml

# Output:
# ⚠ WARNING: Using v1 schema (deprecated)
# ✓ Configuration is valid (v1 schema)
```

### v2 Validation

```bash
opencenter cluster validate --config baremetal-v2.yaml

# Output:
# ✓ Schema validation passed
# ✓ Business rules validation passed
# ✓ Provider validation passed (baremetal)
# ✓ Deployment validation passed (Kubespray)
# ✓ Configuration is valid ✓
```

## Benefits of v2 Schema

1. **Clear Domain Separation**: Each configuration domain has a single, well-defined purpose
2. **No Duplication**: VRRP IP and other settings have exactly one location
3. **Better Validation**: Multi-layered validation with clear error messages
4. **Provider Isolation**: Provider-specific settings cleanly separated
5. **Advanced Features**: Support for Kamaji, reference resolution, provider-region defaults
6. **Explicit Plugin Selection**: Both CNI and CSI plugins explicitly declared

## Backward Compatibility

- v1 configurations continue to work during coexistence period
- Migration tool handles all field relocations automatically
- Both schemas can coexist in the same environment
- v1 support will be removed in Q3 2027

## Related Documentation

- [v1 to v2 Migration Guide](../migration-guide.md)
- [v2 Configuration Reference](../v2-reference.md)
- [v1 Example](v1/baremetal-minimal.yaml)
- [v2 Example](v2/baremetal-minimal.yaml)

---

**Last Updated**: January 2026  
**Schema Versions**: v1.0 (deprecated), v2.0 (current)
