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
	"sync"
)

// AllocationOptimizer provides utilities to reduce memory allocations
// during configuration operations.
type AllocationOptimizer struct {
	// Pre-allocated slices for common operations
	stringSlicePool sync.Pool
	mapPool         sync.Pool
}

var (
	globalAllocOptimizer     *AllocationOptimizer
	globalAllocOptimizerOnce sync.Once
)

// GetAllocationOptimizer returns the global allocation optimizer instance.
func GetAllocationOptimizer() *AllocationOptimizer {
	globalAllocOptimizerOnce.Do(func() {
		globalAllocOptimizer = &AllocationOptimizer{
			stringSlicePool: sync.Pool{
				New: func() interface{} {
					// Pre-allocate with capacity for typical use cases
					s := make([]string, 0, 16)
					return &s
				},
			},
			mapPool: sync.Pool{
				New: func() interface{} {
					// Pre-allocate with capacity for typical service maps
					m := make(map[string]interface{}, 32)
					return &m
				},
			},
		}
	})
	return globalAllocOptimizer
}

// GetStringSlice retrieves a pre-allocated string slice from the pool.
// The caller must call PutStringSlice when done.
func (ao *AllocationOptimizer) GetStringSlice() *[]string {
	return ao.stringSlicePool.Get().(*[]string)
}

// PutStringSlice returns a string slice to the pool after resetting it.
func (ao *AllocationOptimizer) PutStringSlice(s *[]string) {
	if s == nil {
		return
	}
	*s = (*s)[:0] // Reset length but keep capacity
	ao.stringSlicePool.Put(s)
}

// GetMap retrieves a pre-allocated map from the pool.
// The caller must call PutMap when done.
func (ao *AllocationOptimizer) GetMap() *map[string]interface{} {
	return ao.mapPool.Get().(*map[string]interface{})
}

// PutMap returns a map to the pool after clearing it.
func (ao *AllocationOptimizer) PutMap(m *map[string]interface{}) {
	if m == nil {
		return
	}
	// Clear the map
	for k := range *m {
		delete(*m, k)
	}
	ao.mapPool.Put(m)
}

// PreAllocateSlices pre-allocates commonly used slices to reduce allocations
// during config generation. This should be called once at startup.
func PreAllocateSlices() {
	// Pre-warm the pools by allocating and returning objects
	optimizer := GetAllocationOptimizer()

	// Pre-allocate 10 string slices
	slices := make([]*[]string, 10)
	for i := range slices {
		slices[i] = optimizer.GetStringSlice()
	}
	for _, s := range slices {
		optimizer.PutStringSlice(s)
	}

	// Pre-allocate 10 maps
	maps := make([]*map[string]interface{}, 10)
	for i := range maps {
		maps[i] = optimizer.GetMap()
	}
	for _, m := range maps {
		optimizer.PutMap(m)
	}
}

// OptimizedStringSlice creates a string slice with optimized capacity.
// Use this instead of make([]string, 0) for better performance.
func OptimizedStringSlice(estimatedSize int) []string {
	// Round up to next power of 2 for better memory alignment
	capacity := nextPowerOf2(estimatedSize)
	return make([]string, 0, capacity)
}

// OptimizedMap creates a map with optimized capacity.
// Use this instead of make(map[string]interface{}) for better performance.
func OptimizedMap(estimatedSize int) map[string]interface{} {
	// Round up to next power of 2 for better memory alignment
	capacity := nextPowerOf2(estimatedSize)
	return make(map[string]interface{}, capacity)
}

// nextPowerOf2 returns the next power of 2 greater than or equal to n.
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}
	// Handle edge case where n is already a power of 2
	if n&(n-1) == 0 {
		return n
	}
	// Find the next power of 2
	power := 1
	for power < n {
		power <<= 1
	}
	return power
}

// ReuseSliceCapacity reuses the capacity of an existing slice.
// This is useful when you need to clear a slice but want to keep its capacity.
func ReuseSliceCapacity(s []string) []string {
	return s[:0]
}

// ReuseMapCapacity clears a map but keeps its allocated capacity.
// This is useful when you need to reuse a map for a new operation.
func ReuseMapCapacity(m map[string]interface{}) {
	for k := range m {
		delete(m, k)
	}
}

// AllocationStats provides statistics about allocation optimization.
type AllocationStats struct {
	StringSlicesInPool int
	MapsInPool         int
}

// GetStats returns statistics about the allocation optimizer.
// Note: This is approximate as sync.Pool doesn't expose exact counts.
func (ao *AllocationOptimizer) GetStats() AllocationStats {
	// sync.Pool doesn't provide exact statistics, so we return zeros
	// This is a placeholder for future monitoring integration
	return AllocationStats{}
}
