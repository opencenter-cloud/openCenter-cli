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

package config

import (
	"testing"
)

func TestMemoryPool_ConfigErrorSlice(t *testing.T) {
	pool := GetMemoryPool()

	// Get an error slice from the pool
	errors := pool.GetConfigErrorSlice()
	if errors == nil {
		t.Fatal("GetConfigErrorSlice returned nil")
	}

	// Verify initial state
	if len(*errors) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(*errors))
	}

	// Modify the slice
	*errors = append(*errors, &ConfigError{Message: "test error 1"})
	*errors = append(*errors, &ConfigError{Message: "test error 2"})

	// Return to pool
	pool.PutConfigErrorSlice(errors)

	// Get another slice (should be reset)
	errors2 := pool.GetConfigErrorSlice()
	if errors2 == nil {
		t.Fatal("GetConfigErrorSlice returned nil on second call")
	}

	// Verify it was reset
	if len(*errors2) != 0 {
		t.Errorf("Expected empty slice after reset, got length %d", len(*errors2))
	}
}

func TestMemoryPool_StringSlice(t *testing.T) {
	pool := GetMemoryPool()

	// Get a string slice from the pool
	strings := pool.GetStringSlice()
	if strings == nil {
		t.Fatal("GetStringSlice returned nil")
	}

	// Verify initial state
	if len(*strings) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(*strings))
	}

	// Modify the slice
	*strings = append(*strings, "test1", "test2")

	// Return to pool
	pool.PutStringSlice(strings)

	// Get another slice (should be reset)
	strings2 := pool.GetStringSlice()
	if strings2 == nil {
		t.Fatal("GetStringSlice returned nil on second call")
	}

	// Verify it was reset
	if len(*strings2) != 0 {
		t.Errorf("Expected empty slice after reset, got length %d", len(*strings2))
	}
}

func TestMemoryPool_MapStringString(t *testing.T) {
	pool := GetMemoryPool()

	// Get a map from the pool
	m := pool.GetMapStringString()
	if m == nil {
		t.Fatal("GetMapStringString returned nil")
	}

	// Verify initial state
	if len(*m) != 0 {
		t.Errorf("Expected empty map, got length %d", len(*m))
	}

	// Modify the map
	(*m)["key1"] = "value1"
	(*m)["key2"] = "value2"

	// Return to pool
	pool.PutMapStringString(m)

	// Get another map (should be cleared)
	m2 := pool.GetMapStringString()
	if m2 == nil {
		t.Fatal("GetMapStringString returned nil on second call")
	}

	// Verify it was cleared
	if len(*m2) != 0 {
		t.Errorf("Expected empty map after reset, got length %d", len(*m2))
	}
}

func TestMemoryPool_NilHandling(t *testing.T) {
	pool := GetMemoryPool()

	// Test that putting nil doesn't panic
	pool.PutConfigErrorSlice(nil)
	pool.PutStringSlice(nil)
	pool.PutMapStringString(nil)
}

func BenchmarkMemoryPool_ConfigErrorSlice(b *testing.B) {
	pool := GetMemoryPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors := pool.GetConfigErrorSlice()
		*errors = append(*errors, &ConfigError{Message: "test"})
		pool.PutConfigErrorSlice(errors)
	}
}

func BenchmarkMemoryPool_ConfigErrorSlice_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors := make([]*ConfigError, 0, 8)
		errors = append(errors, &ConfigError{Message: "test"})
		_ = errors
	}
}

func BenchmarkMemoryPool_MapStringString(b *testing.B) {
	pool := GetMemoryPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := pool.GetMapStringString()
		(*m)["key"] = "value"
		pool.PutMapStringString(m)
	}
}

func BenchmarkMemoryPool_MapStringString_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]string, 16)
		m["key"] = "value"
		_ = m
	}
}
