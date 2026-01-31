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
	"fmt"
	"sync"
)

// Registry manages validator registration and lookup.
type Registry struct {
	mu         sync.RWMutex
	validators map[string]Validator
}

// NewRegistry creates a new validator registry.
func NewRegistry() *Registry {
	return &Registry{
		validators: make(map[string]Validator),
	}
}

// Register registers a validator with the registry.
// Returns an error if a validator with the same name is already registered.
func (r *Registry) Register(validator Validator) error {
	if validator == nil {
		return fmt.Errorf("validator cannot be nil")
	}

	name := validator.Name()
	if name == "" {
		return fmt.Errorf("validator name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; exists {
		return fmt.Errorf("validator %q is already registered", name)
	}

	r.validators[name] = validator
	return nil
}

// MustRegister registers a validator and panics if registration fails.
// This is useful for registering validators during initialization.
func (r *Registry) MustRegister(validator Validator) {
	if err := r.Register(validator); err != nil {
		panic(err)
	}
}

// Unregister removes a validator from the registry.
// Returns an error if the validator is not found.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.validators[name]; !exists {
		return fmt.Errorf("validator %q not found", name)
	}

	delete(r.validators, name)
	return nil
}

// Get retrieves a validator by name.
// Returns nil if the validator is not found.
func (r *Registry) Get(name string) Validator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.validators[name]
}

// Has checks if a validator is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.validators[name]
	return exists
}

// List returns a list of all registered validator names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.validators))
	for name := range r.validators {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered validators.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.validators)
}

// Clear removes all validators from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validators = make(map[string]Validator)
}

// Clone creates a copy of the registry with all registered validators.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := NewRegistry()
	for name, validator := range r.validators {
		clone.validators[name] = validator
	}
	return clone
}

// globalRegistry is the default global validator registry.
var globalRegistry = NewRegistry()

// Register registers a validator with the global registry.
func Register(validator Validator) error {
	return globalRegistry.Register(validator)
}

// MustRegister registers a validator with the global registry and panics on error.
func MustRegister(validator Validator) {
	globalRegistry.MustRegister(validator)
}

// Unregister removes a validator from the global registry.
func Unregister(name string) error {
	return globalRegistry.Unregister(name)
}

// Get retrieves a validator from the global registry.
func Get(name string) Validator {
	return globalRegistry.Get(name)
}

// Has checks if a validator is registered in the global registry.
func Has(name string) bool {
	return globalRegistry.Has(name)
}

// List returns all validator names from the global registry.
func List() []string {
	return globalRegistry.List()
}

// Count returns the number of validators in the global registry.
func Count() int {
	return globalRegistry.Count()
}

// Clear removes all validators from the global registry.
func Clear() {
	globalRegistry.Clear()
}

// GlobalRegistry returns the global validator registry.
func GlobalRegistry() *Registry {
	return globalRegistry
}
