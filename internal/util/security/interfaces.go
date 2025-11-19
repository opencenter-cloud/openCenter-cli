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
	"context"
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

// CredentialValidator interface for validating credentials don't leak
type CredentialValidator interface {
	ValidateNoCredentialsInConfig(configPath string) error
	ValidateNoCredentialsInLogs(logPath string) error
	ScanForCredentials(content string) []CredentialMatch
	ValidateEnvironmentVariables() error
}

// CredentialMatch represents a potential credential found in content
type CredentialMatch struct {
	Type     string `json:"type"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Context  string `json:"context"`
	Severity string `json:"severity"`
}

// AuditLogger interface for security audit logging
type AuditLogger interface {
	LogSecurityEvent(ctx context.Context, event SecurityEvent) error
	LogSOPSOperation(ctx context.Context, operation, keyName string, success bool) error
	LogConfigChange(ctx context.Context, configPath, operation string, changes map[string]interface{}) error
	LogCredentialAccess(ctx context.Context, credentialType, operation string) error
	LogAuthenticationAttempt(ctx context.Context, user, method string, success bool) error
	GetAuditLog(ctx context.Context, filter AuditFilter) ([]SecurityEvent, error)
}

// SecurityEvent represents a security-relevant event
type SecurityEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	Operation string                 `json:"operation"`
	User      string                 `json:"user,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Success   bool                   `json:"success"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Severity  string                 `json:"severity"`
	Source    string                 `json:"source"`
}

// AuditFilter represents filters for querying audit logs
type AuditFilter struct {
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	EventType string    `json:"event_type,omitempty"`
	Operation string    `json:"operation,omitempty"`
	User      string    `json:"user,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	Severity  string    `json:"severity,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// AtomicOperationManager interface for atomic operations with rollback
type AtomicOperationManager interface {
	BeginTransaction(ctx context.Context, name string) (Transaction, error)
	ExecuteAtomic(ctx context.Context, name string, operation func(Transaction) error) error
}

// Transaction interface for atomic operations
type Transaction interface {
	AddOperation(operation Operation) error
	Commit() error
	Rollback() error
	GetID() string
	GetName() string
	GetOperations() []Operation
	IsCommitted() bool
	IsRolledBack() bool
}

// Operation represents a single operation in a transaction
type Operation struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Execute     func() error           `json:"-"`
	Rollback    func() error           `json:"-"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ExecutedAt  time.Time              `json:"executed_at,omitempty"`
	Status      string                 `json:"status"`
}

// BackupManager interface for managing backups during atomic operations
type BackupManager interface {
	CreateBackup(ctx context.Context, resource string) (string, error)
	RestoreBackup(ctx context.Context, backupID string) error
	DeleteBackup(ctx context.Context, backupID string) error
	ListBackups(ctx context.Context, resource string) ([]BackupInfo, error)
	CleanupOldBackups(ctx context.Context, maxAge time.Duration) error
}

// BackupInfo represents information about a backup
type BackupInfo struct {
	ID        string                 `json:"id"`
	Resource  string                 `json:"resource"`
	CreatedAt time.Time              `json:"created_at"`
	Size      int64                  `json:"size"`
	Path      string                 `json:"path"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SecretRotationManager interface for managing secret rotation
type SecretRotationManager interface {
	RotateSecret(ctx context.Context, secretName string, newValue []byte) error
	RotateSOPSKeys(ctx context.Context, oldKeyPath, newKeyPath string) error
	ValidateRotation(ctx context.Context, secretName string) error
	RollbackRotation(ctx context.Context, secretName string) error
}
