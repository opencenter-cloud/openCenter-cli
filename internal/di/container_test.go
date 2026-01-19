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
	"errors"
	"testing"
)

// Mock components for testing
type Logger struct {
	Name string
}

type Database struct {
	Logger *Logger
}

type Service struct {
	DB     *Database
	Logger *Logger
}

func TestNewContainer(t *testing.T) {
	container := NewContainer()
	if container == nil {
		t.Fatal("NewContainer() returned nil")
	}
}

func TestRegister(t *testing.T) {
	container := NewContainer()

	// Test successful registration
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test"}, nil
	})
	if err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	// Test duplicate registration
	err = container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test2"}, nil
	})
	if err == nil {
		t.Error("Register() should fail for duplicate component")
	}

	// Test invalid constructor (not a function)
	err = container.Register("invalid", "not a function")
	if err == nil {
		t.Error("Register() should fail for non-function constructor")
	}
}

func TestResolve(t *testing.T) {
	container := NewContainer()

	// Register a simple component
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Resolve the component
	instance, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Resolve() failed: %v", err)
	}

	logger, ok := instance.(*Logger)
	if !ok {
		t.Error("Resolve() returned wrong type")
	}
	if logger.Name != "test" {
		t.Errorf("Resolve() returned logger with wrong name: got %s, want test", logger.Name)
	}

	// Test resolving non-existent component
	_, err = container.Resolve("nonexistent")
	if err == nil {
		t.Error("Resolve() should fail for non-existent component")
	}
}

func TestResolveAs(t *testing.T) {
	container := NewContainer()

	// Register a component
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Resolve using ResolveAs
	var logger *Logger
	err = container.ResolveAs("logger", &logger)
	if err != nil {
		t.Errorf("ResolveAs() failed: %v", err)
	}
	if logger == nil {
		t.Error("ResolveAs() did not assign value")
	}
	if logger.Name != "test" {
		t.Errorf("ResolveAs() assigned wrong value: got %s, want test", logger.Name)
	}

	// Test with non-pointer target
	var notPointer Logger
	err = container.ResolveAs("logger", notPointer)
	if err == nil {
		t.Error("ResolveAs() should fail for non-pointer target")
	}
}

func TestSingleton(t *testing.T) {
	container := NewContainer()

	callCount := 0
	err := container.Singleton("logger", func() (*Logger, error) {
		callCount++
		return &Logger{Name: "singleton"}, nil
	})
	if err != nil {
		t.Fatalf("Singleton() failed: %v", err)
	}

	// Initialize singletons
	err = container.Initialize()
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Resolve multiple times
	instance1, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Resolve() failed: %v", err)
	}

	instance2, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Resolve() failed: %v", err)
	}

	// Should be the same instance
	if instance1 != instance2 {
		t.Error("Singleton() should return the same instance")
	}

	// Constructor should be called only once
	if callCount != 1 {
		t.Errorf("Singleton constructor called %d times, want 1", callCount)
	}
}

func TestDependencyResolution(t *testing.T) {
	container := NewContainer()

	// Register logger
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Register database with logger dependency
	err = container.Register("database", func(logger *Logger) (*Database, error) {
		return &Database{Logger: logger}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Resolve database (should automatically resolve logger)
	instance, err := container.Resolve("database")
	if err != nil {
		t.Errorf("Resolve() failed: %v", err)
	}

	db, ok := instance.(*Database)
	if !ok {
		t.Fatal("Resolve() returned wrong type")
	}
	if db.Logger == nil {
		t.Error("Dependency resolution failed: logger is nil")
	}
	if db.Logger.Name != "test" {
		t.Errorf("Dependency resolution failed: logger name is %s, want test", db.Logger.Name)
	}
}

func TestCircularDependencyDetection(t *testing.T) {
	container := NewContainer()

	// Create circular dependency: A depends on B, B depends on A
	type ComponentB struct {
		A interface{}
	}
	type ComponentA struct {
		B *ComponentB
	}

	err := container.Singleton("componentA", func(b *ComponentB) (*ComponentA, error) {
		return &ComponentA{B: b}, nil
	})
	if err != nil {
		t.Fatalf("Singleton() failed: %v", err)
	}

	err = container.Singleton("componentB", func(a *ComponentA) (*ComponentB, error) {
		return &ComponentB{A: a}, nil
	})
	if err != nil {
		t.Fatalf("Singleton() failed: %v", err)
	}

	// Initialize should detect circular dependency
	err = container.Initialize()
	if err == nil {
		t.Error("Initialize() should detect circular dependency")
	}
}

func TestConstructorError(t *testing.T) {
	container := NewContainer()

	// Register component with constructor that returns error
	err := container.Register("failing", func() (*Logger, error) {
		return nil, errors.New("constructor failed")
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Resolve should return the constructor error
	_, err = container.Resolve("failing")
	if err == nil {
		t.Error("Resolve() should return constructor error")
	}
}

func TestShutdown(t *testing.T) {
	container := NewContainer()

	// Create a component with Shutdown method
	type ShutdownableComponent struct {
		ShutdownCalled bool
	}

	shutdownable := &ShutdownableComponent{}

	err := container.Singleton("shutdownable", func() (*ShutdownableComponent, error) {
		return shutdownable, nil
	})
	if err != nil {
		t.Fatalf("Singleton() failed: %v", err)
	}

	// Initialize
	err = container.Initialize()
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Note: The component doesn't implement Shutdown() in this test,
	// so Shutdown() should succeed without calling anything
	err = container.Shutdown()
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}

	// After shutdown, singletons should be cleared
	// Resolving will re-initialize the singleton (calling constructor again)
	instance, err := container.Resolve("shutdownable")
	if err != nil {
		t.Errorf("Resolve() should work after Shutdown(): %v", err)
	}
	
	// Since it's a singleton, it should be the same instance as before
	// (the constructor returns the same pointer)
	newShutdownable, ok := instance.(*ShutdownableComponent)
	if !ok {
		t.Error("Resolve() returned wrong type")
	}
	if newShutdownable != shutdownable {
		t.Error("Singleton should return the same instance even after Shutdown()")
	}
}

func TestMultipleDependencies(t *testing.T) {
	container := NewContainer()

	// Register logger
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Register database
	err = container.Register("database", func(logger *Logger) (*Database, error) {
		return &Database{Logger: logger}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Register service with multiple dependencies
	err = container.Register("service", func(db *Database, logger *Logger) (*Service, error) {
		return &Service{DB: db, Logger: logger}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Resolve service
	instance, err := container.Resolve("service")
	if err != nil {
		t.Errorf("Resolve() failed: %v", err)
	}

	service, ok := instance.(*Service)
	if !ok {
		t.Fatal("Resolve() returned wrong type")
	}
	if service.DB == nil {
		t.Error("Service.DB is nil")
	}
	if service.Logger == nil {
		t.Error("Service.Logger is nil")
	}
	if service.DB.Logger == nil {
		t.Error("Service.DB.Logger is nil")
	}
}
