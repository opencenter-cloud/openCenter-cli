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
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultAtomicOperationManager implements AtomicOperationManager interface
type DefaultAtomicOperationManager struct {
	transactions map[string]*DefaultTransaction
	mu           sync.Mutex
	auditLogger  AuditLogger
}

// NewDefaultAtomicOperationManager creates a new atomic operation manager
func NewDefaultAtomicOperationManager(auditLogger AuditLogger) *DefaultAtomicOperationManager {
	return &DefaultAtomicOperationManager{
		transactions: make(map[string]*DefaultTransaction),
		auditLogger:  auditLogger,
	}
}

// BeginTransaction starts a new transaction
func (m *DefaultAtomicOperationManager) BeginTransaction(ctx context.Context, name string) (Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	txID := uuid.New().String()

	tx := &DefaultTransaction{
		id:          txID,
		name:        name,
		operations:  make([]Operation, 0),
		committed:   false,
		rolledBack:  false,
		createdAt:   time.Now(),
		auditLogger: m.auditLogger,
		ctx:         ctx,
	}

	m.transactions[txID] = tx

	// Log transaction start
	if m.auditLogger != nil {
		m.auditLogger.LogSecurityEvent(ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "transaction_start",
			Operation: "begin_transaction",
			Resource:  name,
			Success:   true,
			Severity:  "low",
			Details: map[string]interface{}{
				"transaction_id":   txID,
				"transaction_name": name,
			},
		})
	}

	return tx, nil
}

// ExecuteAtomic executes an operation atomically with automatic rollback on error
func (m *DefaultAtomicOperationManager) ExecuteAtomic(ctx context.Context, name string, operation func(Transaction) error) error {
	tx, err := m.BeginTransaction(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the operation
	if err := operation(tx); err != nil {
		// Rollback on error
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("operation failed: %w, rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("operation failed and rolled back: %w", err)
	}

	// Commit if successful
	if err := tx.Commit(); err != nil {
		// Attempt rollback if commit fails
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("commit failed: %w, rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("commit failed and rolled back: %w", err)
	}

	return nil
}

// DefaultTransaction implements Transaction interface
type DefaultTransaction struct {
	id          string
	name        string
	operations  []Operation
	committed   bool
	rolledBack  bool
	createdAt   time.Time
	auditLogger AuditLogger
	ctx         context.Context
	mu          sync.Mutex
}

// AddOperation adds an operation to the transaction
func (t *DefaultTransaction) AddOperation(operation Operation) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return fmt.Errorf("cannot add operation to committed transaction")
	}

	if t.rolledBack {
		return fmt.Errorf("cannot add operation to rolled back transaction")
	}

	// Set operation ID if not provided
	if operation.ID == "" {
		operation.ID = uuid.New().String()
	}

	// Set initial status
	operation.Status = "pending"

	t.operations = append(t.operations, operation)

	return nil
}

// Commit commits all operations in the transaction
func (t *DefaultTransaction) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	if t.rolledBack {
		return fmt.Errorf("cannot commit rolled back transaction")
	}

	// Execute all operations
	for i := range t.operations {
		op := &t.operations[i]

		if op.Execute == nil {
			continue
		}

		op.Status = "executing"
		op.ExecutedAt = time.Now()

		if err := op.Execute(); err != nil {
			op.Status = "failed"

			// Log failure
			if t.auditLogger != nil {
				t.auditLogger.LogSecurityEvent(t.ctx, SecurityEvent{
					Timestamp: time.Now(),
					EventType: "operation_failed",
					Operation: op.Type,
					Resource:  op.Description,
					Success:   false,
					Severity:  "high",
					Details: map[string]interface{}{
						"transaction_id": t.id,
						"operation_id":   op.ID,
						"error":          err.Error(),
					},
				})
			}

			// Rollback all previously executed operations
			t.rollbackExecutedOperations(i)

			return fmt.Errorf("operation %s failed: %w", op.ID, err)
		}

		op.Status = "completed"
	}

	t.committed = true

	// Log successful commit
	if t.auditLogger != nil {
		t.auditLogger.LogSecurityEvent(t.ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "transaction_commit",
			Operation: "commit",
			Resource:  t.name,
			Success:   true,
			Severity:  "medium",
			Details: map[string]interface{}{
				"transaction_id":   t.id,
				"transaction_name": t.name,
				"operations_count": len(t.operations),
			},
		})
	}

	return nil
}

// Rollback rolls back all executed operations
func (t *DefaultTransaction) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return fmt.Errorf("cannot rollback committed transaction")
	}

	if t.rolledBack {
		return nil // Already rolled back
	}

	// Rollback all executed operations in reverse order
	errors := make([]error, 0)

	for i := len(t.operations) - 1; i >= 0; i-- {
		op := &t.operations[i]

		if op.Status != "completed" && op.Status != "executing" {
			continue
		}

		if op.Rollback == nil {
			continue
		}

		if err := op.Rollback(); err != nil {
			errors = append(errors, fmt.Errorf("failed to rollback operation %s: %w", op.ID, err))
			op.Status = "rollback_failed"
		} else {
			op.Status = "rolled_back"
		}
	}

	t.rolledBack = true

	// Log rollback
	if t.auditLogger != nil {
		success := len(errors) == 0
		severity := "medium"
		if !success {
			severity = "high"
		}

		t.auditLogger.LogSecurityEvent(t.ctx, SecurityEvent{
			Timestamp: time.Now(),
			EventType: "transaction_rollback",
			Operation: "rollback",
			Resource:  t.name,
			Success:   success,
			Severity:  severity,
			Details: map[string]interface{}{
				"transaction_id":   t.id,
				"transaction_name": t.name,
				"operations_count": len(t.operations),
				"errors_count":     len(errors),
			},
		})
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with errors: %v", errors)
	}

	return nil
}

// rollbackExecutedOperations rolls back operations up to the specified index
func (t *DefaultTransaction) rollbackExecutedOperations(upToIndex int) {
	for i := upToIndex - 1; i >= 0; i-- {
		op := &t.operations[i]

		if op.Status != "completed" {
			continue
		}

		if op.Rollback == nil {
			continue
		}

		if err := op.Rollback(); err != nil {
			op.Status = "rollback_failed"
		} else {
			op.Status = "rolled_back"
		}
	}
}

// GetID returns the transaction ID
func (t *DefaultTransaction) GetID() string {
	return t.id
}

// GetName returns the transaction name
func (t *DefaultTransaction) GetName() string {
	return t.name
}

// GetOperations returns all operations in the transaction
func (t *DefaultTransaction) GetOperations() []Operation {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Return a copy to prevent external modification
	ops := make([]Operation, len(t.operations))
	copy(ops, t.operations)
	return ops
}

// IsCommitted returns true if the transaction is committed
func (t *DefaultTransaction) IsCommitted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.committed
}

// IsRolledBack returns true if the transaction is rolled back
func (t *DefaultTransaction) IsRolledBack() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rolledBack
}

// CreateFileOperation creates a file operation with rollback
func CreateFileOperation(opType, description, filePath string, execute, rollback func() error) Operation {
	return Operation{
		ID:          uuid.New().String(),
		Type:        opType,
		Description: description,
		Execute:     execute,
		Rollback:    rollback,
		Metadata: map[string]interface{}{
			"file_path": filePath,
		},
		Status: "pending",
	}
}

// CreateConfigOperation creates a configuration operation with rollback
func CreateConfigOperation(description, configPath string, execute, rollback func() error) Operation {
	return Operation{
		ID:          uuid.New().String(),
		Type:        "config_change",
		Description: description,
		Execute:     execute,
		Rollback:    rollback,
		Metadata: map[string]interface{}{
			"config_path": configPath,
		},
		Status: "pending",
	}
}

// CreateSOPSOperation creates a SOPS operation with rollback
func CreateSOPSOperation(description, keyPath string, execute, rollback func() error) Operation {
	return Operation{
		ID:          uuid.New().String(),
		Type:        "sops_operation",
		Description: description,
		Execute:     execute,
		Rollback:    rollback,
		Metadata: map[string]interface{}{
			"key_path": keyPath,
		},
		Status: "pending",
	}
}
