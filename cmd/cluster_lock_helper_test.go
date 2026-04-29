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

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
	"github.com/spf13/cobra"
)

func TestAcquireLockWithPrompt_NoExistingLock(t *testing.T) {
	// Create a temporary lock directory
	tmpDir := t.TempDir()
	lockDir := filepath.Join(tmpDir, "locks")

	// Set up lock manager config
	origConfig := resilience.DefaultLockConfig
	resilience.DefaultLockConfig.LockDir = lockDir
	defer func() { resilience.DefaultLockConfig = origConfig }()

	// Create a test command
	cmd := &cobra.Command{}
	cmd.Flags().Bool("break-lock", false, "")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	ctx := context.Background()
	result, err := AcquireLockWithPrompt(ctx, cmd, "test-cluster", "test-operation", 1*time.Hour, map[string]string{
		"operation": "test",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Lock == nil {
		t.Fatal("expected lock, got nil")
	}
	if result.ExistingLock != nil {
		t.Error("expected no existing lock")
	}
	if result.WasBroken {
		t.Error("expected WasBroken to be false")
	}

	// Clean up
	result.LockManager.Release(result.Lock)
}

func TestAcquireLockWithPrompt_WithBreakLockFlag(t *testing.T) {
	// Create a temporary lock directory
	tmpDir := t.TempDir()
	lockDir := filepath.Join(tmpDir, "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatalf("failed to create lock dir: %v", err)
	}

	// Set up lock manager config
	origConfig := resilience.DefaultLockConfig
	resilience.DefaultLockConfig.LockDir = lockDir
	defer func() { resilience.DefaultLockConfig = origConfig }()

	// Create an existing lock file with a future expiry so it's still active
	lockPath := filepath.Join(lockDir, "test-cluster.lock")
	futureExpiry := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
	recentAcquire := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	lockContent := "owner=other-host:99999\nacquired=" + recentAcquire + "\nexpires=" + futureExpiry + "\nttl=1h0m0s\noperation=deploy\ncommand=cluster deploy\n"
	if err := os.WriteFile(lockPath, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Create a test command with --break-lock flag set
	cmd := &cobra.Command{}
	cmd.Flags().Bool("break-lock", false, "")
	cmd.Flags().Set("break-lock", "true")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	ctx := context.Background()
	result, err := AcquireLockWithPrompt(ctx, cmd, "test-cluster", "destroy", 1*time.Hour, map[string]string{
		"operation": "destroy",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Lock == nil {
		t.Fatal("expected lock, got nil")
	}
	if result.ExistingLock == nil {
		t.Error("expected existing lock info")
	}
	if !result.WasBroken {
		t.Error("expected WasBroken to be true")
	}
	if result.UserConfirmed {
		t.Error("expected UserConfirmed to be false (--break-lock was used)")
	}

	// Verify the output mentions breaking the lock
	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Broke existing lock")) {
		t.Errorf("expected output to mention breaking lock, got: %s", output)
	}

	// Clean up
	result.LockManager.Release(result.Lock)
}

func TestAcquireLockWithPrompt_WithoutBreakLockFlag_Confirmed(t *testing.T) {
	// Create a temporary lock directory
	tmpDir := t.TempDir()
	lockDir := filepath.Join(tmpDir, "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatalf("failed to create lock dir: %v", err)
	}

	// Set up lock manager config
	origConfig := resilience.DefaultLockConfig
	resilience.DefaultLockConfig.LockDir = lockDir
	defer func() { resilience.DefaultLockConfig = origConfig }()

	// Create an existing lock file with a future expiry so it's still active
	lockPath := filepath.Join(lockDir, "test-cluster.lock")
	futureExpiry := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
	recentAcquire := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	lockContent := "owner=other-host:99999\nacquired=" + recentAcquire + "\nexpires=" + futureExpiry + "\nttl=1h0m0s\noperation=deploy\ncommand=cluster deploy\n"
	if err := os.WriteFile(lockPath, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Set test mode to use non-interactive prompter that returns true (default)
	os.Setenv("OPENCENTER_TEST_MODE", "1")
	defer os.Unsetenv("OPENCENTER_TEST_MODE")

	// Create a test command without --break-lock flag
	cmd := &cobra.Command{}
	cmd.Flags().Bool("break-lock", false, "")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	ctx := context.Background()
	result, err := AcquireLockWithPrompt(ctx, cmd, "test-cluster", "destroy", 1*time.Hour, map[string]string{
		"operation": "destroy",
	})

	// In test mode, the prompter returns true, so we expect success
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Lock == nil {
		t.Fatal("expected lock, got nil")
	}
	if result.ExistingLock == nil {
		t.Error("expected existing lock info")
	}
	if !result.WasBroken {
		t.Error("expected WasBroken to be true")
	}
	if !result.UserConfirmed {
		t.Error("expected UserConfirmed to be true (user confirmed via prompt)")
	}

	// Verify the output mentions breaking the lock
	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Broke existing lock")) {
		t.Errorf("expected output to mention breaking lock, got: %s", output)
	}

	// Clean up
	result.LockManager.Release(result.Lock)
}

func TestAcquireLockWithPrompt_ExpiredLock_AutoCleanup(t *testing.T) {
	// Create a temporary lock directory
	tmpDir := t.TempDir()
	lockDir := filepath.Join(tmpDir, "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatalf("failed to create lock dir: %v", err)
	}

	// Set up lock manager config
	origConfig := resilience.DefaultLockConfig
	resilience.DefaultLockConfig.LockDir = lockDir
	defer func() { resilience.DefaultLockConfig = origConfig }()

	// Create an existing lock file that has already expired
	lockPath := filepath.Join(lockDir, "test-cluster.lock")
	pastExpiry := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	pastAcquire := time.Now().Add(-90 * time.Minute).Format(time.RFC3339)
	lockContent := "owner=other-host:99999\nacquired=" + pastAcquire + "\nexpires=" + pastExpiry + "\nttl=1h0m0s\noperation=deploy\ncommand=cluster deploy\n"
	if err := os.WriteFile(lockPath, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Create a test command without --break-lock flag
	cmd := &cobra.Command{}
	cmd.Flags().Bool("break-lock", false, "")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	ctx := context.Background()
	result, err := AcquireLockWithPrompt(ctx, cmd, "test-cluster", "deploy", 1*time.Hour, map[string]string{
		"operation": "deploy",
	})

	// Expired lock should be auto-cleaned without prompting
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Lock == nil {
		t.Fatal("expected lock, got nil")
	}
	if result.ExistingLock == nil {
		t.Error("expected existing lock info to be preserved")
	}
	if !result.WasBroken {
		t.Error("expected WasBroken to be true")
	}
	if result.UserConfirmed {
		t.Error("expected UserConfirmed to be false (expired lock should not prompt)")
	}

	// Verify the output mentions cleaning up the expired lock
	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Cleaned up expired lock")) {
		t.Errorf("expected output to mention cleaning up expired lock, got: %s", output)
	}

	// Clean up
	result.LockManager.Release(result.Lock)
}
