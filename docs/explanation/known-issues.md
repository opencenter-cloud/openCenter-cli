# Known Issues and Limitations


## Table of Contents

- [Validation System](#validation-system)
- [Cloud Provider Support](#cloud-provider-support)
- [GitOps and Deployment](#gitops-and-deployment)
- [Configuration Management](#configuration-management)
- [User Interface](#user-interface)
- [Provider-Specific Limitations](#provider-specific-limitations)
- [Performance](#performance)
- [Security](#security)
- [Documentation](#documentation)
- [Workarounds Summary](#workarounds-summary)
- [Reporting Issues](#reporting-issues)
- [Future Improvements](#future-improvements)
- [Related Documentation](#related-documentation)
- [Conclusion](#conclusion)
**doc_type**: explanation

This document describes current limitations, known issues, and planned improvements in openCenter. It's organized by functional area to help you understand what works, what doesn't, and what workarounds exist.

## Validation System

### Pipeline Adapter is a Stub

**Issue**: The `PipelineAdapter` in `internal/config/pipeline_adapter.go` is currently a stub implementation that always returns valid results.

**Impact**: The validation pipeline architecture described in documentation exists, but the actual pipeline adapter doesn't perform validation. Instead, validation runs through the `EnhancedConfigValidator` directly.

**Workaround**: Use `openCenter cluster validate` which calls the enhanced validator. The validation still works—it just doesn't use the pipeline adapter architecture yet.

**Status**: Planned for implementation. The architecture is designed and documented in `docs/dev/validation-pipeline.md`.

### Auto-Repair Mode is Experimental

**Issue**: The `--auto-repair` flag exists but auto-repair logic is limited.

**Impact**: Auto-repair can fix some simple issues (whitespace, case normalization) but doesn't handle complex repairs like adding missing required fields with defaults.

**Workaround**: Fix validation errors manually using the suggestions provided. The suggestion engine gives specific guidance for each error.

**Status**: Auto-repair will be expanded as we identify common fixable error patterns.

### Service-Specific Schema Validation

**Issue**: Some services (Loki, Velero) use the base service schema instead of service-specific schemas.

**Impact**: Service-specific configuration fields aren't validated as strictly as they could be. You might configure invalid service options that pass validation but fail at deployment.

**Workaround**: Refer to service documentation for valid configuration options. FluxCD will report errors if service configuration is invalid.

**Status**: Service-specific schemas will be added as services mature and configuration patterns stabilize.

## Cloud Provider Support

### Drift Detection Not Implemented

**Issue**: The `cluster drift` command exists but returns "not yet available" errors.

**Impact**: You can't detect configuration drift between your config file and actual cloud resources. Manual changes to infrastructure aren't automatically detected.

**Workaround**: Track infrastructure changes manually. Use cloud provider consoles or CLI tools to compare actual resources with expected configuration.

**Status**: Planned for future release. Requires cloud provider factory implementation and state comparison logic.

### Talos Provider Validators are Stubs

**Issue**: Talos-specific validators in `internal/talos/validator/` contain placeholder implementations for:
- Quota checks (compute, network, storage)
- Keystone availability and MFA enforcement
- Barbican secret management
- Glance image verification
- Octavia load balancer quota

**Impact**: Talos provider validation doesn't perform actual API checks. You won't get preflight warnings about quota exhaustion or service unavailability.

**Workaround**: Manually verify OpenStack service availability and quotas before deploying Talos clusters. Use OpenStack CLI tools:
```bash
openstack quota show
openstack service list
openstack image list
```

**Status**: Planned for implementation once Talos provider integration is complete.

### Bare Metal Support

**Issue**: openCenter assumes cloud provider APIs for provisioning. There's no bare metal provider.

**Impact**: You can't use openCenter to provision bare metal servers. You'd need to provision manually and then apply generated Kubernetes manifests.

**Workaround**: 
1. Provision bare metal servers with your existing tools
2. Use openCenter to generate Kubernetes manifests
3. Manually apply manifests to your infrastructure

This workflow isn't officially supported but is technically possible.

**Status**: Bare metal support is not currently planned. The focus is on cloud providers with APIs.

## GitOps and Deployment

### No Automated Cluster Upgrades

**Issue**: openCenter creates clusters but doesn't manage Kubernetes version upgrades.

**Impact**: To upgrade Kubernetes, you must:
1. Update `kubernetes.version` in config
2. Regenerate GitOps repository
3. Manually follow provider-specific upgrade procedures (drain nodes, update, etc.)

**Workaround**: Follow Kubernetes upgrade best practices for your provider. For Kubespray-based deployments, use Ansible playbooks. For managed Kubernetes, use provider tools.

**Status**: Automated upgrades are planned for a future release. This requires careful orchestration to avoid downtime.

### Backup Scheduling Not Implemented

**Issue**: The `ScheduleBackups` method in `internal/operations/backup_manager.go` returns "not yet implemented".

**Impact**: You can't schedule automatic backups through openCenter. Backup operations must be triggered manually.

**Workaround**: Use external scheduling tools:
- Kubernetes CronJobs for in-cluster backups
- System cron for configuration backups
- Cloud provider backup services

**Status**: Backup scheduling will be added once the backup manager implementation is complete.

### Directory Archiving Incomplete

**Issue**: The `archiveDirectory` method in backup manager is a stub.

**Impact**: Directory-based backups don't create tar archives. Only file-based backups work.

**Workaround**: Use external tools to archive directories:
```bash
tar -czf backup.tar.gz /path/to/directory
```

**Status**: Will be implemented as part of backup manager completion.

## Configuration Management

### No Configuration Inheritance

**Issue**: Each cluster configuration is completely independent. There's no way to share common settings across clusters.

**Impact**: If you manage many clusters with similar configurations, you'll have duplication. Changing a common setting requires updating multiple files.

**Workaround**: Use templating or scripting to generate configurations:
```bash
# Generate multiple configs from a template
for cluster in prod staging dev; do
  sed "s/CLUSTER_NAME/$cluster/g" template.yaml > $cluster-config.yaml
done
```

**Status**: Configuration inheritance is not planned. The single-file approach is intentional for auditability and simplicity. Consider using external configuration management if you need inheritance.

### Legacy Service Fields

**Issue**: The `BaseService` struct contains deprecated fields (`Email`, `Region`) kept for backward compatibility.

**Impact**: These fields appear in schema and documentation but shouldn't be used. They'll be removed in a future version.

**Workaround**: Use service-specific configuration fields instead of legacy fields. Check service documentation for current field names.

**Status**: Legacy fields will be removed in version 2.0 after a deprecation period.

### Flag-Based Configuration Updates

**Issue**: The `--set` flag integration in `internal/config/flags/integration.go` has TODO comments indicating incomplete struct application.

**Impact**: Flag-based configuration updates work for map-based configuration but may not fully apply to all struct fields.

**Workaround**: Edit configuration files directly for complex updates. Use `--set` for simple key-value changes.

**Status**: Full struct application will be implemented as the flag system matures.

## User Interface

### Confirmation Prompts in Tests

**Issue**: The `cluster destroy` command has a TODO about confirmation prompts not working properly in test framework.

**Impact**: Tests skip confirmation prompts, which means the interactive confirmation flow isn't fully tested.

**Workaround**: Use `--force` flag to skip confirmation in automated contexts. Manual testing is required for confirmation prompt behavior.

**Status**: Will be fixed when test framework supports interactive prompt testing.

### Error Code Registry Incomplete

**Issue**: The error formatter in `internal/ui/error_formatter.go` has error code categories (E1xxx, E2xxx, etc.) but not all error codes are defined.

**Impact**: Some errors might not have structured error codes or detailed recovery suggestions.

**Workaround**: Use error messages and suggestions provided. Even without error codes, the error output includes actionable guidance.

**Status**: Error codes will be added incrementally as error patterns are identified and standardized.

## Provider-Specific Limitations

### OpenStack

**Limitations**:
- VRRP IP configuration requires manual network setup
- Octavia load balancer support is validated but not fully tested at scale
- Application credential rotation requires manual SOPS re-encryption

**Workarounds**:
- Use Octavia instead of VRRP for production (set `use_octavia: true`)
- Test load balancer configuration in staging before production
- Document credential rotation procedures for your team

### AWS

**Limitations**:
- VPC creation is basic (single subnet, default routing)
- IAM role creation is not automated
- EKS integration is not available (only EC2-based clusters)

**Workarounds**:
- Create VPCs manually for complex networking requirements
- Set up IAM roles before running openCenter
- Use EC2 instances with Kubespray for Kubernetes deployment

### vSphere

**Limitations**:
- Resource pool configuration is basic
- DRS rules are not configured automatically
- Storage policy selection is manual

**Workarounds**:
- Configure resource pools and DRS rules in vCenter before deployment
- Specify storage policies in configuration file
- Use vSphere tags for resource organization

### Kind

**Limitations**:
- Only suitable for local development
- No multi-node cluster support
- Limited networking options

**Workarounds**:
- Use Kind only for development and testing
- For multi-node testing, use a cloud provider
- Configure Kind networking manually if needed

## Performance

### Validation Timeout

**Issue**: Connectivity validation has a fixed 10-second timeout per check.

**Impact**: Slow networks or overloaded cloud providers might cause false-positive connectivity failures.

**Workaround**: Use `--skip-connectivity` to skip connectivity checks if you're confident the configuration is correct. Connectivity will be verified during bootstrap anyway.

**Status**: Configurable timeouts are planned for a future release.

### Sequential Provider Checks

**Issue**: Provider validation runs checks sequentially, not in parallel.

**Impact**: Validation can take several seconds when multiple connectivity checks are needed.

**Workaround**: Skip connectivity checks in CI/CD environments where speed matters more than comprehensive validation.

**Status**: Parallel validation is planned to reduce total validation time.

### Large Configuration Files

**Issue**: Very large configuration files (>10MB) might cause performance issues during validation and rendering.

**Impact**: Validation and template rendering could take several seconds for extremely large configurations.

**Workaround**: Keep configuration files focused. If you have many services, consider whether all are needed. Large configurations are usually a sign of over-configuration.

**Status**: Performance optimization for large files is not currently prioritized. Most configurations are under 100KB.

## Security

### Plaintext Secret Detection

**Issue**: The validator detects plaintext secrets by checking for absence of `ENC[` prefix. This is a heuristic, not cryptographic verification.

**Impact**: Cleverly formatted plaintext secrets might not be detected. The check catches obvious cases but isn't foolproof.

**Workaround**: Always use SOPS encryption for production secrets. Don't rely on validation to catch all plaintext secrets—make encryption part of your workflow.

**Status**: Improved secret detection using entropy analysis is planned but not scheduled.

### Key Rotation Complexity

**Issue**: Rotating SOPS encryption keys requires manual steps (generate new key, re-encrypt all secrets, update `.sops.yaml`).

**Impact**: Key rotation is error-prone. Missing a file or misconfiguring `.sops.yaml` can leave secrets encrypted with the old key.

**Workaround**: Document your key rotation procedure. Test it in a non-production environment first. Consider using a script to automate the steps.

**Status**: Automated key rotation is planned for a future release.

## Documentation

### Provider-Specific Guides Incomplete

**Issue**: Not all providers have complete documentation in `docs/providers/`.

**Impact**: You might need to refer to provider documentation directly for advanced configuration options.

**Workaround**: Check provider official documentation for details not covered in openCenter docs. Open an issue if you find gaps.

**Status**: Provider documentation is being expanded incrementally.

### API Documentation

**Issue**: There's no API documentation for using openCenter as a library.

**Impact**: If you want to embed openCenter in another Go application, you'll need to read the source code to understand the API.

**Workaround**: The CLI commands in `cmd/` show how to use the internal packages. Use them as examples.

**Status**: API documentation is not currently planned. openCenter is designed as a CLI tool, not a library.

## Workarounds Summary

Here's a quick reference for common issues and their workarounds:

| Issue | Workaround |
|-------|-----------|
| Drift detection unavailable | Use cloud provider tools to detect drift manually |
| No automated upgrades | Follow provider-specific upgrade procedures |
| No configuration inheritance | Use templating or scripting to generate configs |
| Slow connectivity validation | Use `--skip-connectivity` flag |
| Plaintext secrets not detected | Always use SOPS encryption, don't rely on validation |
| Backup scheduling unavailable | Use CronJobs or external scheduling tools |
| Provider validators are stubs | Manually verify quotas and service availability |
| No bare metal support | Provision manually, then apply generated manifests |

## Reporting Issues

If you encounter an issue not listed here:

1. Check if it's a configuration error (run `openCenter cluster validate`)
2. Search GitHub issues for similar problems
3. If not found, open a new issue with:
   - openCenter version (`openCenter version`)
   - Provider and environment details
   - Steps to reproduce
   - Expected vs. actual behavior
   - Configuration file (redact secrets)

For security issues, see `SECURITY.md` for responsible disclosure procedures.

## Future Improvements

Planned improvements that will address current limitations:

- **Full validation pipeline**: Complete the pipeline adapter implementation
- **Drift detection**: Detect and reconcile configuration drift
- **Automated upgrades**: Safe, automated Kubernetes version upgrades
- **Backup scheduling**: Built-in backup scheduling and retention
- **Parallel validation**: Speed up validation with concurrent checks
- **Configuration inheritance**: Share common settings across clusters (under consideration)
- **Enhanced error codes**: Complete error code registry with recovery procedures
- **Provider API integration**: Real API checks instead of stub implementations

Check the GitHub milestones for planned release timelines.

## Related Documentation

- [FAQ](faq.md) - Common questions and answers
- [Troubleshooting Guide](../how-to/troubleshooting.md) - Detailed problem-solving steps
- [Validation Pipeline](validation-pipeline.md) - How validation works
- [Architecture](architecture.md) - System design and trade-offs
- [Contributing](../contributing.md) - How to help fix these issues

## Conclusion

openCenter is under active development. These limitations reflect the current state, not the final vision. Many are planned for future releases.

The workarounds provided are production-tested. They're not ideal, but they work. If you find better workarounds or want to contribute fixes, see the contributing guide.

Understanding these limitations helps you make informed decisions about using openCenter. For most use cases, the workarounds are sufficient. For cases where they're not, consider whether openCenter is the right tool for your needs.
