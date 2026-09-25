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
)

// TestNewRenderContext tests creating a new render context.
func TestNewRenderContext(t *testing.T) {
	data := map[string]string{"key": "value"}
	ctx := NewRenderContext(data)

	if ctx == nil {
		t.Fatal("NewRenderContext returned nil")
		return
	}

	if ctx.Data == nil {
		t.Error("data should be set")
	}

	if ctx.Metadata == nil {
		t.Error("metadata should be initialized")
	}

	if ctx.Functions == nil {
		t.Error("functions should be initialized")
	}
}

// TestRenderContextWithMetadata tests adding metadata.
func TestRenderContextWithMetadata(t *testing.T) {
	ctx := NewRenderContext(nil)

	result := ctx.WithMetadata("key1", "value1")

	if result != ctx {
		t.Error("WithMetadata should return the same context for chaining")
	}

	if ctx.Metadata["key1"] != "value1" {
		t.Error("metadata not set correctly")
	}
}

// TestRenderContextWithFunction tests adding a function.
func TestRenderContextWithFunction(t *testing.T) {
	ctx := NewRenderContext(nil)

	testFunc := func(s string) string { return s }
	result := ctx.WithFunction("testFunc", testFunc)

	if result != ctx {
		t.Error("WithFunction should return the same context for chaining")
	}

	if _, ok := ctx.Functions["testFunc"]; !ok {
		t.Error("function not added")
	}
}

// TestRenderContextWithFunctions tests adding multiple functions.
func TestRenderContextWithFunctions(t *testing.T) {
	ctx := NewRenderContext(nil)

	funcMap := template.FuncMap{
		"func1": func() {},
		"func2": func() {},
	}

	result := ctx.WithFunctions(funcMap)

	if result != ctx {
		t.Error("WithFunctions should return the same context for chaining")
	}

	if len(ctx.Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(ctx.Functions))
	}
}

// TestRenderContextWithStrictMode tests enabling strict mode.
func TestRenderContextWithStrictMode(t *testing.T) {
	ctx := NewRenderContext(nil)

	result := ctx.WithStrictMode(true)

	if result != ctx {
		t.Error("WithStrictMode should return the same context for chaining")
	}

	if !ctx.Options.StrictMode {
		t.Error("strict mode should be enabled")
	}

	if ctx.Options.MissingKeyBehavior != MissingKeyError {
		t.Error("missing key behavior should be set to error in strict mode")
	}
}

// TestRenderContextWithMissingKeyBehavior tests setting missing key behavior.
func TestRenderContextWithMissingKeyBehavior(t *testing.T) {
	ctx := NewRenderContext(nil)

	result := ctx.WithMissingKeyBehavior(MissingKeyZero)

	if result != ctx {
		t.Error("WithMissingKeyBehavior should return the same context for chaining")
	}

	if ctx.Options.MissingKeyBehavior != MissingKeyZero {
		t.Error("missing key behavior not set correctly")
	}
}

// TestRenderContextWithValidation tests enabling validation.
func TestRenderContextWithValidation(t *testing.T) {
	ctx := NewRenderContext(nil)

	result := ctx.WithValidation(false)

	if result != ctx {
		t.Error("WithValidation should return the same context for chaining")
	}

	if ctx.Options.ValidateBeforeRender {
		t.Error("validation should be disabled")
	}
}

// TestRenderContextToTemplateData tests converting to template data.
func TestRenderContextToTemplateData(t *testing.T) {
	data := map[string]string{"key": "value"}
	ctx := NewRenderContext(data)
	ctx.WithMetadata("meta", "data")

	templateData := ctx.ToTemplateData()

	if templateData["Data"] == nil {
		t.Error("data not included in template data")
	}

	metadata, ok := templateData["Metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata not included in template data")
	}

	if metadata["meta"] != "data" {
		t.Error("metadata value not correct")
	}
}

// TestRenderContextValidate tests context validation.
func TestRenderContextValidate(t *testing.T) {
	// Valid context
	ctx := NewRenderContext(map[string]string{"key": "value"})
	if err := ctx.Validate(); err != nil {
		t.Errorf("valid context should not return error: %v", err)
	}

	// Invalid context - nil data
	ctx = NewRenderContext(nil)
	if err := ctx.Validate(); err == nil {
		t.Error("context with nil data should return error")
	}

	// Invalid context - nil function
	ctx = NewRenderContext(map[string]string{"key": "value"})
	ctx.WithFunction("nilFunc", nil)
	if err := ctx.Validate(); err == nil {
		t.Error("context with nil function should return error")
	}
}

// TestRenderContextClone tests cloning a context.
func TestRenderContextClone(t *testing.T) {
	original := NewRenderContext(map[string]string{"key": "value"})
	original.WithMetadata("meta", "data")
	original.WithFunction("func", func() {})

	clone := original.Clone()

	if clone == original {
		t.Error("clone should be a different instance")
	}

	if clone.Data == nil {
		t.Error("data should be copied")
	}

	if len(clone.Metadata) != len(original.Metadata) {
		t.Error("metadata should be copied")
	}

	if len(clone.Functions) != len(original.Functions) {
		t.Error("functions should be copied")
	}

	// Modify clone should not affect original
	clone.WithMetadata("new", "value")
	if _, ok := original.Metadata["new"]; ok {
		t.Error("modifying clone should not affect original")
	}
}

// TestContextBuilder tests the context builder.
func TestContextBuilder(t *testing.T) {
	builder := NewContextBuilder()

	if builder == nil {
		t.Fatal("NewContextBuilder returned nil")
		return
	}

	if builder.context == nil {
		t.Error("context should be initialized")
	}
}

// TestContextBuilderWithData tests setting data.
func TestContextBuilderWithData(t *testing.T) {
	data := map[string]string{"key": "value"}
	builder := NewContextBuilder()

	result := builder.WithData(data)

	if result != builder {
		t.Error("WithData should return the same builder for chaining")
	}

	if builder.context.Data == nil {
		t.Error("data should be set")
	}
}

// TestContextBuilderBuild tests building a context.
func TestContextBuilderBuild(t *testing.T) {
	data := map[string]string{"key": "value"}
	builder := NewContextBuilder()

	ctx, err := builder.
		WithData(data).
		WithMetadata("meta", "data").
		WithStrictMode(true).
		Build()

	if err != nil {
		t.Fatalf("build should not return error: %v", err)
	}

	if ctx == nil {
		t.Fatal("build returned nil context")
		return
	}

	if !ctx.Options.StrictMode {
		t.Error("strict mode should be enabled")
	}

	if ctx.Metadata["meta"] != "data" {
		t.Error("metadata not set correctly")
	}
}

// TestContextBuilderBuildInvalid tests building an invalid context.
func TestContextBuilderBuildInvalid(t *testing.T) {
	builder := NewContextBuilder()

	// Build without setting data
	_, err := builder.Build()

	if err == nil {
		t.Error("building context without data should return error")
	}
}

// TestContextBuilderChaining tests method chaining.
func TestContextBuilderChaining(t *testing.T) {
	data := map[string]string{"key": "value"}

	ctx, err := NewContextBuilder().
		WithData(data).
		WithMetadata("key1", "value1").
		WithMetadata("key2", "value2").
		WithFunction("func1", func() {}).
		WithStrictMode(true).
		WithValidation(false).
		Build()

	if err != nil {
		t.Fatalf("chained build should not return error: %v", err)
	}

	if len(ctx.Metadata) != 2 {
		t.Errorf("expected 2 metadata entries, got %d", len(ctx.Metadata))
	}

	if len(ctx.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(ctx.Functions))
	}

	if !ctx.Options.StrictMode {
		t.Error("strict mode should be enabled")
	}

	if ctx.Options.ValidateBeforeRender {
		t.Error("validation should be disabled")
	}
}
