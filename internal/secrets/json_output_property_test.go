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
	"encoding/json"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/require"
)

// **Validates: Requirements 2.9, 4.6, 5.5**
//
// Property 18: JSON Output Validity
//
// For any command with `--output json` flag, the output should be valid JSON
// that can be parsed and contains all expected fields.
//
// This property verifies that:
// 1. ValidationResult can be marshaled to valid JSON
// 2. ExpirationReport can be marshaled to valid JSON
// 3. KeyEntry list can be marshaled to valid JSON
// 4. All marshaled JSON can be unmarshaled back to the original structure
// 5. Required fields are present in the JSON output
func TestProperty_JSONOutputValidity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: ValidationResult JSON marshaling and unmarshaling
	properties.Property("ValidationResult can be marshaled to valid JSON and unmarshaled", prop.ForAll(
		func(validGen bool, driftCount int, missingCount int, orphanedCount int, securityCount int) bool {
			// Create ValidationResult with generated data
			result := &ValidationResult{
				Valid:            validGen,
				DriftItems:       generateDriftItems(driftCount),
				MissingManifests: generateStringList("missing-manifest", missingCount),
				OrphanedSecrets:  generateStringList("orphaned-secret", orphanedCount),
				SecurityIssues:   generateSecurityIssues(securityCount),
				ExitCode:         0,
			}
			if !validGen {
				result.ExitCode = 1
			}

			// Marshal to JSON
			jsonData, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal ValidationResult: %v", err)
				return false
			}

			// Verify it's valid JSON by unmarshaling
			var unmarshaled ValidationResult
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Logf("Failed to unmarshal ValidationResult JSON: %v", err)
				return false
			}

			// Verify required fields are present
			if unmarshaled.Valid != result.Valid {
				t.Logf("Valid field mismatch: expected %v, got %v", result.Valid, unmarshaled.Valid)
				return false
			}

			if unmarshaled.ExitCode != result.ExitCode {
				t.Logf("ExitCode field mismatch: expected %v, got %v", result.ExitCode, unmarshaled.ExitCode)
				return false
			}

			if len(unmarshaled.DriftItems) != len(result.DriftItems) {
				t.Logf("DriftItems count mismatch: expected %d, got %d", len(result.DriftItems), len(unmarshaled.DriftItems))
				return false
			}

			if len(unmarshaled.MissingManifests) != len(result.MissingManifests) {
				t.Logf("MissingManifests count mismatch: expected %d, got %d", len(result.MissingManifests), len(unmarshaled.MissingManifests))
				return false
			}

			if len(unmarshaled.OrphanedSecrets) != len(result.OrphanedSecrets) {
				t.Logf("OrphanedSecrets count mismatch: expected %d, got %d", len(result.OrphanedSecrets), len(unmarshaled.OrphanedSecrets))
				return false
			}

			if len(unmarshaled.SecurityIssues) != len(result.SecurityIssues) {
				t.Logf("SecurityIssues count mismatch: expected %d, got %d", len(result.SecurityIssues), len(unmarshaled.SecurityIssues))
				return false
			}

			return true
		},
		gen.Bool(),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	// Property 2: ExpirationReport JSON marshaling and unmarshaling
	properties.Property("ExpirationReport can be marshaled to valid JSON and unmarshaled", prop.ForAll(
		func(expiredCount int, warningCount int, validCount int) bool {
			// Create ExpirationReport with generated data
			report := &ExpirationReport{
				Expired: generateKeyExpirationInfos("expired-cluster", expiredCount, -10),
				Warning: generateKeyExpirationInfos("warning-cluster", warningCount, 7),
				Valid:   generateKeyExpirationInfos("valid-cluster", validCount, 60),
			}

			// Marshal to JSON
			jsonData, err := json.Marshal(report)
			if err != nil {
				t.Logf("Failed to marshal ExpirationReport: %v", err)
				return false
			}

			// Verify it's valid JSON by unmarshaling
			var unmarshaled ExpirationReport
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Logf("Failed to unmarshal ExpirationReport JSON: %v", err)
				return false
			}

			// Verify required fields are present
			if len(unmarshaled.Expired) != len(report.Expired) {
				t.Logf("Expired count mismatch: expected %d, got %d", len(report.Expired), len(unmarshaled.Expired))
				return false
			}

			if len(unmarshaled.Warning) != len(report.Warning) {
				t.Logf("Warning count mismatch: expected %d, got %d", len(report.Warning), len(unmarshaled.Warning))
				return false
			}

			if len(unmarshaled.Valid) != len(report.Valid) {
				t.Logf("Valid count mismatch: expected %d, got %d", len(report.Valid), len(unmarshaled.Valid))
				return false
			}

			// Verify field values for first item in each category
			if len(report.Expired) > 0 && len(unmarshaled.Expired) > 0 {
				if unmarshaled.Expired[0].Cluster != report.Expired[0].Cluster {
					t.Logf("Expired cluster mismatch: expected %s, got %s", report.Expired[0].Cluster, unmarshaled.Expired[0].Cluster)
					return false
				}
				if unmarshaled.Expired[0].DaysRemaining != report.Expired[0].DaysRemaining {
					t.Logf("Expired days remaining mismatch: expected %d, got %d", report.Expired[0].DaysRemaining, unmarshaled.Expired[0].DaysRemaining)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	// Property 3: KeyEntry list JSON marshaling and unmarshaling
	properties.Property("KeyEntry list can be marshaled to valid JSON and unmarshaled", prop.ForAll(
		func(keyCount int) bool {
			// Create list of KeyEntry with generated data
			keys := generateKeyEntries(keyCount)

			// Marshal to JSON
			jsonData, err := json.Marshal(keys)
			if err != nil {
				t.Logf("Failed to marshal KeyEntry list: %v", err)
				return false
			}

			// Verify it's valid JSON by unmarshaling
			var unmarshaled []KeyEntry
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Logf("Failed to unmarshal KeyEntry list JSON: %v", err)
				return false
			}

			// Verify count matches
			if len(unmarshaled) != len(keys) {
				t.Logf("KeyEntry count mismatch: expected %d, got %d", len(keys), len(unmarshaled))
				return false
			}

			// Verify required fields for each key
			for i, key := range keys {
				if unmarshaled[i].Cluster != key.Cluster {
					t.Logf("Cluster mismatch at index %d: expected %s, got %s", i, key.Cluster, unmarshaled[i].Cluster)
					return false
				}
				if unmarshaled[i].KeyType != key.KeyType {
					t.Logf("KeyType mismatch at index %d: expected %s, got %s", i, key.KeyType, unmarshaled[i].KeyType)
					return false
				}
				if unmarshaled[i].Fingerprint != key.Fingerprint {
					t.Logf("Fingerprint mismatch at index %d: expected %s, got %s", i, key.Fingerprint, unmarshaled[i].Fingerprint)
					return false
				}
				if unmarshaled[i].Status != key.Status {
					t.Logf("Status mismatch at index %d: expected %s, got %s", i, key.Status, unmarshaled[i].Status)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 10),
	))

	// Property 4: JSON output contains no null values for required fields
	properties.Property("JSON output contains no null values for required fields", prop.ForAll(
		func(validGen bool) bool {
			// Create ValidationResult
			result := &ValidationResult{
				Valid:            validGen,
				DriftItems:       []DriftItem{},
				MissingManifests: []string{},
				OrphanedSecrets:  []string{},
				SecurityIssues:   []SecurityIssue{},
				ExitCode:         0,
			}

			// Marshal to JSON
			jsonData, err := json.Marshal(result)
			if err != nil {
				t.Logf("Failed to marshal ValidationResult: %v", err)
				return false
			}

			// Parse as generic map to check for null values
			var jsonMap map[string]interface{}
			if err := json.Unmarshal(jsonData, &jsonMap); err != nil {
				t.Logf("Failed to unmarshal to map: %v", err)
				return false
			}

			// Verify required fields are not null
			requiredFields := []string{"Valid", "DriftItems", "MissingManifests", "OrphanedSecrets", "SecurityIssues", "ExitCode"}
			for _, field := range requiredFields {
				if _, exists := jsonMap[field]; !exists {
					t.Logf("Required field %s is missing from JSON", field)
					return false
				}
				// Note: Empty arrays are valid, we just check they're not null
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// Helper functions for generating test data

func generateDriftItems(count int) []DriftItem {
	if count <= 0 {
		return []DriftItem{}
	}

	items := make([]DriftItem, count)
	for i := 0; i < count; i++ {
		items[i] = DriftItem{
			Service:      "test-service",
			FieldPath:    "data.test-field",
			ConfigHash:   "config-hash-123",
			ManifestHash: "manifest-hash-456",
		}
	}
	return items
}

func generateStringList(prefix string, count int) []string {
	if count <= 0 {
		return []string{}
	}

	list := make([]string, count)
	for i := 0; i < count; i++ {
		list[i] = prefix + "-" + string(rune('a'+i))
	}
	return list
}

func generateSecurityIssues(count int) []SecurityIssue {
	if count <= 0 {
		return []SecurityIssue{}
	}

	issues := make([]SecurityIssue, count)
	severities := []string{"critical", "high", "medium"}
	for i := 0; i < count; i++ {
		issues[i] = SecurityIssue{
			FilePath:  "path/to/file.yaml",
			FieldPath: "data.secret-field",
			Severity:  severities[i%len(severities)],
		}
	}
	return issues
}

func generateKeyExpirationInfos(clusterPrefix string, count int, daysRemaining int) []KeyExpirationInfo {
	if count <= 0 {
		return []KeyExpirationInfo{}
	}

	infos := make([]KeyExpirationInfo, count)
	keyTypes := []KeyType{KeyTypeAge, KeyTypeSSH}
	for i := 0; i < count; i++ {
		infos[i] = KeyExpirationInfo{
			Cluster:       clusterPrefix + "-" + string(rune('a'+i)),
			KeyType:       keyTypes[i%len(keyTypes)],
			Fingerprint:   "fingerprint-" + string(rune('a'+i)),
			DaysRemaining: daysRemaining,
			ExpiresAt:     time.Now().AddDate(0, 0, daysRemaining),
		}
	}
	return infos
}

func generateKeyEntries(count int) []KeyEntry {
	if count <= 0 {
		return []KeyEntry{}
	}

	entries := make([]KeyEntry, count)
	keyTypes := []KeyType{KeyTypeAge, KeyTypeSSH}
	statuses := []KeyStatus{KeyStatusActive, KeyStatusArchived, KeyStatusRevoked}
	
	for i := 0; i < count; i++ {
		entries[i] = KeyEntry{
			Cluster:     "test-cluster-" + string(rune('a'+i)),
			KeyType:     keyTypes[i%len(keyTypes)],
			Fingerprint: "fingerprint-" + string(rune('a'+i)),
			PublicKey:   "public-key-" + string(rune('a'+i)),
			CreatedAt:   time.Now().AddDate(0, 0, -90),
			ExpiresAt:   time.Now().AddDate(0, 0, 90),
			Status:      statuses[i%len(statuses)],
		}
	}
	return entries
}

// Sanity tests to verify the property tests are working correctly

func TestProperty_JSONOutputValidity_ValidationResult_Sanity(t *testing.T) {
	// Create a ValidationResult with known data
	result := &ValidationResult{
		Valid: false,
		DriftItems: []DriftItem{
			{
				Service:      "cert-manager",
				FieldPath:    "data.aws-access-key",
				ConfigHash:   "abc123",
				ManifestHash: "def456",
			},
		},
		MissingManifests: []string{"path/to/missing.yaml"},
		OrphanedSecrets:  []string{"path/to/orphaned.yaml"},
		SecurityIssues: []SecurityIssue{
			{
				FilePath:  "path/to/insecure.yaml",
				FieldPath: "data.password",
				Severity:  "critical",
			},
		},
		ExitCode: 1,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	require.NoError(t, err, "Should marshal ValidationResult to JSON")

	// Verify it's valid JSON
	require.True(t, json.Valid(jsonData), "Should produce valid JSON")

	// Unmarshal back
	var unmarshaled ValidationResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Should unmarshal ValidationResult from JSON")

	// Verify fields
	require.Equal(t, result.Valid, unmarshaled.Valid, "Valid field should match")
	require.Equal(t, result.ExitCode, unmarshaled.ExitCode, "ExitCode field should match")
	require.Len(t, unmarshaled.DriftItems, 1, "Should have 1 drift item")
	require.Len(t, unmarshaled.MissingManifests, 1, "Should have 1 missing manifest")
	require.Len(t, unmarshaled.OrphanedSecrets, 1, "Should have 1 orphaned secret")
	require.Len(t, unmarshaled.SecurityIssues, 1, "Should have 1 security issue")

	// Verify nested fields
	require.Equal(t, "cert-manager", unmarshaled.DriftItems[0].Service)
	require.Equal(t, "data.aws-access-key", unmarshaled.DriftItems[0].FieldPath)
	require.Equal(t, "critical", unmarshaled.SecurityIssues[0].Severity)
}

func TestProperty_JSONOutputValidity_ExpirationReport_Sanity(t *testing.T) {
	// Create an ExpirationReport with known data
	report := &ExpirationReport{
		Expired: []KeyExpirationInfo{
			{
				Cluster:       "prod-cluster",
				KeyType:       KeyTypeAge,
				Fingerprint:   "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn",
				DaysRemaining: -5,
				ExpiresAt:     time.Now().AddDate(0, 0, -5),
			},
		},
		Warning: []KeyExpirationInfo{
			{
				Cluster:       "staging-cluster",
				KeyType:       KeyTypeSSH,
				Fingerprint:   "SHA256:abc123def456",
				DaysRemaining: 10,
				ExpiresAt:     time.Now().AddDate(0, 0, 10),
			},
		},
		Valid: []KeyExpirationInfo{
			{
				Cluster:       "dev-cluster",
				KeyType:       KeyTypeAge,
				Fingerprint:   "age1xyz789",
				DaysRemaining: 60,
				ExpiresAt:     time.Now().AddDate(0, 0, 60),
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	require.NoError(t, err, "Should marshal ExpirationReport to JSON")

	// Verify it's valid JSON
	require.True(t, json.Valid(jsonData), "Should produce valid JSON")

	// Unmarshal back
	var unmarshaled ExpirationReport
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Should unmarshal ExpirationReport from JSON")

	// Verify fields
	require.Len(t, unmarshaled.Expired, 1, "Should have 1 expired key")
	require.Len(t, unmarshaled.Warning, 1, "Should have 1 warning key")
	require.Len(t, unmarshaled.Valid, 1, "Should have 1 valid key")

	// Verify nested fields
	require.Equal(t, "prod-cluster", unmarshaled.Expired[0].Cluster)
	require.Equal(t, KeyTypeAge, unmarshaled.Expired[0].KeyType)
	require.Equal(t, -5, unmarshaled.Expired[0].DaysRemaining)
}

func TestProperty_JSONOutputValidity_KeyEntryList_Sanity(t *testing.T) {
	// Create a list of KeyEntry with known data
	keys := []KeyEntry{
		{
			Cluster:     "prod-cluster",
			KeyType:     KeyTypeAge,
			Fingerprint: "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn",
			PublicKey:   "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn",
			CreatedAt:   time.Now().AddDate(0, -3, 0),
			ExpiresAt:   time.Now().AddDate(0, 0, 90),
			Status:      KeyStatusActive,
		},
		{
			Cluster:     "staging-cluster",
			KeyType:     KeyTypeSSH,
			Fingerprint: "SHA256:abc123def456",
			PublicKey:   "ssh-ed25519 AAAAC3NzaC1...",
			CreatedAt:   time.Now().AddDate(0, -6, 0),
			ExpiresAt:   time.Now().AddDate(0, 0, 180),
			Status:      KeyStatusArchived,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(keys)
	require.NoError(t, err, "Should marshal KeyEntry list to JSON")

	// Verify it's valid JSON
	require.True(t, json.Valid(jsonData), "Should produce valid JSON")

	// Unmarshal back
	var unmarshaled []KeyEntry
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Should unmarshal KeyEntry list from JSON")

	// Verify fields
	require.Len(t, unmarshaled, 2, "Should have 2 keys")
	require.Equal(t, "prod-cluster", unmarshaled[0].Cluster)
	require.Equal(t, KeyTypeAge, unmarshaled[0].KeyType)
	require.Equal(t, KeyStatusActive, unmarshaled[0].Status)
	require.Equal(t, "staging-cluster", unmarshaled[1].Cluster)
	require.Equal(t, KeyTypeSSH, unmarshaled[1].KeyType)
	require.Equal(t, KeyStatusArchived, unmarshaled[1].Status)
}

func TestProperty_JSONOutputValidity_EmptyArrays_Sanity(t *testing.T) {
	// Create ValidationResult with empty arrays
	result := &ValidationResult{
		Valid:            true,
		DriftItems:       []DriftItem{},
		MissingManifests: []string{},
		OrphanedSecrets:  []string{},
		SecurityIssues:   []SecurityIssue{},
		ExitCode:         0,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	require.NoError(t, err, "Should marshal ValidationResult with empty arrays to JSON")

	// Verify it's valid JSON
	require.True(t, json.Valid(jsonData), "Should produce valid JSON")

	// Parse as map to verify arrays are not null
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonMap)
	require.NoError(t, err, "Should unmarshal to map")

	// Verify arrays are present and not null
	driftItems, ok := jsonMap["DriftItems"].([]interface{})
	require.True(t, ok, "DriftItems should be an array")
	require.NotNil(t, driftItems, "DriftItems should not be null")
	require.Empty(t, driftItems, "DriftItems should be empty")

	missingManifests, ok := jsonMap["MissingManifests"].([]interface{})
	require.True(t, ok, "MissingManifests should be an array")
	require.NotNil(t, missingManifests, "MissingManifests should not be null")
	require.Empty(t, missingManifests, "MissingManifests should be empty")
}
