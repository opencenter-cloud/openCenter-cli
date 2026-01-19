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

package di

import (
	"fmt"
	"reflect"
	"sync"
)

// Container manages component lifecycle and dependencies.
// Requirements: 18.1, 18.2, 19.1, 19.2, 19.3, 19.4
type Container interface {
	// Register registers a constructor function for a named component
	Register(name string, constructor interface{}) error

	// Resolve resolves a component by name and returns it
	Resolve(name string) (interface{}, error)

	// ResolveAs resolves a component and assigns it to the target pointer
	ResolveAs(name string, target interface{}) error

	// Singleton registers a constructor that will be called once and cached
	Singleton(name string, constructor interface{}) error

	// Initialize initializes all registered singletons
	Initialize() error

	// Shutdown cleans up all components
	Shutdown() error
}

// DIContainer is the default implementation of Container.
type DIContainer struct {
	constructors map[string]interface{}
	singletons   map[string]interface{}
	initialized  map[string]bool
	mu           sync.RWMutex
	initOrder    []string // Track initialization order for circular dependency detection
}

// NewContainer creates a new dependency injection container.
func NewContainer() Container {
	return &DIContainer{
		constructors: make(map[string]interface{}),
		singletons:   make(map[string]interface{}),
		initialized:  make(map[string]bool),
		initOrder:    make([]string, 0),
	}
}

// Register registers a constructor function for a named component.
// The constructor can have dependencies as parameters, which will be resolved automatically.
func (c *DIContainer) Register(name string, constructor interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.constructors[name]; exists {
		return fmt.Errorf("component '%s' is already registered", name)
	}

	// Validate that constructor is a function
	constructorType := reflect.TypeOf(constructor)
	if constructorType.Kind() != reflect.Func {
		return fmt.Errorf("constructor for '%s' must be a function, got %s", name, constructorType.Kind())
	}

	c.constructors[name] = constructor
	return nil
}

// Resolve resolves a component by name and returns it.
func (c *DIContainer) Resolve(name string) (interface{}, error) {
	c.mu.RLock()
	// Check if it's a singleton that's already initialized
	if instance, exists := c.singletons[name]; exists {
		c.mu.RUnlock()
		return instance, nil
	}
	c.mu.RUnlock()

	// Get constructor
	c.mu.RLock()
	constructor, exists := c.constructors[name]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("component '%s' is not registered", name)
	}

	// Call constructor with dependency resolution
	return c.callConstructor(name, constructor)
}

// ResolveAs resolves a component and assigns it to the target pointer.
func (c *DIContainer) ResolveAs(name string, target interface{}) error {
	// Validate that target is a pointer
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer, got %s", targetValue.Kind())
	}

	// Resolve the component
	instance, err := c.Resolve(name)
	if err != nil {
		return err
	}

	// Assign to target
	instanceValue := reflect.ValueOf(instance)
	targetElem := targetValue.Elem()

	if !instanceValue.Type().AssignableTo(targetElem.Type()) {
		return fmt.Errorf("cannot assign %s to %s", instanceValue.Type(), targetElem.Type())
	}

	targetElem.Set(instanceValue)
	return nil
}

// Singleton registers a constructor that will be called once and cached.
func (c *DIContainer) Singleton(name string, constructor interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.constructors[name]; exists {
		return fmt.Errorf("component '%s' is already registered", name)
	}

	// Validate that constructor is a function
	constructorType := reflect.TypeOf(constructor)
	if constructorType.Kind() != reflect.Func {
		return fmt.Errorf("constructor for '%s' must be a function, got %s", name, constructorType.Kind())
	}

	c.constructors[name] = constructor
	c.initialized[name] = false // Mark as singleton but not yet initialized
	return nil
}

// Initialize initializes all registered singletons.
func (c *DIContainer) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize all singletons
	for name := range c.initialized {
		if c.initialized[name] {
			continue // Already initialized
		}

		// Check for circular dependencies
		if c.isInInitOrder(name) {
			return fmt.Errorf("circular dependency detected: %v -> %s", c.initOrder, name)
		}

		// Add to init order for circular dependency detection
		c.initOrder = append(c.initOrder, name)

		constructor := c.constructors[name]
		instance, err := c.callConstructorUnsafe(name, constructor)
		if err != nil {
			return fmt.Errorf("failed to initialize singleton '%s': %w", name, err)
		}

		c.singletons[name] = instance
		c.initialized[name] = true

		// Remove from init order after successful initialization
		c.initOrder = c.initOrder[:len(c.initOrder)-1]
	}

	return nil
}

// Shutdown cleans up all components.
func (c *DIContainer) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Call Shutdown() method on components that implement it
	for name, instance := range c.singletons {
		if shutdowner, ok := instance.(interface{ Shutdown() error }); ok {
			if err := shutdowner.Shutdown(); err != nil {
				return fmt.Errorf("failed to shutdown component '%s': %w", name, err)
			}
		}
	}

	// Clear all state
	c.singletons = make(map[string]interface{})
	// Reset initialized flags so singletons can be re-initialized
	for name := range c.initialized {
		c.initialized[name] = false
	}
	c.initOrder = make([]string, 0)

	return nil
}

// callConstructor calls a constructor function with dependency resolution.
// This is the thread-safe version that acquires locks.
func (c *DIContainer) callConstructor(name string, constructor interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.callConstructorUnsafe(name, constructor)
}

// callConstructorUnsafe calls a constructor function with dependency resolution.
// This version assumes the caller already holds the lock.
func (c *DIContainer) callConstructorUnsafe(name string, constructor interface{}) (interface{}, error) {
	constructorValue := reflect.ValueOf(constructor)
	constructorType := constructorValue.Type()

	// Prepare arguments by resolving dependencies
	args := make([]reflect.Value, constructorType.NumIn())
	for i := 0; i < constructorType.NumIn(); i++ {
		argType := constructorType.In(i)

		// Try to find a registered component that matches this type
		var found bool
		for depName, depConstructor := range c.constructors {
			depType := reflect.TypeOf(depConstructor)
			if depType.Kind() != reflect.Func {
				continue
			}

			// Check if the return type matches
			if depType.NumOut() > 0 {
				returnType := depType.Out(0)
				if returnType.AssignableTo(argType) {
					// Resolve this dependency
					var depInstance interface{}
					var err error

					// Check if it's a singleton
					if singleton, exists := c.singletons[depName]; exists {
						depInstance = singleton
					} else {
						// Check for circular dependency
						if c.isInInitOrder(depName) {
							return nil, fmt.Errorf("circular dependency detected: %v -> %s", c.initOrder, depName)
						}

						// Add to init order
						c.initOrder = append(c.initOrder, depName)

						depInstance, err = c.callConstructorUnsafe(depName, depConstructor)
						if err != nil {
							return nil, fmt.Errorf("failed to resolve dependency '%s' for '%s': %w", depName, name, err)
						}

						// Remove from init order
						c.initOrder = c.initOrder[:len(c.initOrder)-1]
					}

					args[i] = reflect.ValueOf(depInstance)
					found = true
					break
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("cannot resolve dependency of type %s for component '%s'", argType, name)
		}
	}

	// Call the constructor
	results := constructorValue.Call(args)

	// Handle return values
	if len(results) == 0 {
		return nil, fmt.Errorf("constructor for '%s' must return at least one value", name)
	}

	// Check for error return
	if len(results) == 2 {
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			return nil, fmt.Errorf("constructor for '%s' returned error: %w", name, err)
		}
	}

	return results[0].Interface(), nil
}

// isInInitOrder checks if a component is currently being initialized (circular dependency check).
func (c *DIContainer) isInInitOrder(name string) bool {
	for _, n := range c.initOrder {
		if n == name {
			return true
		}
	}
	return false
}
