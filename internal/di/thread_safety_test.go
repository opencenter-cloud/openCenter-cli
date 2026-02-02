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
	"sync"
	"testing"
)

// TestConcurrentRegister tests concurrent registration of components.
func TestConcurrentRegister(t *testing.T) {
	container := NewContainer()
	var wg sync.WaitGroup

	// Register 100 components concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := string(rune('a'+(id%26))) + string(rune('0'+(id/26)))
			err := container.Register(name, func() (*Logger, error) {
				return &Logger{Name: name}, nil
			})
			if err != nil {
				t.Errorf("Register() failed for %s: %v", name, err)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentResolve tests concurrent resolution of components.
func TestConcurrentResolve(t *testing.T) {
	container := NewContainer()

	// Register a component
	err := container.Register("logger", func() (*Logger, error) {
		return &Logger{Name: "concurrent"}, nil
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Resolve 100 times concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := container.Resolve("logger")
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Resolve() failed: %v", err)
	}
}

// TestConcurrentSingletonResolve tests concurrent resolution of singleton components.
func TestConcurrentSingletonResolve(t *testing.T) {
	container := NewContainer()

	callCount := 0
	var mu sync.Mutex

	// Register a singleton
	err := container.Singleton("logger", func() (*Logger, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &Logger{Name: "singleton"}, nil
	})
	if err != nil {
		t.Fatalf("Singleton() failed: %v", err)
	}

	// Initialize
	err = container.Initialize()
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	var wg sync.WaitGroup
	instances := make(chan interface{}, 100)

	// Resolve 100 times concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			instance, err := container.Resolve("logger")
			if err != nil {
				t.Errorf("Resolve() failed: %v", err)
				return
			}
			instances <- instance
		}()
	}

	wg.Wait()
	close(instances)

	// All instances should be the same
	var firstInstance interface{}
	count := 0
	for instance := range instances {
		if firstInstance == nil {
			firstInstance = instance
		} else if instance != firstInstance {
			t.Error("Singleton returned different instances")
		}
		count++
	}

	if count != 100 {
		t.Errorf("Expected 100 instances, got %d", count)
	}

	// Constructor should be called only once
	mu.Lock()
	defer mu.Unlock()
	if callCount != 1 {
		t.Errorf("Singleton constructor called %d times, want 1", callCount)
	}
}

// TestConcurrentRegisterAndResolve tests concurrent registration and resolution.
func TestConcurrentRegisterAndResolve(t *testing.T) {
	container := NewContainer()
	var wg sync.WaitGroup

	// Register and resolve concurrently
	for i := 0; i < 50; i++ {
		wg.Add(2)

		// Register
		go func(id int) {
			defer wg.Done()
			name := string(rune('a'+(id%26))) + string(rune('0'+(id/26)))
			err := container.Register(name, func() (*Logger, error) {
				return &Logger{Name: name}, nil
			})
			if err != nil {
				// Duplicate registration is expected in concurrent scenario
				return
			}
		}(i)

		// Resolve
		go func(id int) {
			defer wg.Done()
			name := string(rune('a'+(id%26))) + string(rune('0'+(id/26)))
			_, err := container.Resolve(name)
			if err != nil {
				// Component might not be registered yet, which is fine
				return
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentInitialize tests concurrent initialization of singletons.
func TestConcurrentInitialize(t *testing.T) {
	container := NewContainer()

	// Register multiple singletons
	for i := 0; i < 10; i++ {
		name := string(rune('a' + i))
		err := container.Singleton(name, func() (*Logger, error) {
			return &Logger{Name: name}, nil
		})
		if err != nil {
			t.Fatalf("Singleton() failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Initialize concurrently (should be safe)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := container.Initialize()
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Initialize() failed: %v", err)
	}
}
