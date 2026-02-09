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

// BenchmarkReadFile benchmarks file reading
func BenchmarkReadFile(b *testing.B) {
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

// BenchmarkWriteFile benchmarks file writing
func BenchmarkWriteFile(b *testing.B) {
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

// BenchmarkWriteFileAtomic benchmarks atomic file writing
func BenchmarkWriteFileAtomic(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testData := make([]byte, 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "test_"+string(rune('a'+i%26))+".txt")
		err := fs.WriteFileAtomic(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("WriteFileAtomic failed: %v", err)
		}
	}
}

// BenchmarkWriteFileAtomic_Overwrite benchmarks atomic overwriting
func BenchmarkWriteFileAtomic_Overwrite(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := make([]byte, 1024) // 1KB
	
	// Create initial file
	if err := fs.WriteFileAtomic(testFile, testData, 0644); err != nil {
		b.Fatalf("failed to create initial file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := fs.WriteFileAtomic(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("WriteFileAtomic failed: %v", err)
		}
	}
}

// BenchmarkExists benchmarks file existence checking
func BenchmarkExists(b *testing.B) {
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

// BenchmarkStat benchmarks file stat operations
func BenchmarkStat(b *testing.B) {
	fs := NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking())
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.Stat(testFile)
		if err != nil {
			b.Fatalf("Stat failed: %v", err)
		}
	}
}

// BenchmarkGenerateRandomString benchmarks random string generation
func BenchmarkGenerateRandomString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateRandomString(8)
	}
}

// BenchmarkGenerateRandomString_16 benchmarks 16-character random strings
func BenchmarkGenerateRandomString_16(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateRandomString(16)
	}
}

// BenchmarkGenerateRandomString_32 benchmarks 32-character random strings
func BenchmarkGenerateRandomString_32(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateRandomString(32)
	}
}
