# Validation Pipeline Architecture


## Table of Contents

- [What Validation Does](#what-validation-does)
- [The Four Validation Stages](#the-four-validation-stages)
- [Error Handling and Suggestions](#error-handling-and-suggestions)
- [Why Staged Validation?](#why-staged-validation)
- [Validation Result Structure](#validation-result-structure)
- [Auto-Repair Mode](#auto-repair-mode)
- [Validation Performance](#validation-performance)
- [Integration with Other Systems](#integration-with-other-systems)
- [Trade-offs and Limitations](#trade-offs-and-limitations)
- [Common Validation Patterns](#common-validation-patterns)
- [Future Directions](#future-directions)
- [Related Documentation](#related-documentation)
- [Conclusion](#conclusion)
**doc_type**: explanation

This document explains how openCenter validates cluster configurations before deployment. It covers the validation system's architecture, the stages configurations pass through, and why validation is structured this way.

## What Validation Does

Validation catches configuration errors before they reach production infrastructure. A configuration file might be syntactically correct YAML but semantically wrong—like specifying three network plugins when only one can run, or providing an OpenStack auth URL without credentials. The validation pipeline detects these problems and suggests fixes.

The pipeline runs automatically during `cluster validate` commands and before any operation that modifies infrastructure. You can't bootstrap a cluster with an invalid configuration. This prevents a class of deployment failures that would otherwise require manual cleanup.

## The Four Validation Stages

Validation happens in four distinct stages, each catching different error types:

### Stage 1: Schema Validation

The first stage checks structural correctness against a JSON schema. This catches:

- Missing required fields (`cluster_name`, `git_dir`, `provider`)
- Wrong data types (string where integer expected)
- Invalid enum values (provider must be `openstack`, `aws`, `vsphere`, or `kind`)
- Unknown fields (typos in field names)

Schema validation runs fast because it's pure structure checking. No business logic, no API calls, just "does this YAML match the expected shape?"

The schema is generated from Go struct tags using `hack/generate-schema-from-structs.go`. When you add a field to the configuration structs, the schema updates automatically. This keeps validation in sync with the code.

### Stage 2: Business Rules Validation

The second stage checks logical constraints that can't be expressed in JSON schema:

- Node counts must be positive (master_count >= 1, worker_count >= 0)
- Cluster names must use valid characters (alphanumeric, hyphens, underscores)
- Kubernetes versions must follow semantic versioning (1.31.4, not 1.31)
- Network subnets must be valid CIDR notation (10.42.0.0/16)
- Pod and service subnets can't overlap

These rules live in `internal/config/enhanced_validator.go`. The `validateBasicStructure` and `validateCrossFieldDependencies` methods implement them.

Business rules validation catches configuration mistakes that would cause runtime failures. For example, if you enable Windows workers but set `worker_count_windows: 0`, that's a logical contradiction. The schema allows it (both fields are valid individually), but the business rules reject it.

### Stage 3: Provider-Specific Validation

The third stage delegates to provider adapters for cloud-specific checks. Each provider implements the `CloudProviderValidator` interface:

```go
type CloudProviderValidator interface {
    ValidateCredentials(ctx context.Context, config *Config) []*errors.StructuredError
    ValidateConfiguration(ctx context.Context, config *Config) []*errors.StructuredError
    ValidateConnectivity(ctx context.Context, config *Config) []*errors.StructuredError
    GetRequiredFields() []string
}
```

Provider validators check:

- **OpenStack**: Auth URL format (must end with /v3/), application credential UUID format, floating network ID validity
- **AWS**: Access key format (20 uppercase alphanumeric), region validity, VPC configuration
- **vSphere**: Datacenter and cluster names, resource pool configuration

Provider validation runs after business rules because there's no point checking OpenStack credentials if the cluster name is invalid. Early stages filter out simple errors before expensive provider checks run.

The provider validators live in separate files: `openstack_validator.go`, `aws_validator.go`, `vsphere_validator.go`. This isolation means you can update OpenStack validation without touching AWS code.

### Stage 4: Connectivity Validation

The fourth stage performs network checks to verify cloud provider accessibility. This includes:

- DNS resolution for auth URLs and API endpoints
- HTTP connectivity tests (HEAD requests to verify endpoints respond)
- Credential format validation (UUID structure, key length)
- Security checks (detecting plaintext secrets that should be encrypted)

Connectivity validation runs last because it's the slowest—it makes network requests. If earlier stages find errors, connectivity checks are skipped entirely. No point testing network connectivity if the configuration is structurally broken.

Connectivity errors are reported as warnings, not errors. A network timeout doesn't necessarily mean the configuration is wrong—it might mean your laptop isn't connected to the VPN. You can proceed with warnings, but errors block deployment.

The connectivity validator lives in `internal/config/connectivity_validator.go` and uses a 10-second timeout for HTTP requests.

## Error Handling and Suggestions

When validation fails, openCenter provides structured errors with context and suggestions. Each error includes:

- **Type**: The error category (validation, credential, cloud, network)
- **Field**: The configuration path that failed (e.g., `opencenter.cluster.kubernetes.master_count`)
- **Message**: What went wrong
- **Suggestions**: How to fix it

The suggestion engine (`internal/config/suggestions.go`) maintains a database of field-specific guidance. When you get an error on `subnet_pods`, it suggests valid CIDR formats and warns about subnet conflicts.

Suggestions are context-aware. If a field name contains "password" or "secret", the suggestion engine recommends SOPS encryption. If it contains "url", it suggests checking protocol and endpoint accessibility.

This approach reduces the "what do I do now?" problem. Instead of cryptic error messages, you get actionable guidance.

## Why Staged Validation?

The staged approach serves three purposes:

### Fast Feedback for Common Errors

Schema validation catches 80% of errors in milliseconds. You don't wait for network timeouts to learn you misspelled `cluster_name`. Early stages provide instant feedback on simple mistakes.

### Expensive Checks Only When Needed

Provider connectivity checks make HTTP requests and DNS lookups. These take seconds. By running them last, we avoid slow validation when the configuration has basic errors.

If schema validation fails, business rules don't run. If business rules fail, provider validation doesn't run. Each stage acts as a gate, preventing unnecessary work.

### Clear Error Categorization

Staged validation produces categorized errors. A schema error means you have a structural problem. A provider error means cloud-specific configuration is wrong. A connectivity warning means network access might be an issue.

This categorization helps with debugging. If you see only connectivity warnings, you know the configuration is structurally sound—you just need to check network access or credentials.

## Validation Result Structure

The validation pipeline returns a `ConfigValidationResult`:

```go
type ConfigValidationResult struct {
    Valid    bool
    Errors   []*ConfigValidationError
    Warnings []*ConfigValidationError
}
```

Errors block deployment. Warnings don't. The distinction matters because some checks (like connectivity) can fail for environmental reasons unrelated to configuration correctness.

Each `ConfigValidationError` contains:

- `Type`: Error category (validation, credential, cloud, network)
- `Field`: Configuration path
- `Message`: Human-readable description
- `Suggestions`: Array of fix recommendations

The CLI formats these errors with color coding and indentation for readability. Errors appear in red, warnings in yellow, suggestions in cyan.

## Auto-Repair Mode

The validator supports an auto-repair mode (currently experimental). When enabled, it attempts to fix certain classes of errors automatically:

- Adding missing required fields with sensible defaults
- Correcting common typos in field names
- Normalizing formats (trimming whitespace, fixing case)

Auto-repair is conservative. It only fixes errors where the intent is unambiguous. If there are multiple valid fixes, it reports the error and suggests options rather than guessing.

Auto-repair mode is disabled by default. Enable it with `--auto-repair` flag on validation commands.

## Validation Performance

Validation is designed to be fast enough for interactive use. Typical validation times:

- Schema validation: < 10ms
- Business rules: < 50ms
- Provider validation: < 100ms
- Connectivity checks: 1-5 seconds (network dependent)

Total validation time is usually under 5 seconds. The staged approach means you get feedback on simple errors in under 100ms—fast enough that validation feels instant.

For CI/CD pipelines, you can skip connectivity checks with `--skip-connectivity` to reduce validation time to under 200ms. This is useful when running validation in environments without cloud provider access.

## Integration with Other Systems

### Schema Generation

The JSON schema is generated from Go struct tags. When you add a field to `internal/config/types_*.go`, you annotate it:

```go
type ClusterConfig struct {
    ClusterName string `yaml:"cluster_name" json:"cluster_name" jsonschema:"required,description=Cluster name"`
}
```

Running `mise run schema` regenerates `schema/config-schema.json`. The validator loads this schema at runtime.

This approach keeps validation rules close to the code. You don't maintain separate schema files that drift from the implementation.

### Error Aggregation

The validator uses `internal/util/errors.ValidationAggregator` to collect errors across stages. This prevents the "fix one error, run again, find another error" loop. All errors are reported together.

The aggregator groups related errors. If you have three subnet configuration errors, they're grouped under a "Network Configuration" heading rather than scattered through the output.

### Provider Adapters

Provider validators are registered in a map:

```go
validator.cloudValidators["openstack"] = NewOpenStackValidator()
validator.cloudValidators["aws"] = NewAWSValidator()
validator.cloudValidators["vsphere"] = NewVSphereValidator()
```

Adding a new provider means implementing the `CloudProviderValidator` interface and registering it. The core validation pipeline doesn't change.

## Trade-offs and Limitations

### Strictness vs. Flexibility

openCenter validation is strict. Some configurations that "might work" are rejected. For example, a Kubernetes version without patch number (1.31 instead of 1.31.4) fails validation even though Kubernetes might accept it.

This strictness prevents ambiguity. It's better to reject a questionable configuration than to deploy something that fails unpredictably.

### Validation vs. Runtime Checks

Validation catches configuration errors, not runtime failures. It can't predict that your OpenStack quota is exhausted or that a specific VM flavor is unavailable. Those failures happen during provisioning.

The validation pipeline checks what's knowable from configuration alone. Runtime failures require different handling (retry logic, quota checks, resource availability probes).

### Network Dependency

Connectivity validation requires network access to cloud providers. This creates a dependency on network availability and VPN connectivity.

To handle this, connectivity checks are warnings, not errors. You can proceed without connectivity validation if you're confident the configuration is correct. This is common in CI/CD environments where the build system doesn't have cloud provider access.

### Performance vs. Completeness

Comprehensive validation would make API calls to verify every resource exists (networks, flavors, images). This would be slow and require credentials for validation.

openCenter balances completeness with performance. It validates format and structure without making expensive API calls. Preflight checks (run during bootstrap) perform the expensive verification.

## Common Validation Patterns

### Mutual Exclusivity

Network plugins are mutually exclusive—only one can be enabled. The validator checks this in business rules:

```go
if enabledCount > 1 {
    aggregator.AddError(errors.CreateValidationError(
        "opencenter.cluster.kubernetes.network_plugin",
        fmt.Sprintf("only one network plugin can be enabled, found: %s", 
            strings.Join(enabledPlugins, ", ")),
        "Choose one network plugin and disable others",
    ))
}
```

This pattern applies to any mutually exclusive options.

### Conditional Requirements

Some fields are required only when others are set. For example, if you enable Windows workers, `worker_count_windows` must be positive:

```go
if config.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
    if config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
        aggregator.AddError(errors.CreateValidationError(
            "opencenter.cluster.kubernetes.windows_workers.enabled",
            "Windows workers enabled but worker_count_windows is 0",
            "Set worker_count_windows to positive number",
        ))
    }
}
```

This pattern handles dependencies between fields.

### Format Validation

Fields like CIDR ranges, UUIDs, and URLs have specific formats. The validator checks these with helper functions:

```go
func (v *EnhancedConfigValidator) isValidCIDR(cidr string) bool {
    _, _, err := net.ParseCIDR(cidr)
    return err == nil
}
```

Format validation catches typos and malformed values before they reach infrastructure code.

## Future Directions

The validation pipeline is designed for extension. Planned improvements include:

- **Async validation**: Run connectivity checks in parallel to reduce total validation time
- **Caching**: Cache provider validation results to avoid repeated checks
- **Custom validators**: Plugin system for user-defined validation rules
- **Validation profiles**: Different strictness levels (strict, normal, permissive)

The staged architecture supports these additions without requiring changes to existing validators.

## Related Documentation

- [Configuration System](configuration-system.md) - How configuration loading and defaults work
- [Error Handling](error-handling.md) - Structured error system used by validators
- [Provider Architecture](../providers/README.md) - Provider-specific validation details
- [CLI Commands Reference](../reference/cli-commands.md) - Validation command options

## Conclusion

The validation pipeline prevents configuration errors from reaching production infrastructure. By catching mistakes early and providing actionable suggestions, it reduces deployment failures and debugging time.

The staged approach balances speed with thoroughness. Simple errors are caught instantly. Complex errors are detected before expensive operations run. The result is fast feedback on common mistakes and comprehensive checking when needed.

Validation isn't perfect—it can't predict all runtime failures. But it eliminates entire classes of configuration errors, making deployments more reliable and reducing the "it worked on my machine" problem.
