# Circuit Breaker Integration Example

This document demonstrates how circuit breakers are integrated into cloud provider API clients.

## Overview

Circuit breakers prevent cascading failures by failing fast when a service is down. They wrap API calls and track failure rates, automatically opening the circuit after a threshold of consecutive failures.

## Integration Pattern

### 1. Add Circuit Breaker to Client

```go
type Client struct {
    client         *gophercloud.ServiceClient
    config         *config.Config
    retryHandler   resilience.RetryHandler
    circuitBreaker resilience.CircuitBreaker  // Add circuit breaker
}
```

### 2. Initialize Circuit Breaker

```go
func NewClient(cfg *config.Config) (*Client, error) {
    // ... client initialization ...
    
    // Create retry handler
    retryHandler := resilience.NewRetryHandler(resilience.DefaultConfig)
    
    // Create circuit breaker with default config
    circuitBreaker := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig)
    
    return &Client{
        client:         client,
        config:         cfg,
        retryHandler:   retryHandler,
        circuitBreaker: circuitBreaker,
    }, nil
}
```

### 3. Wrap API Calls

Wrap API calls with circuit breaker first, then retry handler:

```go
func (c *Client) GetResource(ctx context.Context, id string) (*Resource, error) {
    var result *Resource
    
    // Circuit breaker wraps retry handler
    err := c.circuitBreaker.Call(ctx, func() error {
        // Retry handler wraps actual API call
        return c.retryHandler.Do(ctx, func() error {
            // Actual API call
            result, err := api.Get(c.client, id).Extract()
            if err != nil {
                return fmt.Errorf("failed to get resource: %w", err)
            }
            return nil
        })
    })
    
    return result, err
}
```

## State Transitions

The circuit breaker has three states:

1. **Closed** (Normal): All requests pass through
2. **Open** (Failing): Requests fail immediately without calling the API
3. **Half-Open** (Testing): Limited requests allowed to test recovery

### Transition Rules

- **Closed → Open**: After 5 consecutive failures
- **Open → Half-Open**: After 60 seconds timeout
- **Half-Open → Closed**: After 2 consecutive successes
- **Half-Open → Open**: On any failure

## Configuration

Default configuration:

```go
DefaultCircuitBreakerConfig = CircuitBreakerConfig{
    FailureThreshold:  5,           // Open after 5 failures
    SuccessThreshold:  2,           // Close after 2 successes
    Timeout:           60 * time.Second,  // Wait 60s before half-open
    HalfOpenRequests:  1,           // Allow 1 concurrent request in half-open
}
```

Custom configuration:

```go
config := resilience.CircuitBreakerConfig{
    FailureThreshold:  3,           // More sensitive
    SuccessThreshold:  1,           // Faster recovery
    Timeout:           30 * time.Second,  // Shorter timeout
    HalfOpenRequests:  2,           // More test requests
}
circuitBreaker := resilience.NewCircuitBreaker(config)
```

## Benefits

1. **Fail Fast**: When a service is down, requests fail immediately instead of waiting for timeouts
2. **Prevent Cascading Failures**: Stops overwhelming a failing service with requests
3. **Automatic Recovery**: Tests service health and automatically recovers when service is back
4. **Resource Protection**: Reduces wasted resources on doomed requests

## Integrated Clients

The following clients have circuit breaker integration:

- **Barbican Client** (`internal/barbican/client.go`): OpenStack secrets management
  - All API methods wrapped: GetSecret, PutSecret, ListSecrets, DescribeSecret, DeleteSecret

## Future Integrations

Additional cloud provider clients to integrate:

- OpenStack Compute (Nova)
- OpenStack Networking (Neutron)
- OpenStack Block Storage (Cinder)
- AWS EC2
- AWS S3
- Terraform/OpenTofu operations

## Monitoring

Circuit breaker state can be monitored via:

```go
state := client.circuitBreaker.GetState()
switch state {
case resilience.StateClosed:
    // Normal operation
case resilience.StateOpen:
    // Service is down, failing fast
case resilience.StateHalfOpen:
    // Testing recovery
}
```

Future: Export circuit breaker state as Prometheus metrics (see task 12.3).

## Testing

Circuit breaker behavior is validated through:

1. **Unit Tests**: `internal/resilience/circuit_breaker_test.go`
2. **Property Tests**: `internal/resilience/circuit_breaker_property_test.go`
3. **Integration Tests**: Verify circuit breaker works with actual API clients

## References

- Design Document: `.kiro/specs/security-and-operational-remediation/design.md`
- Requirements: 7.5, 7.6
- Property 11: Circuit Breaker State Transitions
