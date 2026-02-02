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

func TestAllocationOptimizer_StringSlice(t *testing.T) {
	optimizer := GetAllocationOptimizer()

	// Get a string slice from the pool
	s := optimizer.GetStringSlice()
	if s == nil {
		t.Fatal("GetStringSlice returned nil")
	}

	// Verify initial state
	if len(*s) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(*s))
	}
	if cap(*s) < 16 {
		t.Errorf("Expected capacity >= 16, got %d", cap(*s))
	}

	// Modify the slice
	*s = append(*s, "test1", "test2", "test3")

	// Return to pool
	optimizer.PutStringSlice(s)

	// Get another slice (should be reset)
	s2 := optimizer.GetStringSlice()
	if s2 == nil {
		t.Fatal("GetStringSlice returned nil on second call")
	}

	// Verify it was reset
	if len(*s2) != 0 {
		t.Errorf("Expected empty slice after reset, got length %d", len(*s2))
	}
}

func TestAllocationOptimizer_Map(t *testing.T) {
	optimizer := GetAllocationOptimizer()

	// Get a map from the pool
	m := optimizer.GetMap()
	if m == nil {
		t.Fatal("GetMap returned nil")
	}

	// Verify initial state
	if len(*m) != 0 {
		t.Errorf("Expected empty map, got length %d", len(*m))
	}

	// Modify the map
	(*m)["key1"] = "value1"
	(*m)["key2"] = "value2"

	// Return to pool
	optimizer.PutMap(m)

	// Get another map (should be cleared)
	m2 := optimizer.GetMap()
	if m2 == nil {
		t.Fatal("GetMap returned nil on second call")
	}

	// Verify it was cleared
	if len(*m2) != 0 {
		t.Errorf("Expected empty map after reset, got length %d", len(*m2))
	}
}

func TestOptimizedStringSlice(t *testing.T) {
	tests := []struct {
		name          string
		estimatedSize int
		wantMinCap    int
	}{
		{"zero size", 0, 1},
		{"small size", 5, 8},
		{"medium size", 20, 32},
		{"large size", 100, 128},
		{"power of 2", 16, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := OptimizedStringSlice(tt.estimatedSize)
			if cap(s) < tt.wantMinCap {
				t.Errorf("OptimizedStringSlice(%d) capacity = %d, want >= %d", tt.estimatedSize, cap(s), tt.wantMinCap)
			}
			if len(s) != 0 {
				t.Errorf("OptimizedStringSlice(%d) length = %d, want 0", tt.estimatedSize, len(s))
			}
		})
	}
}

func TestOptimizedMap(t *testing.T) {
	tests := []struct {
		name          string
		estimatedSize int
		wantMinCap    int
	}{
		{"zero size", 0, 1},
		{"small size", 5, 8},
		{"medium size", 20, 32},
		{"large size", 100, 128},
		{"power of 2", 16, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := OptimizedMap(tt.estimatedSize)
			if len(m) != 0 {
				t.Errorf("OptimizedMap(%d) length = %d, want 0", tt.estimatedSize, len(m))
			}
		})
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{17, 32},
		{31, 32},
		{32, 32},
		{33, 64},
		{100, 128},
		{1000, 1024},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := nextPowerOf2(tt.input)
			if got != tt.want {
				t.Errorf("nextPowerOf2(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestReuseSliceCapacity(t *testing.T) {
	// Create a slice with some capacity
	s := make([]string, 0, 32)
	s = append(s, "test1", "test2", "test3")

	// Reuse the capacity
	s = ReuseSliceCapacity(s)

	// Verify length is 0 but capacity is preserved
	if len(s) != 0 {
		t.Errorf("Expected length 0, got %d", len(s))
	}
	if cap(s) != 32 {
		t.Errorf("Expected capacity 32, got %d", cap(s))
	}
}

func TestReuseMapCapacity(t *testing.T) {
	// Create a map with some entries
	m := make(map[string]interface{}, 32)
	m["key1"] = "value1"
	m["key2"] = "value2"
	m["key3"] = "value3"

	// Reuse the capacity
	ReuseMapCapacity(m)

	// Verify map is empty
	if len(m) != 0 {
		t.Errorf("Expected empty map, got length %d", len(m))
	}
}

func TestPreAllocateSlices(t *testing.T) {
	// This should not panic
	PreAllocateSlices()
}

func BenchmarkAllocationOptimizer_StringSlice(b *testing.B) {
	optimizer := GetAllocationOptimizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := optimizer.GetStringSlice()
		*s = append(*s, "test")
		optimizer.PutStringSlice(s)
	}
}

func BenchmarkAllocationOptimizer_StringSlice_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]string, 0, 16)
		s = append(s, "test")
		_ = s
	}
}

func BenchmarkAllocationOptimizer_Map(b *testing.B) {
	optimizer := GetAllocationOptimizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := optimizer.GetMap()
		(*m)["key"] = "value"
		optimizer.PutMap(m)
	}
}

func BenchmarkAllocationOptimizer_Map_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{}, 32)
		m["key"] = "value"
		_ = m
	}
}

func BenchmarkOptimizedStringSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := OptimizedStringSlice(20)
		_ = s
	}
}

func BenchmarkOptimizedStringSlice_Standard(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]string, 0, 20)
		_ = s
	}
}
