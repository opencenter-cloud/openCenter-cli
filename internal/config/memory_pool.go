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

// MemoryPool provides centralized memory pooling for frequently allocated objects.
// This reduces GC pressure and improves performance by reusing memory allocations.
//
// Usage:
//
//	pool := GetMemoryPool()
//	obj := pool.GetConfigErrorSlice()
//	defer pool.PutConfigErrorSlice(obj)
type MemoryPool struct {
	configErrorSlices sync.Pool
	stringSlices      sync.Pool
	mapStringString   sync.Pool
}

var (
	globalMemoryPool     *MemoryPool
	globalMemoryPoolOnce sync.Once
)

// GetMemoryPool returns the global memory pool instance.
func GetMemoryPool() *MemoryPool {
	globalMemoryPoolOnce.Do(func() {
		globalMemoryPool = &MemoryPool{
			configErrorSlices: sync.Pool{
				New: func() interface{} {
					s := make([]*ConfigError, 0, 8)
					return &s
				},
			},
			stringSlices: sync.Pool{
				New: func() interface{} {
					s := make([]string, 0, 16)
					return &s
				},
			},
			mapStringString: sync.Pool{
				New: func() interface{} {
					m := make(map[string]string, 16)
					return &m
				},
			},
		}
	})
	return globalMemoryPool
}

// GetConfigErrorSlice retrieves a ConfigError slice from the pool.
func (mp *MemoryPool) GetConfigErrorSlice() *[]*ConfigError {
	return mp.configErrorSlices.Get().(*[]*ConfigError)
}

// PutConfigErrorSlice returns a ConfigError slice to the pool after resetting it.
func (mp *MemoryPool) PutConfigErrorSlice(s *[]*ConfigError) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	mp.configErrorSlices.Put(s)
}

// GetStringSlice retrieves a string slice from the pool.
func (mp *MemoryPool) GetStringSlice() *[]string {
	return mp.stringSlices.Get().(*[]string)
}

// PutStringSlice returns a string slice to the pool after resetting it.
func (mp *MemoryPool) PutStringSlice(s *[]string) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	mp.stringSlices.Put(s)
}

// GetMapStringString retrieves a map[string]string from the pool.
func (mp *MemoryPool) GetMapStringString() *map[string]string {
	return mp.mapStringString.Get().(*map[string]string)
}

// PutMapStringString returns a map[string]string to the pool after clearing it.
func (mp *MemoryPool) PutMapStringString(m *map[string]string) {
	if m == nil {
		return
	}
	// Clear the map
	for k := range *m {
		delete(*m, k)
	}
	mp.mapStringString.Put(m)
}

// PoolStats provides statistics about memory pool usage.
type PoolStats struct {
	ConfigErrorSlicesInUse int
	StringSlicesInUse      int
	MapsInUse              int
}

// GetStats returns statistics about the memory pool.
// Note: This is approximate as sync.Pool doesn't expose exact counts.
func (mp *MemoryPool) GetStats() PoolStats {
	// sync.Pool doesn't provide exact statistics, so we return zeros
	// This is a placeholder for future monitoring integration
	return PoolStats{}
}
