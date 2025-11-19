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

package files

import (
	"os"
)

// FileOperator interface for basic file operations
type FileOperator interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	WriteFileAtomic(filename string, data []byte, perm os.FileMode) error
	AppendToFile(filename string, data []byte) error
	CopyFile(src, dst string) error
	MoveFile(src, dst string) error
	DeleteFile(filename string) error
	FileExists(filename string) bool
	GetFileInfo(filename string) (os.FileInfo, error)
}

// FileValidator interface for validating files and permissions
type FileValidator interface {
	ValidateFileExists(filename string) error
	ValidateFileReadable(filename string) error
	ValidateFileWritable(filename string) error
	ValidateFilePermissions(filename string, expectedPerm os.FileMode) error
	ValidateFileSize(filename string, maxSize int64) error
	ValidateFileExtension(filename string, allowedExtensions []string) error
	ValidateFileIsDirectory(filename string) error
	ValidateFileIsRegular(filename string) error
}

// AtomicFileWriter interface for atomic file operations
type AtomicFileWriter interface {
	WriteAtomic(filename string, data []byte, perm os.FileMode) error
	WriteAtomicWithBackup(filename string, data []byte, perm os.FileMode) error
	CreateTempFile(dir, pattern string) (*os.File, error)
	CommitTempFile(tempFile *os.File, finalPath string) error
}

// FileBackupManager interface for managing file backups
type FileBackupManager interface {
	CreateBackup(filename string) (string, error)
	RestoreBackup(backupPath, originalPath string) error
	CleanupBackups(pattern string, maxAge int64) error
	ListBackups(pattern string) ([]string, error)
}

// FileLockManager interface for file locking operations
type FileLockManager interface {
	LockFile(filename string) (FileLock, error)
	TryLockFile(filename string) (FileLock, bool, error)
	IsFileLocked(filename string) bool
}

// FileLock interface for file lock operations
type FileLock interface {
	Unlock() error
	IsLocked() bool
	GetPath() string
}

// FileWatcher interface for monitoring file changes
type FileWatcher interface {
	WatchFile(filename string, callback func(string)) error
	WatchDirectory(dirname string, callback func(string)) error
	StopWatching(path string) error
	StopAll() error
}

// FileCompressor interface for file compression operations
type FileCompressor interface {
	CompressFile(src, dst string) error
	DecompressFile(src, dst string) error
	CompressDirectory(srcDir, dstFile string) error
	DecompressDirectory(srcFile, dstDir string) error
}

// FileHasher interface for file hashing operations
type FileHasher interface {
	HashFile(filename string, algorithm string) (string, error)
	VerifyFileHash(filename string, expectedHash string, algorithm string) error
	HashDirectory(dirname string, algorithm string) (string, error)
}

// FileMetadata represents file metadata
type FileMetadata struct {
	Path        string      `json:"path"`
	Size        int64       `json:"size"`
	Mode        os.FileMode `json:"mode"`
	ModTime     int64       `json:"mod_time"`
	IsDir       bool        `json:"is_dir"`
	Hash        string      `json:"hash,omitempty"`
	Permissions string      `json:"permissions"`
	Owner       string      `json:"owner,omitempty"`
	Group       string      `json:"group,omitempty"`
}

// FileOperation represents a file operation for batch processing
type FileOperation struct {
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Destination string                 `json:"destination,omitempty"`
	Data        []byte                 `json:"data,omitempty"`
	Permissions os.FileMode            `json:"permissions,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// BatchFileProcessor interface for processing multiple file operations
type BatchFileProcessor interface {
	AddOperation(op FileOperation) error
	ExecuteOperations() error
	RollbackOperations() error
	GetOperationCount() int
	ClearOperations()
}
