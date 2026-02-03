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

package validation

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ValidationEngine manages validator registration and execution.
//
// The engine provides:
//   - Thread-safe validator registration and lookup
//   - Single validator execution via Validate()
//   - Multiple validator execution via ValidateAll()
//   - Parallel validation via ValidateParallel()
//   - Automatic suggestion generation
//   - Always-run security validators that cannot be bypassed
//   - Validation result caching with automatic expiration
//
// Example usage:
//
//	engine := validation.NewValidationEngine()
//	engine.Register(myValidator)
//	result, err := engine.Validate(ctx, "my-validator", value)
type ValidationEngine struct {
	registry           *Registry
	suggestionEngine   *SuggestionEngine
	securityValidators []Validator
	cache              *ValidationCache
	mu                 sync.RWMutex
}

// NewValidationEngine creates a new validation engine.
//
// The engine is initialized with:
//   - Empty validator registry
//   - Default suggestion engine with typo detection
//   - Empty security validators list
//   - Validation cache with 5-minute TTL
//
// Returns:
//   - *ValidationEngine: New validation engine instance
func NewValidationEngine() *ValidationEngine {
	return &ValidationEngine{
		registry:           NewRegistry(),
		suggestionEngine:   NewSuggestionEngine(),
		securityValidators: make([]Validator, 0),
		cache:              NewValidationCache(5 * time.Minute),
	}
}

// NewValidationEngineWithCache creates a new validation engine with custom cache TTL.
//
// Parameters:
//   - cacheTTL: Time-to-live for cache entries (0 disables caching)
//
// Returns:
//   - *ValidationEngine: New validation engine instance
//
// Example:
//
//	// Create engine with 10-minute cache
//	engine := validation.NewValidationEngineWithCache(10 * time.Minute)
//	
//	// Create engine with caching disabled
//	engine := validation.NewValidationEngineWithCache(0)
func NewValidationEngineWithCache(cacheTTL time.Duration) *ValidationEngine {
	return &ValidationEngine{
		registry:           NewRegistry(),
		suggestionEngine:   NewSuggestionEngine(),
		securityValidators: make([]Validator, 0),
		cache:              NewValidationCache(cacheTTL),
	}
}

// Register registers a validator with the engine.
//
// The validator name must be unique. Attempting to register a validator
// with a duplicate name returns an error.
//
// Parameters:
//   - validator: Validator to register (must not be nil)
//
// Returns:
//   - error: Registration error (nil on success)
//
// Example:
//
//	validator := validation.NewValidatorFunc("cluster-name", validateClusterName)
//	err := engine.Register(validator)
func (e *ValidationEngine) Register(validator Validator) error {
	return e.registry.Register(validator)
}

// MustRegister registers a validator and panics on error.
//
// This is useful for registering validators during initialization where
// registration failure should be fatal.
//
// Parameters:
//   - validator: Validator to register
//
// Panics:
//   - If registration fails
//
// Example:
//
//	engine.MustRegister(myValidator) // Panics if registration fails
func (e *ValidationEngine) MustRegister(validator Validator) {
	e.registry.MustRegister(validator)
}

// RegisterSecurityValidator registers a security validator that always runs.
//
// Security validators are executed automatically in all validation operations
// and cannot be bypassed. This ensures security checks are always applied.
//
// Parameters:
//   - validator: Security validator to register
//
// Returns:
//   - error: Registration error (nil on success)
//
// Example:
//
//	secValidator := validators.NewSecurityValidator()
//	err := engine.RegisterSecurityValidator(secValidator)
func (e *ValidationEngine) RegisterSecurityValidator(validator Validator) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Also register in the main registry for explicit access
	if err := e.registry.Register(validator); err != nil {
		return err
	}

	// Add to security validators list
	e.securityValidators = append(e.securityValidators, validator)
	return nil
}

// MustRegisterSecurityValidator registers a security validator and panics on error.
//
// Parameters:
//   - validator: Security validator to register
//
// Panics:
//   - If registration fails
func (e *ValidationEngine) MustRegisterSecurityValidator(validator Validator) {
	if err := e.RegisterSecurityValidator(validator); err != nil {
		panic(fmt.Sprintf("failed to register security validator: %v", err))
	}
}

// Unregister removes a validator from the engine.
//
// Parameters:
//   - name: Validator name to remove
//
// Returns:
//   - error: Error if validator not found
func (e *ValidationEngine) Unregister(name string) error {
	return e.registry.Unregister(name)
}

// Has checks if a validator is registered.
//
// Parameters:
//   - name: Validator name to check
//
// Returns:
//   - bool: True if validator exists
func (e *ValidationEngine) Has(name string) bool {
	return e.registry.Has(name)
}

// List returns names of all registered validators.
//
// Returns:
//   - []string: Validator names (empty if none registered)
func (e *ValidationEngine) List() []string {
	return e.registry.List()
}

// ListSecurityValidators returns names of all registered security validators.
//
// Returns:
//   - []string: Security validator names (empty if none registered)
func (e *ValidationEngine) ListSecurityValidators() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.securityValidators))
	for _, validator := range e.securityValidators {
		names = append(names, validator.Name())
	}
	return names
}

// Validate executes a specific validator.
//
// The validator must be registered before calling this method.
// The context can be used for cancellation and passing metadata.
//
// Security validators are automatically executed before the requested validator
// to ensure security checks cannot be bypassed.
//
// Caching: Results are cached based on validator name and data hash. Cached
// results are returned if available and not expired, avoiding redundant validation.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - name: Validator name to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Validation result with errors/warnings
//   - error: Execution error (not validation failure)
//
// Example:
//
//	result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
//	if err != nil {
//	    return err // Execution error
//	}
//	if !result.Valid {
//	    // Handle validation errors
//	}
func (e *ValidationEngine) Validate(ctx context.Context, name string, value interface{}) (*ValidationResult, error) {
	// Check cache first
	if cached := e.cache.Get(name, value); cached != nil {
		return cached, nil
	}

	// First, run all security validators (cannot be bypassed)
	aggregated := NewValidationResult()
	
	e.mu.RLock()
	securityValidators := make([]Validator, len(e.securityValidators))
	copy(securityValidators, e.securityValidators)
	e.mu.RUnlock()

	for _, secValidator := range securityValidators {
		secResult, err := secValidator.Validate(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("security validator %q failed: %w", secValidator.Name(), err)
		}
		aggregated.Merge(secResult)
	}

	// If security validation failed, return immediately
	if !aggregated.Valid {
		e.suggestionEngine.EnhanceResult(aggregated, nil)
		return aggregated, nil
	}

	// Now run the requested validator
	validator := e.registry.Get(name)
	if validator == nil {
		return nil, fmt.Errorf("validator %q not found", name)
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		return nil, err
	}

	// Merge with security validation results
	aggregated.Merge(result)

	// Enhance result with suggestions
	e.suggestionEngine.EnhanceResult(aggregated, nil)

	// Cache the result
	e.cache.Set(name, value, aggregated, 0)

	return aggregated, nil
}

// ValidateWithOptions executes a validator with custom options.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - name: Validator name to execute
//   - value: Value to validate
//   - opts: Validation options (nil for defaults)
//
// Returns:
//   - *ValidationResult: Validation result
//   - error: Execution error
func (e *ValidationEngine) ValidateWithOptions(ctx context.Context, name string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	if opts == nil {
		opts = DefaultValidationOptions()
	}

	result, err := e.Validate(ctx, name, value)
	if err != nil {
		return nil, err
	}

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		result.Warnings = nil
	}

	return result, nil
}

// ValidateAll executes multiple validators sequentially.
//
// All validators are executed even if some fail. The results are aggregated
// into a single ValidationResult.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error (not validation failure)
//
// Example:
//
//	result, err := engine.ValidateAll(ctx, []string{"validator1", "validator2"}, value)
func (e *ValidationEngine) ValidateAll(ctx context.Context, names []string, value interface{}) (*ValidationResult, error) {
	return e.ValidateAllWithOptions(ctx, names, value, nil)
}

// ValidateAllWithOptions executes multiple validators with custom options.
//
// Security validators are automatically executed first to ensure security
// checks cannot be bypassed.
//
// Validators are sorted by priority before execution, with lower priority
// values running first. This ensures fast validators (format checks) run
// before slow validators (network checks, file I/O).
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//   - opts: Validation options (nil for defaults)
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error
func (e *ValidationEngine) ValidateAllWithOptions(ctx context.Context, names []string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	if opts == nil {
		opts = DefaultValidationOptions()
	}

	aggregated := NewValidationResult()

	// First, run all security validators (cannot be bypassed)
	e.mu.RLock()
	securityValidators := make([]Validator, len(e.securityValidators))
	copy(securityValidators, e.securityValidators)
	e.mu.RUnlock()

	for _, secValidator := range securityValidators {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		secResult, err := secValidator.Validate(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("security validator %q failed: %w", secValidator.Name(), err)
		}

		// Merge results
		aggregated.Merge(secResult)

		// Stop on first error if requested
		if opts.StopOnFirstError && !secResult.Valid {
			break
		}
	}

	// If security validation failed and stop on first error, return immediately
	if opts.StopOnFirstError && !aggregated.Valid {
		if !opts.IncludeWarnings {
			aggregated.Warnings = nil
		}
		return aggregated, nil
	}

	// Collect validators and sort by priority
	validators := make([]Validator, 0, len(names))
	for _, name := range names {
		validator := e.registry.Get(name)
		if validator == nil {
			return nil, fmt.Errorf("validator %q not found", name)
		}
		validators = append(validators, validator)
	}

	// Sort validators by priority (lower values first)
	sortValidatorsByPriority(validators)

	// Now run the requested validators in priority order
	for _, validator := range validators {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := validator.Validate(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("validator %q failed: %w", validator.Name(), err)
		}

		// Merge results
		aggregated.Merge(result)

		// Stop on first error if requested
		if opts.StopOnFirstError && !result.Valid {
			break
		}
	}

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		aggregated.Warnings = nil
	}

	return aggregated, nil
}

// ValidateParallel executes multiple validators in parallel.
//
// All validators run concurrently using goroutines. This is faster than
// ValidateAll for independent validators but uses more resources.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error (not validation failure)
//
// Example:
//
//	result, err := engine.ValidateParallel(ctx, []string{"validator1", "validator2"}, value)
func (e *ValidationEngine) ValidateParallel(ctx context.Context, names []string, value interface{}) (*ValidationResult, error) {
	return e.ValidateParallelWithOptions(ctx, names, value, nil)
}

// ValidateParallelWithOptions executes multiple validators in parallel with options.
//
// Security validators are executed first sequentially to ensure security checks
// cannot be bypassed, then the requested validators run in parallel.
//
// Note: Validators are sorted by priority before parallel execution. While they
// run concurrently, validators with the same priority level are grouped together.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//   - opts: Validation options (nil for defaults)
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error
func (e *ValidationEngine) ValidateParallelWithOptions(ctx context.Context, names []string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	if opts == nil {
		opts = DefaultValidationOptions()
	}

	aggregated := NewValidationResult()

	// First, run all security validators sequentially (cannot be bypassed)
	e.mu.RLock()
	securityValidators := make([]Validator, len(e.securityValidators))
	copy(securityValidators, e.securityValidators)
	e.mu.RUnlock()

	for _, secValidator := range securityValidators {
		secResult, err := secValidator.Validate(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("security validator %q failed: %w", secValidator.Name(), err)
		}
		aggregated.Merge(secResult)
	}

	// If security validation failed, return immediately
	if !aggregated.Valid {
		if !opts.IncludeWarnings {
			aggregated.Warnings = nil
		}
		return aggregated, nil
	}

	// Collect validators and sort by priority
	validators := make([]Validator, 0, len(names))
	for _, name := range names {
		validator := e.registry.Get(name)
		if validator == nil {
			return nil, fmt.Errorf("validator %q not found", name)
		}
		validators = append(validators, validator)
	}

	// Sort validators by priority (lower values first)
	sortValidatorsByPriority(validators)

	// Create channels for results and errors
	type validationJob struct {
		name   string
		result *ValidationResult
		err    error
	}

	jobs := make(chan validationJob, len(validators))
	var wg sync.WaitGroup

	// Launch validators in parallel (already sorted by priority)
	for _, validator := range validators {
		wg.Add(1)
		go func(v Validator) {
			defer wg.Done()

			result, err := v.Validate(ctx, value)
			jobs <- validationJob{
				name:   v.Name(),
				result: result,
				err:    err,
			}
		}(validator)
	}

	// Wait for all validators to complete
	go func() {
		wg.Wait()
		close(jobs)
	}()

	// Aggregate results
	for job := range jobs {
		if job.err != nil {
			return nil, fmt.Errorf("validator %q failed: %w", job.name, job.err)
		}
		aggregated.Merge(job.result)
	}

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		aggregated.Warnings = nil
	}

	return aggregated, nil
}

// AddSuggestionRule adds a custom suggestion rule to the engine.
//
// Parameters:
//   - rule: Suggestion rule to add
func (e *ValidationEngine) AddSuggestionRule(rule SuggestionRule) {
	e.suggestionEngine.AddRule(rule)
}

// GetRegistry returns the validator registry.
//
// Returns:
//   - *Registry: Validator registry
func (e *ValidationEngine) GetRegistry() *Registry {
	return e.registry
}

// GetSuggestionEngine returns the suggestion engine.
//
// Returns:
//   - *SuggestionEngine: Suggestion engine
func (e *ValidationEngine) GetSuggestionEngine() *SuggestionEngine {
	return e.suggestionEngine
}

// GetCache returns the validation cache.
//
// Returns:
//   - *ValidationCache: Validation cache
func (e *ValidationEngine) GetCache() *ValidationCache {
	return e.cache
}

// InvalidateCache invalidates cached results for a specific validator and data.
//
// Parameters:
//   - validatorName: Name of the validator
//   - data: Data to invalidate
//
// Example:
//
//	// Data changed - invalidate cache
//	engine.InvalidateCache("cluster-name", oldData)
func (e *ValidationEngine) InvalidateCache(validatorName string, data interface{}) {
	e.cache.Invalidate(validatorName, data)
}

// InvalidateAllCache invalidates all cached results for a validator.
//
// Parameters:
//   - validatorName: Name of the validator
//
// Example:
//
//	// Validator logic changed - invalidate all entries
//	engine.InvalidateAllCache("cluster-name")
func (e *ValidationEngine) InvalidateAllCache(validatorName string) {
	e.cache.InvalidateAll(validatorName)
}

// ClearCache removes all cached validation results.
//
// Example:
//
//	engine.ClearCache() // Clear all cached results
func (e *ValidationEngine) ClearCache() {
	e.cache.Clear()
}

// CleanExpiredCache removes expired entries from the cache.
//
// Returns:
//   - int: Number of entries removed
//
// Example:
//
//	removed := engine.CleanExpiredCache()
//	log.Printf("Cleaned %d expired cache entries", removed)
func (e *ValidationEngine) CleanExpiredCache() int {
	return e.cache.CleanExpired()
}

// CacheStats returns cache statistics.
//
// Returns:
//   - CacheStats: Cache statistics
func (e *ValidationEngine) CacheStats() CacheStats {
	return e.cache.Stats()
}

// defaultEngine is the global default validation engine.
var defaultEngine = NewValidationEngine()

// DefaultEngine returns the global default validation engine.
//
// This is useful for simple use cases where a single global engine is sufficient.
//
// Returns:
//   - *ValidationEngine: Global validation engine
func DefaultEngine() *ValidationEngine {
	return defaultEngine
}

// Validate validates a value using the default engine.
//
// This is a convenience function for simple validation without creating an engine.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - name: Validator name to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Validation result
//   - error: Execution error
func Validate(ctx context.Context, name string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.Validate(ctx, name, value)
}

// ValidateAll validates a value using multiple validators from the default engine.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error
func ValidateAll(ctx context.Context, names []string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.ValidateAll(ctx, names, value)
}

// ValidateParallel validates a value using multiple validators in parallel from the default engine.
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - names: Validator names to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Aggregated validation result
//   - error: Execution error
func ValidateParallel(ctx context.Context, names []string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.ValidateParallel(ctx, names, value)
}

// sortValidatorsByPriority sorts validators by priority (lower values first).
//
// This ensures fast validators (format checks, simple rules) run before
// slow validators (network checks, file I/O).
//
// Parameters:
//   - validators: Slice of validators to sort (modified in place)
func sortValidatorsByPriority(validators []Validator) {
	// Use a simple insertion sort since validator lists are typically small
	for i := 1; i < len(validators); i++ {
		key := validators[i]
		keyPriority := key.Priority()
		j := i - 1

		// Move validators with higher priority values to the right
		for j >= 0 && validators[j].Priority() > keyPriority {
			validators[j+1] = validators[j]
			j--
		}
		validators[j+1] = key
	}
}
