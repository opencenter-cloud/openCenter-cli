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
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOptimizedYAMLMarshal(t *testing.T) {
	config := defaultConfig("test-cluster")

	// Test optimized marshal
	data, err := OptimizedYAMLMarshal(&config)
	if err != nil {
		t.Fatalf("OptimizedYAMLMarshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("OptimizedYAMLMarshal returned empty data")
	}

	// Verify it's valid YAML by unmarshaling
	var result Config
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Marshaled data is not valid YAML: %v", err)
	}

	// Verify cluster name is preserved
	if result.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", result.ClusterName())
	}
}

func TestOptimizedYAMLUnmarshal(t *testing.T) {
	config := defaultConfig("test-cluster")
	data, err := yaml.Marshal(&config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Test optimized unmarshal
	var result Config
	if err := OptimizedYAMLUnmarshal(data, &result); err != nil {
		t.Fatalf("OptimizedYAMLUnmarshal failed: %v", err)
	}

	// Verify cluster name is preserved
	if result.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", result.ClusterName())
	}
}

func TestMarshalConfigOptimized(t *testing.T) {
	config := defaultConfig("test-cluster")

	// Test convenience function
	data, err := MarshalConfigOptimized(&config)
	if err != nil {
		t.Fatalf("MarshalConfigOptimized failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("MarshalConfigOptimized returned empty data")
	}

	// Verify it's valid YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Marshaled data is not valid YAML: %v", err)
	}
}

func TestUnmarshalConfigOptimized(t *testing.T) {
	config := defaultConfig("test-cluster")
	data, err := yaml.Marshal(&config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Test convenience function
	var result Config
	if err := UnmarshalConfigOptimized(data, &result); err != nil {
		t.Fatalf("UnmarshalConfigOptimized failed: %v", err)
	}

	// Verify cluster name is preserved
	if result.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", result.ClusterName())
	}
}

func TestStreamingYAMLMarshal(t *testing.T) {
	config := defaultConfig("test-cluster")

	// Test streaming marshal
	data, err := StreamingYAMLMarshal(&config)
	if err != nil {
		t.Fatalf("StreamingYAMLMarshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("StreamingYAMLMarshal returned empty data")
	}

	// Verify it's valid YAML
	var result Config
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Marshaled data is not valid YAML: %v", err)
	}

	// Verify cluster name is preserved
	if result.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", result.ClusterName())
	}
}

func TestOptimizedYAMLMarshalEquivalence(t *testing.T) {
	config := defaultConfig("test-cluster")

	// Marshal with standard method
	standardData, err := yaml.Marshal(&config)
	if err != nil {
		t.Fatalf("Standard marshal failed: %v", err)
	}

	// Marshal with optimized method
	optimizedData, err := OptimizedYAMLMarshal(&config)
	if err != nil {
		t.Fatalf("Optimized marshal failed: %v", err)
	}

	// Both should produce valid YAML
	var standardResult, optimizedResult Config
	if err := yaml.Unmarshal(standardData, &standardResult); err != nil {
		t.Fatalf("Standard data is not valid YAML: %v", err)
	}
	if err := yaml.Unmarshal(optimizedData, &optimizedResult); err != nil {
		t.Fatalf("Optimized data is not valid YAML: %v", err)
	}

	// Both should have the same cluster name
	if standardResult.ClusterName() != optimizedResult.ClusterName() {
		t.Errorf("Cluster names differ: standard=%s, optimized=%s",
			standardResult.ClusterName(), optimizedResult.ClusterName())
	}
}

func TestBufferPooling(t *testing.T) {
	// Test that buffers are properly pooled and reused
	config := defaultConfig("test-cluster")

	// Get initial buffer
	buf1 := getBuffer()
	initialCap := buf1.Cap()
	putBuffer(buf1)

	// Marshal multiple times
	for i := 0; i < 10; i++ {
		_, err := OptimizedYAMLMarshal(&config)
		if err != nil {
			t.Fatalf("Marshal %d failed: %v", i, err)
		}
	}

	// Get buffer again and verify it was reused
	buf2 := getBuffer()
	if buf2.Cap() < initialCap {
		t.Errorf("Buffer capacity decreased: initial=%d, current=%d", initialCap, buf2.Cap())
	}
	putBuffer(buf2)
}

func TestOptimizedYAMLMarshalNil(t *testing.T) {
	_, err := MarshalConfigOptimized(nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
}

func TestOptimizedYAMLUnmarshalNil(t *testing.T) {
	err := UnmarshalConfigOptimized([]byte("test: data"), nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
}

func TestOptimizedYAMLUnmarshalEmpty(t *testing.T) {
	var config Config
	err := UnmarshalConfigOptimized([]byte{}, &config)
	if err == nil {
		t.Fatal("Expected error for empty data, got nil")
	}
}

// BenchmarkYAMLMarshalComparison compares standard vs optimized marshaling
func BenchmarkYAMLMarshalComparison(b *testing.B) {
	config := defaultConfig("test-cluster")

	b.Run("StandardMarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := yaml.Marshal(&config)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
		}
	})

	b.Run("OptimizedMarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := OptimizedYAMLMarshal(&config)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
		}
	})

	b.Run("StreamingMarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := StreamingYAMLMarshal(&config)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
		}
	})
}

// BenchmarkYAMLUnmarshalComparison compares standard vs optimized unmarshaling
func BenchmarkYAMLUnmarshalComparison(b *testing.B) {
	config := defaultConfig("test-cluster")
	data, err := yaml.Marshal(&config)
	if err != nil {
		b.Fatalf("Failed to marshal config: %v", err)
	}

	b.Run("StandardUnmarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var cfg Config
			err := yaml.Unmarshal(data, &cfg)
			if err != nil {
				b.Fatalf("Unmarshal failed: %v", err)
			}
		}
	})

	b.Run("OptimizedUnmarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var cfg Config
			err := OptimizedYAMLUnmarshal(data, &cfg)
			if err != nil {
				b.Fatalf("Unmarshal failed: %v", err)
			}
		}
	})
}

// BenchmarkBufferPooling benchmarks buffer pool performance
func BenchmarkBufferPooling(b *testing.B) {
	b.Run("WithPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := getBuffer()
			buf.WriteString("test data")
			putBuffer(buf)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(make([]byte, 0, 64*1024))
			buf.WriteString("test data")
		}
	})
}
