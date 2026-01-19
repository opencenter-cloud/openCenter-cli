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
	"strings"
	"testing"
	"time"
)

func TestTemplateSandbox_SafeFunctions(t *testing.T) {
	sandbox := NewTemplateSandbox()

	tests := []struct {
		name     string
		template string
		data     interface{}
		wantErr  bool
	}{
		{
			name:     "upper function",
			template: `{{ . | upper }}`,
			data:     "hello",
			wantErr:  false,
		},
		{
			name:     "lower function",
			template: `{{ . | lower }}`,
			data:     "HELLO",
			wantErr:  false,
		},
		{
			name:     "trim function",
			template: `{{ . | trim }}`,
			data:     "  hello  ",
			wantErr:  false,
		},
		{
			name:     "printf function",
			template: `{{ printf "Hello %s" . }}`,
			data:     "World",
			wantErr:  false,
		},
		{
			name:     "quote function",
			template: `{{ quote . }}`,
			data:     "test",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sandbox.RenderWithTimeout(tt.template, tt.data, 5*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTemplateSandbox_DangerousFunctions(t *testing.T) {
	sandbox := NewTemplateSandbox()

	tests := []struct {
		name     string
		template string
	}{
		{
			name:     "env function",
			template: `{{ env "PATH" }}`,
		},
		{
			name:     "expandenv function",
			template: `{{ expandenv "$PATH" }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sandbox.ValidateTemplate(tt.template)
			if err == nil {
				t.Errorf("ValidateTemplate() should have failed for dangerous function")
			}
			if !strings.Contains(err.Error(), "not defined") {
				t.Errorf("ValidateTemplate() error should mention 'not defined', got: %v", err)
			}
		})
	}
}

func TestTemplateSandbox_Timeout(t *testing.T) {
	sandbox := NewTemplateSandbox()

	// Template with a loop that will take some time
	template := `{{ range $i := until 1000 }}{{ range $j := until 1000 }}{{ $i }}{{ end }}{{ end }}`

	// Use a very short timeout
	_, err := sandbox.RenderWithTimeout(template, nil, 10*time.Millisecond)

	if err == nil {
		t.Error("RenderWithTimeout() should have timed out")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("RenderWithTimeout() error should mention 'timed out', got: %v", err)
	}
}

func TestGoTemplateEngine_Sandboxing(t *testing.T) {
	engine := NewGoTemplateEngine()

	// Initially not sandboxed
	if engine.IsSandboxed() {
		t.Error("Engine should not be sandboxed by default")
	}

	// Enable sandboxing
	engine.EnableSandbox()

	if !engine.IsSandboxed() {
		t.Error("Engine should be sandboxed after EnableSandbox()")
	}

	// Test that dangerous functions are rejected
	ctx := context.Background()
	_, err := engine.RenderString(ctx, "test", `{{ env "PATH" }}`, nil)

	if err == nil {
		t.Error("RenderString() should have failed for dangerous function when sandboxed")
	}

	// Test that safe functions work
	result, err := engine.RenderString(ctx, "test", `{{ . | upper }}`, "hello")
	if err != nil {
		t.Errorf("RenderString() failed for safe function: %v", err)
	}

	if string(result) != "HELLO" {
		t.Errorf("RenderString() = %s, want HELLO", result)
	}

	// Disable sandboxing
	engine.DisableSandbox()

	if engine.IsSandboxed() {
		t.Error("Engine should not be sandboxed after DisableSandbox()")
	}
}

func TestDangerousFunctions(t *testing.T) {
	dangerous := DangerousFunctions()

	// Should include key dangerous functions
	expectedFunctions := []string{"env", "expandenv", "readFile", "writeFile", "exec"}

	for _, expected := range expectedFunctions {
		found := false
		for _, df := range dangerous {
			if df == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DangerousFunctions() should include %s", expected)
		}
	}
}

func TestIsDangerousFunction(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		want     bool
	}{
		{"env is dangerous", "env", true},
		{"expandenv is dangerous", "expandenv", true},
		{"readFile is dangerous", "readFile", true},
		{"writeFile is dangerous", "writeFile", true},
		{"exec is dangerous", "exec", true},
		{"upper is safe", "upper", false},
		{"lower is safe", "lower", false},
		{"printf is safe", "printf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDangerousFunction(tt.funcName); got != tt.want {
				t.Errorf("IsDangerousFunction(%s) = %v, want %v", tt.funcName, got, tt.want)
			}
		})
	}
}
