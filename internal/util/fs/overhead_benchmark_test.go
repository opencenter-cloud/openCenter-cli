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

package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// BenchmarkReadFile_Direct benchmarks direct os.ReadFile for comparison
func BenchmarkReadFile_Direct(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := make([]byte, 1024) // 1KB
	
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(testFile)
		if err != nil {
			b.Fatalf("os.ReadFile failed: %v", err)
		}
	}
}

// BenchmarkReadFile_Wrapped benchmarks FileSystem.ReadFile
func BenchmarkReadFile_Wrapped(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := make([]byte, 1024) // 1KB
	
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.ReadFile(testFile)
		if err != nil {
			b.Fatalf("ReadFile failed: %v", err)
		}
	}
}

// BenchmarkWriteFile_Direct benchmarks direct os.WriteFile for comparison
func BenchmarkWriteFile_Direct(b *testing.B) {
	tmpDir := b.TempDir()
	testData := make([]byte, 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "test_"+string(rune('a'+i%26))+".txt")
		err := os.WriteFile(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("os.WriteFile failed: %v", err)
		}
	}
}

// BenchmarkWriteFile_Wrapped benchmarks FileSystem.WriteFile
func BenchmarkWriteFile_Wrapped(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testData := make([]byte, 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "test_"+string(rune('a'+i%26))+".txt")
		err := fs.WriteFile(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("WriteFile failed: %v", err)
		}
	}
}

// BenchmarkExists_Direct benchmarks direct file existence check
func BenchmarkExists_Direct(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = os.Stat(testFile)
	}
}

// BenchmarkExists_Wrapped benchmarks FileSystem.Exists
func BenchmarkExists_Wrapped(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fs.Exists(testFile)
	}
}

// BenchmarkMkdirAll_Direct benchmarks direct os.MkdirAll
func BenchmarkMkdirAll_Direct(b *testing.B) {
	tmpDir := b.TempDir()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDir := filepath.Join(tmpDir, "dir"+string(rune('a'+i%26)), "subdir")
		err := os.MkdirAll(testDir, 0755)
		if err != nil {
			b.Fatalf("os.MkdirAll failed: %v", err)
		}
	}
}

// BenchmarkMkdirAll_Wrapped benchmarks FileSystem.MkdirAll
func BenchmarkMkdirAll_Wrapped(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDir := filepath.Join(tmpDir, "dir"+string(rune('a'+i%26)), "subdir")
		err := fs.MkdirAll(testDir, 0755)
		if err != nil {
			b.Fatalf("MkdirAll failed: %v", err)
		}
	}
}

// BenchmarkLargeFile_Direct benchmarks direct read of large file
func BenchmarkLargeFile_Direct(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	testData := make([]byte, 1024*1024) // 1MB
	
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(testFile)
		if err != nil {
			b.Fatalf("os.ReadFile failed: %v", err)
		}
	}
}

// BenchmarkLargeFile_Wrapped benchmarks FileSystem read of large file
func BenchmarkLargeFile_Wrapped(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	testData := make([]byte, 1024*1024) // 1MB
	
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.ReadFile(testFile)
		if err != nil {
			b.Fatalf("ReadFile failed: %v", err)
		}
	}
}
