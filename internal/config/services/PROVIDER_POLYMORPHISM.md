# Service Provider Polymorphism

## Overview

Service provider polymorphism enables services to automatically adapt to the infrastructure provider, selecting compatible service providers (e.g., DNS providers for cert-manager, storage backends for loki/tempo/velero) based on the underlying infrastructure.

## Architecture

### Components

1. **ServiceProviderRegistry** (`provider_registry.go`)
   - Maintains compatibility matrix between infrastructure and service providers
   - Manages default provider selections per infrastructure type
   - Handles special cases (e.g., OpenStack with/without Designate)

2. **ServiceProviderValidator** (`provider_validator.go`)
   - Validates service provider compatibility with infrastructure
   - Auto-selects providers when not explicitly configured
   - Applies default providers during configuration hydration

### Supported Services

#### cert-manager (DNS Providers)
- **AWS**: route53 (default), cloudflare
- **OpenStack**: designate (default if available), cloudflare (fallback)
- **GCP**: clouddns (default), cloudflare
- **Azure**: azuredns (default), cloudflare
- **BareMetal/VSphere**: cloudflare (default)

#### loki (Storage Providers)
- **AWS**: s3 (default)
- **OpenStack**: swift (default), s3
- **GCP**: gcs (default), s3
- **Azure**: azure (default), s3

#### tempo (Storage Providers)
- **AWS**: s3 (default)
- **OpenStack**: swift (default), s3
- **GCP**: gcs (default), s3
- **Azure**: azure (default), s3

#### velero (Storage Providers)
- **AWS**: s3 (default)
- **OpenStack**: swift (default), s3
- **GCP**: gcs (default)
- **Azure**: azure (default)

## Usage

### Auto-Selection

When a service is enabled without an explicit provider configuration, the system automatically selects the best provider based on infrastructure:

```yaml
opencenter:
  infrastructure:
    provider: aws
  services:
    cert-manager:
      enabled: true
      # dns_provider is auto-selected as "route53"
    loki:
      enabled: true
      # loki_storage_type is auto-selected as "s3"
```

### Explicit Configuration

Users can override auto-selection by explicitly configuring providers:

```yaml
opencenter:
  infrastructure:
    provider: aws
  services:
    cert-manager:
      enabled: true
      dns_provider: cloudflare  # Override default route53
```

### OpenStack Special Case

For OpenStack, cert-manager DNS provider selection depends on Designate availability:

```yaml
opencenter:
  infrastructure:
    provider: openstack
    networking:
      use_designate: true  # Enables Designate
  services:
    cert-manager:
      enabled: true
      # dns_provider is auto-selected as "designate"
```

If `use_designate: false`, the system falls back to cloudflare.

## Validation

The validator performs two types of checks:

1. **Compatibility Validation**: Ensures explicitly configured providers are compatible with infrastructure
2. **Auto-Selection**: Automatically selects providers when not configured

### Error Messages

When an incompatible provider is configured, the validator provides clear error messages with suggestions:

```
cert-manager: route53 provider is not compatible with openstack infrastructure: Route53 is AWS-specific. Compatible providers: [designate cloudflare]
```

## Integration Points

### Configuration Loading

Service provider validation and auto-selection should be integrated into the configuration loading pipeline:

1. Load YAML configuration
2. Normalize and resolve references
3. **Apply default service providers** (hydration phase)
4. **Validate service provider compatibility** (validation phase)
5. Freeze configuration

### Example Integration

```go
// In config loader or hydrator
validator := services.NewServiceProviderValidator()

// Apply defaults during hydration
err := validator.ApplyDefaultProviders(
    config.OpenCenter.Services,
    config.OpenCenter.Infrastructure.Provider,
    config.OpenCenter.Infrastructure.Networking.UseDesignate,
    config.OpenCenter.Meta.Name,
)

// Validate during validation phase
errors := validator.ValidateServiceProviders(
    config.OpenCenter.Services,
    config.OpenCenter.Infrastructure.Provider,
    config.OpenCenter.Infrastructure.Networking.UseDesignate,
    config.OpenCenter.Meta.Name,
)
```

## Testing

### Unit Tests

- `provider_registry_test.go`: Tests registry functionality, compatibility matrix, and auto-selection
- `provider_validator_test.go`: Tests validation logic and default application

### Test Coverage

- Default provider selection for all infrastructure types
- Compatibility validation (valid and invalid combinations)
- Auto-selection behavior
- Explicit configuration override
- OpenStack Designate availability handling
- Multiple service validation
- Error message formatting

## Requirements Satisfied

This implementation satisfies the following requirements from the v2 cluster configuration schema:

- **8.1**: Service provider polymorphism based on infrastructure
- **8.2**: AWS → route53 for cert-manager
- **8.3**: OpenStack + Designate → designate for cert-manager
- **8.4**: OpenStack - Designate → cloudflare for cert-manager
- **8.5**: User override of auto-selected providers
- **8.6**: Validation of service provider compatibility
- **8.7**: Rejection of incompatible service-provider combinations

## Future Enhancements

1. **Dynamic Provider Discovery**: Query infrastructure to detect available services (e.g., check if Designate is actually available in OpenStack)
2. **Provider-Specific Configuration Validation**: Validate required fields for each provider (e.g., AWS credentials for route53)
3. **Additional Services**: Extend polymorphism to other services (e.g., ingress controllers, CSI drivers)
4. **Cost Optimization**: Consider cost implications when auto-selecting providers
5. **Performance Metrics**: Track provider selection decisions for analytics
