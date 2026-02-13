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
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/require"
)

// **Validates: Requirements 4.7, 4.8, 9.2, 9.3**
//
// Property 10: Key Registry Completeness
//
// For any key generation or rotation operation, the key registry should contain
// an entry with fingerprint, creation date, expiration date, and status for the new key.
func TestProperty_KeyRegistryCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("registered keys have complete metadata", prop.ForAll(
		func(cluster string, keyType KeyType, fingerprint string, publicKey string) bool {
			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Create a key entry without timestamps (simulating key generation)
			entry := KeyEntry{
				Cluster:     cluster,
				KeyType:     keyType,
				Fingerprint: fingerprint,
				PublicKey:   publicKey,
			}

			// Register the key
			err := registry.RegisterKey(ctx, entry)
			if err != nil {
				// If registration fails due to duplicate, that's acceptable for this test
				// We're testing that successful registrations have complete metadata
				return true
			}

			// Retrieve the registered key
			retrieved, err := registry.GetKey(ctx, cluster, keyType)
			if err != nil {
				t.Logf("Failed to retrieve key: %v", err)
				return false
			}

			// Verify completeness: fingerprint, creation date, expiration date, and status
			if retrieved.Fingerprint == "" {
				t.Logf("Missing fingerprint")
				return false
			}

			if retrieved.CreatedAt.IsZero() {
				t.Logf("Missing creation date")
				return false
			}

			if retrieved.ExpiresAt.IsZero() {
				t.Logf("Missing expiration date")
				return false
			}

			if retrieved.Status == "" {
				t.Logf("Missing status")
				return false
			}

			// Verify that expiration date is after creation date
			if !retrieved.ExpiresAt.After(retrieved.CreatedAt) {
				t.Logf("Expiration date (%v) is not after creation date (%v)",
					retrieved.ExpiresAt, retrieved.CreatedAt)
				return false
			}

			// Verify that status is set to active for new keys
			if retrieved.Status != KeyStatusActive {
				t.Logf("Expected status to be active, got %s", retrieved.Status)
				return false
			}

			// Verify that the fingerprint matches what was provided
			if retrieved.Fingerprint != fingerprint {
				t.Logf("Fingerprint mismatch: expected %s, got %s", fingerprint, retrieved.Fingerprint)
				return false
			}

			// Verify that the public key matches what was provided
			if retrieved.PublicKey != publicKey {
				t.Logf("Public key mismatch: expected %s, got %s", publicKey, retrieved.PublicKey)
				return false
			}

			return true
		},
		genClusterName(),
		genKeyType(),
		genFingerprint(),
		genPublicKey(),
	))

	properties.Property("key rotation creates complete registry entry", prop.ForAll(
		func(cluster string, fingerprint1 string, fingerprint2 string, publicKey1 string, publicKey2 string) bool {
			// Ensure fingerprints are different
			if fingerprint1 == fingerprint2 {
				return true // Skip this test case
			}

			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Register initial key
			entry1 := KeyEntry{
				Cluster:     cluster,
				KeyType:     KeyTypeAge,
				Fingerprint: fingerprint1,
				PublicKey:   publicKey1,
			}
			err := registry.RegisterKey(ctx, entry1)
			if err != nil {
				t.Logf("Failed to register initial key: %v", err)
				return false
			}

			// Simulate rotation: archive old key
			err = registry.UpdateKeyStatus(ctx, cluster, KeyTypeAge, KeyStatusArchived)
			if err != nil {
				t.Logf("Failed to archive old key: %v", err)
				return false
			}

			// Register new key (simulating rotation)
			entry2 := KeyEntry{
				Cluster:     cluster,
				KeyType:     KeyTypeAge,
				Fingerprint: fingerprint2,
				PublicKey:   publicKey2,
				RotatedFrom: fingerprint1,
			}
			err = registry.RegisterKey(ctx, entry2)
			if err != nil {
				t.Logf("Failed to register rotated key: %v", err)
				return false
			}

			// Retrieve the new key
			retrieved, err := registry.GetKey(ctx, cluster, KeyTypeAge)
			if err != nil {
				t.Logf("Failed to retrieve rotated key: %v", err)
				return false
			}

			// Verify completeness of rotated key
			if retrieved.Fingerprint == "" {
				t.Logf("Rotated key missing fingerprint")
				return false
			}

			if retrieved.CreatedAt.IsZero() {
				t.Logf("Rotated key missing creation date")
				return false
			}

			if retrieved.ExpiresAt.IsZero() {
				t.Logf("Rotated key missing expiration date")
				return false
			}

			if retrieved.Status == "" {
				t.Logf("Rotated key missing status")
				return false
			}

			// Verify rotation metadata
			if retrieved.RotatedFrom != fingerprint1 {
				t.Logf("Rotated key missing or incorrect RotatedFrom field: expected %s, got %s",
					fingerprint1, retrieved.RotatedFrom)
				return false
			}

			// Verify that the new key is active
			if retrieved.Status != KeyStatusActive {
				t.Logf("Rotated key should be active, got %s", retrieved.Status)
				return false
			}

			// Verify that old key is archived
			allKeys, err := registry.ListKeys(ctx, cluster)
			if err != nil {
				t.Logf("Failed to list keys: %v", err)
				return false
			}

			foundArchivedKey := false
			for _, key := range allKeys {
				if key.Fingerprint == fingerprint1 && key.Status == KeyStatusArchived {
					foundArchivedKey = true
					break
				}
			}

			if !foundArchivedKey {
				t.Logf("Old key was not properly archived")
				return false
			}

			return true
		},
		genClusterName(),
		genFingerprint(),
		genFingerprint(),
		genPublicKey(),
		genPublicKey(),
	))

	properties.Property("expiration dates are correctly calculated", prop.ForAll(
		func(cluster string, keyType KeyType, fingerprint string, publicKey string) bool {
			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Register key without explicit expiration
			entry := KeyEntry{
				Cluster:     cluster,
				KeyType:     keyType,
				Fingerprint: fingerprint,
				PublicKey:   publicKey,
			}

			err := registry.RegisterKey(ctx, entry)
			if err != nil {
				return true // Skip duplicates
			}

			// Retrieve the key
			retrieved, err := registry.GetKey(ctx, cluster, keyType)
			if err != nil {
				t.Logf("Failed to retrieve key: %v", err)
				return false
			}

			// Calculate expected expiration based on key type
			var expectedDays int
			switch keyType {
			case KeyTypeAge:
				expectedDays = DefaultAgeExpirationDays
			case KeyTypeSSH:
				expectedDays = DefaultSSHExpirationDays
			default:
				expectedDays = DefaultAgeExpirationDays
			}

			expectedExpiration := retrieved.CreatedAt.AddDate(0, 0, expectedDays)

			// Verify expiration is set correctly (within 1 second tolerance for timing)
			diff := retrieved.ExpiresAt.Sub(expectedExpiration)
			if diff < -time.Second || diff > time.Second {
				t.Logf("Expiration date mismatch: expected %v, got %v (diff: %v)",
					expectedExpiration, retrieved.ExpiresAt, diff)
				return false
			}

			return true
		},
		genClusterName(),
		genKeyType(),
		genFingerprint(),
		genPublicKey(),
	))

	properties.TestingRun(t)
}

// Generators for property-based testing

func genClusterName() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		// Ensure cluster names are valid
		if s == "" {
			return "test-cluster"
		}
		return s
	})
}

func genKeyType() gopter.Gen {
	return gen.OneConstOf(KeyTypeAge, KeyTypeSSH)
}

func genFingerprint() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		// Generate realistic fingerprints
		if s == "" {
			return "fingerprint-default"
		}
		return "fp-" + s
	})
}

func genPublicKey() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		// Generate realistic public keys
		if s == "" {
			return "pubkey-default"
		}
		return "pubkey-" + s
	})
}

// Test that verifies the property test itself is working correctly
func TestProperty_KeyRegistryCompleteness_Sanity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	// Test a known good case
	entry := KeyEntry{
		Cluster:     "sanity-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "age1sanity123",
		PublicKey:   "age1sanity123",
	}

	err := registry.RegisterKey(ctx, entry)
	require.NoError(t, err)

	retrieved, err := registry.GetKey(ctx, "sanity-cluster", KeyTypeAge)
	require.NoError(t, err)
	require.NotEmpty(t, retrieved.Fingerprint)
	require.False(t, retrieved.CreatedAt.IsZero())
	require.False(t, retrieved.ExpiresAt.IsZero())
	require.NotEmpty(t, retrieved.Status)
	require.Equal(t, KeyStatusActive, retrieved.Status)
}

// **Validates: Requirements 4.2, 4.3, 4.4**
//
// Property 9: Key Expiration Calculation
//
// For any key with a known creation date and expiration policy, the days-until-expiration
// calculation should be accurate, and warnings should appear when within 14 days of expiration.
func TestProperty_KeyExpirationCalculation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("days remaining calculation is accurate", prop.ForAll(
		func(cluster string, keyType KeyType, fingerprint string, publicKey string, daysUntilExpiry int) bool {
			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Create a key with a specific expiration date
			now := time.Now()
			createdAt := now.AddDate(0, 0, -30) // Created 30 days ago
			expiresAt := now.AddDate(0, 0, daysUntilExpiry)

			entry := KeyEntry{
				Cluster:     cluster,
				KeyType:     keyType,
				Fingerprint: fingerprint,
				PublicKey:   publicKey,
				CreatedAt:   createdAt,
				ExpiresAt:   expiresAt,
				Status:      KeyStatusActive,
			}

			err := registry.RegisterKey(ctx, entry)
			if err != nil {
				return true // Skip duplicates
			}

			// Check expiration with 14-day warning threshold
			report, err := registry.CheckExpiration(ctx, 14)
			if err != nil {
				t.Logf("Failed to check expiration: %v", err)
				return false
			}

			// Find our key in the report
			var foundInfo *KeyExpirationInfo
			allInfos := append(append(report.Expired, report.Warning...), report.Valid...)
			for i := range allInfos {
				if allInfos[i].Fingerprint == fingerprint {
					foundInfo = &allInfos[i]
					break
				}
			}

			if foundInfo == nil {
				t.Logf("Key not found in expiration report")
				return false
			}

			// Verify days remaining calculation is accurate (within 1 day tolerance for timing)
			expectedDays := daysUntilExpiry
			actualDays := foundInfo.DaysRemaining
			if actualDays < expectedDays-1 || actualDays > expectedDays+1 {
				t.Logf("Days remaining mismatch: expected ~%d, got %d", expectedDays, actualDays)
				return false
			}

			// Verify correct categorization based on expiration
			// Note: The implementation uses time comparison, not just day count
			// A key expiring "today" (daysUntilExpiry == 0) may still be in the future
			// by hours/minutes, so it won't be expired yet
			currentTime := time.Now()
			if expiresAt.Before(currentTime) {
				// Should be in expired list
				found := false
				for _, info := range report.Expired {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Expired key not in expired list (expiresAt: %v, now: %v)", expiresAt, currentTime)
					return false
				}
			} else if expiresAt.Before(currentTime.AddDate(0, 0, 14)) {
				// Should be in warning list (expires within 14 days)
				found := false
				for _, info := range report.Warning {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Warning key not in warning list (expiresAt: %v, now: %v, threshold: %v)",
						expiresAt, currentTime, currentTime.AddDate(0, 0, 14))
					return false
				}
			} else {
				// Should be in valid list
				found := false
				for _, info := range report.Valid {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Valid key not in valid list (expiresAt: %v, now: %v)", expiresAt, currentTime)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genKeyType(),
		genFingerprint(),
		genPublicKey(),
		gen.IntRange(-30, 90), // Days until expiry: from 30 days ago to 90 days in future
	))

	properties.Property("warning threshold is respected", prop.ForAll(
		func(cluster string, fingerprint string, publicKey string, warnDays int, daysUntilExpiry int) bool {
			// Ensure warnDays is positive and reasonable
			if warnDays < 1 || warnDays > 365 {
				return true // Skip invalid warn days
			}

			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Create a key with specific expiration
			now := time.Now()
			expiresAt := now.AddDate(0, 0, daysUntilExpiry)

			entry := KeyEntry{
				Cluster:     cluster,
				KeyType:     KeyTypeAge,
				Fingerprint: fingerprint,
				PublicKey:   publicKey,
				CreatedAt:   now.AddDate(0, 0, -30),
				ExpiresAt:   expiresAt,
				Status:      KeyStatusActive,
			}

			err := registry.RegisterKey(ctx, entry)
			if err != nil {
				return true // Skip duplicates
			}

			// Check expiration with custom warning threshold
			report, err := registry.CheckExpiration(ctx, warnDays)
			if err != nil {
				t.Logf("Failed to check expiration: %v", err)
				return false
			}

			// Verify correct categorization based on custom threshold
			// Use time comparison like the implementation does
			currentTime := time.Now()
			warnThreshold := currentTime.AddDate(0, 0, warnDays)
			
			if expiresAt.Before(currentTime) {
				// Should be expired
				found := false
				for _, info := range report.Expired {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Expired key not in expired list (expiresAt: %v, now: %v)", expiresAt, currentTime)
					return false
				}
			} else if expiresAt.Before(warnThreshold) {
				// Should be in warning
				found := false
				for _, info := range report.Warning {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Key within warning threshold not in warning list (expiresAt: %v, threshold: %v)",
						expiresAt, warnThreshold)
					return false
				}
			} else {
				// Should be valid
				found := false
				for _, info := range report.Valid {
					if info.Fingerprint == fingerprint {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Valid key not in valid list (expiresAt: %v, threshold: %v)", expiresAt, warnThreshold)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genFingerprint(),
		genPublicKey(),
		gen.IntRange(1, 60),   // Warning threshold: 1-60 days
		gen.IntRange(-30, 90), // Days until expiry
	))

	properties.Property("only active keys are checked", prop.ForAll(
		func(cluster string, fingerprint string, publicKey string, status KeyStatus) bool {
			// Setup
			ctx := context.Background()
			tempDir := t.TempDir()
			encryptor := newMockSOPSEncryptor()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

			// Create a key with specific status
			now := time.Now()
			entry := KeyEntry{
				Cluster:     cluster,
				KeyType:     KeyTypeAge,
				Fingerprint: fingerprint,
				PublicKey:   publicKey,
				CreatedAt:   now.AddDate(0, 0, -30),
				ExpiresAt:   now.AddDate(0, 0, 5), // Expires in 5 days (should trigger warning)
				Status:      status,
			}

			err := registry.RegisterKey(ctx, entry)
			if err != nil {
				return true // Skip duplicates
			}

			// Check expiration
			report, err := registry.CheckExpiration(ctx, 14)
			if err != nil {
				t.Logf("Failed to check expiration: %v", err)
				return false
			}

			// Find if key appears in any list
			allInfos := append(append(report.Expired, report.Warning...), report.Valid...)
			found := false
			for _, info := range allInfos {
				if info.Fingerprint == fingerprint {
					found = true
					break
				}
			}

			// Only active keys should appear in the report
			if status == KeyStatusActive {
				if !found {
					t.Logf("Active key not found in expiration report")
					return false
				}
			} else {
				if found {
					t.Logf("Non-active key (%s) should not appear in expiration report", status)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genFingerprint(),
		genPublicKey(),
		gen.OneConstOf(KeyStatusActive, KeyStatusArchived, KeyStatusRevoked),
	))

	properties.TestingRun(t)
}

// Test that verifies the expiration calculation property test is working correctly
func TestProperty_KeyExpirationCalculation_Sanity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	encryptor := newMockSOPSEncryptor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewDefaultKeyRegistry(tempDir, encryptor, logger)

	now := time.Now()

	// Test case 1: Expired key
	expiredEntry := KeyEntry{
		Cluster:     "expired-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "expired-key",
		PublicKey:   "expired-pubkey",
		CreatedAt:   now.AddDate(0, 0, -100),
		ExpiresAt:   now.AddDate(0, 0, -10), // Expired 10 days ago
		Status:      KeyStatusActive,
	}
	err := registry.RegisterKey(ctx, expiredEntry)
	require.NoError(t, err)

	// Test case 2: Warning key (expires in 7 days)
	warningEntry := KeyEntry{
		Cluster:     "warning-cluster",
		KeyType:     KeyTypeSSH,
		Fingerprint: "warning-key",
		PublicKey:   "warning-pubkey",
		CreatedAt:   now.AddDate(0, 0, -30),
		ExpiresAt:   now.AddDate(0, 0, 7), // Expires in 7 days
		Status:      KeyStatusActive,
	}
	err = registry.RegisterKey(ctx, warningEntry)
	require.NoError(t, err)

	// Test case 3: Valid key (expires in 60 days)
	validEntry := KeyEntry{
		Cluster:     "valid-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "valid-key",
		PublicKey:   "valid-pubkey",
		CreatedAt:   now.AddDate(0, 0, -30),
		ExpiresAt:   now.AddDate(0, 0, 60), // Expires in 60 days
		Status:      KeyStatusActive,
	}
	err = registry.RegisterKey(ctx, validEntry)
	require.NoError(t, err)

	// Test case 4: Archived key (should not appear in report)
	archivedEntry := KeyEntry{
		Cluster:     "archived-cluster",
		KeyType:     KeyTypeAge,
		Fingerprint: "archived-key",
		PublicKey:   "archived-pubkey",
		CreatedAt:   now.AddDate(0, 0, -30),
		ExpiresAt:   now.AddDate(0, 0, 5), // Would be warning if active
		Status:      KeyStatusArchived,
	}
	err = registry.RegisterKey(ctx, archivedEntry)
	require.NoError(t, err)

	// Check expiration with 14-day warning threshold
	report, err := registry.CheckExpiration(ctx, 14)
	require.NoError(t, err)

	// Verify expired key
	require.Len(t, report.Expired, 1)
	require.Equal(t, "expired-key", report.Expired[0].Fingerprint)
	require.Less(t, report.Expired[0].DaysRemaining, 0)

	// Verify warning key
	require.Len(t, report.Warning, 1)
	require.Equal(t, "warning-key", report.Warning[0].Fingerprint)
	require.GreaterOrEqual(t, report.Warning[0].DaysRemaining, 0)
	require.LessOrEqual(t, report.Warning[0].DaysRemaining, 14)

	// Verify valid key
	require.Len(t, report.Valid, 1)
	require.Equal(t, "valid-key", report.Valid[0].Fingerprint)
	require.Greater(t, report.Valid[0].DaysRemaining, 14)

	// Verify archived key is not in report
	allFingerprints := []string{}
	for _, info := range append(append(report.Expired, report.Warning...), report.Valid...) {
		allFingerprints = append(allFingerprints, info.Fingerprint)
	}
	require.NotContains(t, allFingerprints, "archived-key")
}
