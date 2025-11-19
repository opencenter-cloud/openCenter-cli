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

// Package files provides comprehensive file operation utilities with atomic operations and validation.
//
// This package extracts common file I/O operations to provide reusable, testable components
// for file manipulation, validation, atomic operations, and backup management with proper
// error handling and permission checking.
//
// Key interfaces:
//   - FileOperator: Basic file operations (read, write, copy, move, delete)
//   - FileValidator: File validation and permission checking
//   - AtomicFileWriter: Atomic file operations to prevent corruption
//   - FileBackupManager: File backup and restore operations
//
// Key features:
//   - Atomic file operations to prevent data corruption
//   - Comprehensive file validation and permission checking
//   - Backup and restore functionality with timestamp management
//   - Safe file operations with proper error handling
//   - Cross-platform compatibility with proper path handling
//
// The package ensures data integrity through atomic operations and provides
// robust error handling with detailed context for troubleshooting.
package files
