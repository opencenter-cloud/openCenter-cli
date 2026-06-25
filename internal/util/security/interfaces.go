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

package security

import (
	"os"
	"time"
)

// CredentialMasker interface for masking sensitive data in logs and errors
type CredentialMasker interface {
	MaskString(input string) string
	MaskMap(data map[string]interface{}) map[string]interface{}
	MaskError(err error) error
	AddSensitivePattern(pattern string)
	AddSensitiveField(fieldName string)
	IsSensitiveField(fieldName string) bool
}

// SecureTempFileManager interface for secure temporary file operations
type SecureTempFileManager interface {
	CreateSecureTempFile(pattern string) (*SecureTempFile, error)
	CreateSecureTempDir(pattern string) (string, error)
	CleanupTempFile(path string) error
	CleanupTempDir(path string) error
	CleanupAll() error
}

// SecureTempFile represents a secure temporary file
type SecureTempFile struct {
	File        *os.File
	Path        string
	Permissions os.FileMode
	CreatedAt   time.Time
}

// Write writes data to the secure temporary file
func (stf *SecureTempFile) Write(data []byte) (int, error) {
	return stf.File.Write(data)
}

// Close closes the secure temporary file
func (stf *SecureTempFile) Close() error {
	return stf.File.Close()
}

// Remove removes the secure temporary file
func (stf *SecureTempFile) Remove() error {
	return os.Remove(stf.Path)
}


