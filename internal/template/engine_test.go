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
	"context"
	"testing"
	"text/template"
)

// TestNewGoTemplateEngine tests the creation of a new Go template engine.
func TestNewGoTemplateEngine(t *testing.T) {
	engine := NewGoTemplateEngine()

	if engine == nil {
		t.Fatal("NewGoTemplateEngine returned nil")
		return
	}

	if engine.funcMap == nil {
		t.Error("funcMap should be initialized")
	}

	if engine.cache == nil {
		t.Error("cache should be initialized")
	}

	if !engine.cacheEnabled {
		t.Error("cache should be enabled by default")
	}
}

// TestRegisterFunction tests registering custom functions.
func TestRegisterFunction(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Register a simple function
	testFunc := func(s string) string {
		return "test_" + s
	}

	engine.RegisterFunction("testFunc", testFunc)

	// Verify function was registered
	if _, ok := engine.funcMap["testFunc"]; !ok {
		t.Error("function was not registered")
	}

	// Verify cache was cleared
	if len(engine.cache) != 0 {
		t.Error("cache should be cleared after registering function")
	}
}

// TestSetCacheEnabled tests enabling and disabling cache.
func TestSetCacheEnabled(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Add a dummy entry to cache
	engine.cache["test"] = template.New("test")

	// Disable cache
	engine.SetCacheEnabled(false)

	if engine.cacheEnabled {
		t.Error("cache should be disabled")
	}

	if len(engine.cache) != 0 {
		t.Error("cache should be cleared when disabled")
	}

	// Enable cache
	engine.SetCacheEnabled(true)

	if !engine.cacheEnabled {
		t.Error("cache should be enabled")
	}
}

// TestClearCache tests clearing the cache.
func TestClearCache(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Add dummy entries to cache
	engine.cache["test1"] = template.New("test1")
	engine.cache["test2"] = template.New("test2")

	if len(engine.cache) != 2 {
		t.Fatalf("expected 2 cache entries, got %d", len(engine.cache))
	}

	// Clear cache
	engine.ClearCache()

	if len(engine.cache) != 0 {
		t.Errorf("cache should be empty after clear, got %d entries", len(engine.cache))
	}
}

// TestRenderContextCancellation tests that rendering respects context cancellation.
func TestRenderContextCancellation(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Attempt to render with cancelled context
	_, err := engine.Render(ctx, "test.tmpl", nil)

	if err == nil {
		t.Error("expected error when rendering with cancelled context")
	}
}

// TestTemplateContext tests the TemplateContext functionality.
func TestTemplateContext(t *testing.T) {
	config := map[string]string{"key": "value"}
	ctx := NewTemplateContext(config)

	if ctx.Config == nil {
		t.Error("config was not set correctly")
	}

	if ctx.Metadata == nil {
		t.Error("metadata should be initialized")
	}

	if ctx.Functions == nil {
		t.Error("functions should be initialized")
	}
}

// TestTemplateContextWithMetadata tests adding metadata to context.
func TestTemplateContextWithMetadata(t *testing.T) {
	ctx := NewTemplateContext(nil)

	ctx.WithMetadata("key1", "value1").
		WithMetadata("key2", 42)

	if ctx.Metadata["key1"] != "value1" {
		t.Error("metadata key1 not set correctly")
	}

	if ctx.Metadata["key2"] != 42 {
		t.Error("metadata key2 not set correctly")
	}
}

// TestTemplateContextWithFunction tests adding functions to context.
func TestTemplateContextWithFunction(t *testing.T) {
	ctx := NewTemplateContext(nil)

	testFunc := func(s string) string {
		return s
	}

	ctx.WithFunction("testFunc", testFunc)

	if _, ok := ctx.Functions["testFunc"]; !ok {
		t.Error("function was not added to context")
	}
}

// TestTemplateContextToMap tests converting context to map.
func TestTemplateContextToMap(t *testing.T) {
	config := map[string]string{"key": "value"}
	ctx := NewTemplateContext(config)
	ctx.WithMetadata("meta", "data")

	data := ctx.ToMap()

	if data["Config"] == nil {
		t.Error("config not included in map")
	}

	metadata, ok := data["Metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata not included in map")
	}

	if metadata["meta"] != "data" {
		t.Error("metadata value not correct")
	}
}

// TestConcurrentCacheAccess tests concurrent access to the cache.
func TestConcurrentCacheAccess(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			engine.RegisterFunction("func", func() {})
			engine.SetCacheEnabled(true)
			engine.ClearCache()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without deadlock or panic, test passes
}

// TestTemplateEngineInterface tests that GoTemplateEngine implements TemplateEngine.
func TestTemplateEngineInterface(t *testing.T) {
	var _ TemplateEngine = (*GoTemplateEngine)(nil)
}
