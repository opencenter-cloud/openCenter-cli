/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"testing"
	"text/template"
	"time"
)

// TestNewInMemoryTemplateCache tests cache creation.
func TestNewInMemoryTemplateCache(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)

	if cache == nil {
		t.Fatal("NewInMemoryTemplateCache returned nil")
		return
	}

	if cache.entries == nil {
		t.Error("entries map should be initialized")
	}
}

// TestCacheGetSet tests basic get and set operations.
func TestCacheGetSet(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)
	tmpl := template.New("test")

	// Set template
	cache.Set("test", tmpl)

	// Get template
	retrieved, ok := cache.Get("test")
	if !ok {
		t.Fatal("template not found in cache")
	}

	if retrieved != tmpl {
		t.Error("retrieved template does not match original")
	}
}

// TestCacheGetMissing tests getting a non-existent template.
func TestCacheGetMissing(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected false for non-existent template")
	}
}

// TestCacheDelete tests deleting templates.
func TestCacheDelete(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)
	tmpl := template.New("test")

	cache.Set("test", tmpl)
	cache.Delete("test")

	_, ok := cache.Get("test")
	if ok {
		t.Error("template should be deleted")
	}
}

// TestCacheClear tests clearing all templates.
func TestCacheClear(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)

	cache.Set("test1", template.New("test1"))
	cache.Set("test2", template.New("test2"))

	if cache.Size() != 2 {
		t.Fatalf("expected 2 entries, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("cache should be empty, got %d entries", cache.Size())
	}
}

// TestCacheSize tests the Size method.
func TestCacheSize(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)

	if cache.Size() != 0 {
		t.Error("new cache should be empty")
	}

	cache.Set("test1", template.New("test1"))
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	cache.Set("test2", template.New("test2"))
	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}
}

// TestCacheTTL tests TTL-based expiration.
func TestCacheTTL(t *testing.T) {
	// Create cache with 100ms TTL
	cache := NewInMemoryTemplateCache(100*time.Millisecond, 0)
	tmpl := template.New("test")

	cache.Set("test", tmpl)

	// Should be available immediately
	_, ok := cache.Get("test")
	if !ok {
		t.Error("template should be available immediately")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get("test")
	if ok {
		t.Error("template should be expired")
	}
}

// TestCacheMaxSize tests size-based eviction.
func TestCacheMaxSize(t *testing.T) {
	// Create cache with max size of 2
	cache := NewInMemoryTemplateCache(0, 2)

	cache.Set("test1", template.New("test1"))
	cache.Set("test2", template.New("test2"))

	if cache.Size() != 2 {
		t.Fatalf("expected size 2, got %d", cache.Size())
	}

	// Adding third entry should trigger eviction
	cache.Set("test3", template.New("test3"))

	if cache.Size() > 2 {
		t.Errorf("cache size should not exceed max size, got %d", cache.Size())
	}
}

// TestCacheLRUEviction tests that least recently used entries are evicted.
func TestCacheLRUEviction(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 2)

	cache.Set("test1", template.New("test1"))
	time.Sleep(10 * time.Millisecond)
	cache.Set("test2", template.New("test2"))

	// Access test2 to make it more recently used
	cache.Get("test2")
	time.Sleep(10 * time.Millisecond)

	// Add test3, should evict test1 (least recently used)
	cache.Set("test3", template.New("test3"))

	// test1 should be evicted
	_, ok := cache.Get("test1")
	if ok {
		t.Error("test1 should have been evicted")
	}

	// test2 and test3 should still be present
	_, ok = cache.Get("test2")
	if !ok {
		t.Error("test2 should still be in cache")
	}

	_, ok = cache.Get("test3")
	if !ok {
		t.Error("test3 should be in cache")
	}
}

// TestCacheAccessCount tests that access count is tracked.
func TestCacheAccessCount(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)
	tmpl := template.New("test")

	cache.Set("test", tmpl)

	// Access multiple times
	for i := 0; i < 5; i++ {
		cache.Get("test")
	}

	// Check stats
	stats := cache.GetStats()
	if stats.TotalAccess < 5 {
		t.Errorf("expected at least 5 accesses, got %d", stats.TotalAccess)
	}
}

// TestCacheGetStats tests the GetStats method.
func TestCacheGetStats(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 0)

	cache.Set("test1", template.New("test1"))
	cache.Set("test2", template.New("test2"))

	stats := cache.GetStats()

	if stats.Size != 2 {
		t.Errorf("expected size 2, got %d", stats.Size)
	}
}

// TestCacheConcurrentAccess tests concurrent cache operations.
func TestCacheConcurrentAccess(t *testing.T) {
	cache := NewInMemoryTemplateCache(0, 100)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			tmpl := template.New("test")
			cache.Set("test", tmpl)
			cache.Get("test")
			cache.Delete("test")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without deadlock or panic, test passes
}

// TestCacheInterface tests that InMemoryTemplateCache implements TemplateCache.
func TestCacheInterface(t *testing.T) {
	var _ TemplateCache = (*InMemoryTemplateCache)(nil)
}
