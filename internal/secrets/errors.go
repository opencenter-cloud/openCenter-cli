/*
Copyright 2025.

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

package secrets

import (
	"errors"
	"fmt"
)

// ErrConfigNotFound is returned when a cluster config file cannot be found.
type ErrConfigNotFound struct {
	Cluster      string
	ExpectedPath string
}

func (e *ErrConfigNotFound) Error() string {
	return fmt.Sprintf("config file not found for cluster %q (expected at: %s)", e.Cluster, e.ExpectedPath)
}

// ErrKeyNotFound is returned when an encryption key cannot be found.
type ErrKeyNotFound struct {
	Cluster string
	KeyType KeyType
}

func (e *ErrKeyNotFound) Error() string {
	return fmt.Sprintf("%s key not found for cluster %q", e.KeyType, e.Cluster)
}

// ErrDecryptionFailed is returned when a manifest cannot be decrypted.
type ErrDecryptionFailed struct {
	FilePath string
	Cause    error
}

func (e *ErrDecryptionFailed) Error() string {
	return fmt.Sprintf("failed to decrypt manifest %q: %v", e.FilePath, e.Cause)
}

func (e *ErrDecryptionFailed) Unwrap() error {
	return e.Cause
}

// ErrEncryptionFailed is returned when a manifest cannot be encrypted.
type ErrEncryptionFailed struct {
	FilePath string
	Cause    error
}

func (e *ErrEncryptionFailed) Error() string {
	return fmt.Sprintf("failed to encrypt manifest %q: %v", e.FilePath, e.Cause)
}

func (e *ErrEncryptionFailed) Unwrap() error {
	return e.Cause
}

// ErrRotationInProgress is returned when attempting an operation that conflicts with an ongoing rotation.
type ErrRotationInProgress struct {
	Cluster string
	KeyType KeyType
}

func (e *ErrRotationInProgress) Error() string {
	return fmt.Sprintf("%s key rotation already in progress for cluster %q", e.KeyType, e.Cluster)
}

// ErrSingleKeyRevocation is returned when attempting to revoke the only encryption key.
type ErrSingleKeyRevocation struct {
	Cluster string
	KeyType KeyType
}

func (e *ErrSingleKeyRevocation) Error() string {
	return fmt.Sprintf("cannot revoke the only %s key for cluster %q (add a new key first)", e.KeyType, e.Cluster)
}

// ErrRegistryCorrupted is returned when the key registry is invalid or corrupted.
type ErrRegistryCorrupted struct {
	Cause error
}

func (e *ErrRegistryCorrupted) Error() string {
	return fmt.Sprintf("key registry is corrupted: %v", e.Cause)
}

func (e *ErrRegistryCorrupted) Unwrap() error {
	return e.Cause
}

// NewKeyNotFoundError creates a new ErrKeyNotFound error.
func NewKeyNotFoundError(cluster string, keyType KeyType, cause error) error {
	err := &ErrKeyNotFound{
		Cluster: cluster,
		KeyType: keyType,
	}
	if cause != nil {
		return fmt.Errorf("%w: %v", err, cause)
	}
	return err
}

// IsKeyNotFoundError checks if an error is or wraps an ErrKeyNotFound.
func IsKeyNotFoundError(err error) bool {
	var keyNotFoundErr *ErrKeyNotFound
	return errors.As(err, &keyNotFoundErr)
}

// NewRegistryCorruptedError creates a new ErrRegistryCorrupted error.
func NewRegistryCorruptedError(path string, cause error) error {
	return &ErrRegistryCorrupted{
		Cause: fmt.Errorf("registry at %s is corrupted: %w", path, cause),
	}
}
