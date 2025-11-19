package pulumi

import (
	"context"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: talos-openstack-provider, Property 23: Secrets passphrase encryption
// For any generated Pulumi secrets provider passphrase, the passphrase should be
// encrypted using SOPS with Barbican key references before storage.
// Validates: Requirements 10.6, 10.7
func TestProperty_SecretsPassphraseEncryption(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("generated passphrases are encrypted with SOPS and Barbican", prop.ForAll(
		func(barbicanKeyID string) bool {
			// Create secrets provider
			logger := &testLogger{}
			provider, err := NewSecretsProvider(logger)
			if err != nil {
				t.Logf("Failed to create secrets provider: %v", err)
				return false
			}

			ctx := context.Background()

			// Generate passphrase
			passphrase, err := provider.GeneratePassphrase(ctx)
			if err != nil {
				t.Logf("Failed to generate passphrase: %v", err)
				return false
			}

			// Verify passphrase is not empty
			if passphrase == "" {
				t.Log("Generated passphrase is empty")
				return false
			}

			// Verify passphrase is not encrypted yet
			if provider.IsEncrypted() {
				t.Log("Passphrase should not be encrypted immediately after generation")
				return false
			}

			// Encrypt passphrase with Barbican key reference
			encryptedPassphrase, err := provider.EncryptPassphrase(ctx, passphrase, barbicanKeyID)
			if err != nil {
				t.Logf("Failed to encrypt passphrase: %v", err)
				return false
			}

			// Verify encrypted passphrase is not empty
			if encryptedPassphrase == "" {
				t.Log("Encrypted passphrase is empty")
				return false
			}

			// Verify encrypted passphrase is different from original
			if encryptedPassphrase == passphrase {
				t.Log("Encrypted passphrase should differ from original")
				return false
			}

			// Verify passphrase is marked as encrypted
			if !provider.IsEncrypted() {
				t.Log("Passphrase should be marked as encrypted")
				return false
			}

			// Verify encrypted passphrase has SOPS format
			if !strings.HasPrefix(encryptedPassphrase, "ENC[") && !strings.HasPrefix(encryptedPassphrase, "sops:") {
				t.Logf("Encrypted passphrase does not have SOPS format: %s", encryptedPassphrase)
				return false
			}

			// Validate the encrypted passphrase
			err = provider.ValidatePassphrase(ctx, encryptedPassphrase)
			if err != nil {
				t.Logf("Encrypted passphrase validation failed: %v", err)
				return false
			}

			return true
		},
		genBarbicanKeyID(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_PassphraseValidation tests that unencrypted passphrases are rejected.
func TestProperty_PassphraseValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("unencrypted passphrases are rejected", prop.ForAll(
		func(plainPassphrase string) bool {
			// Create secrets provider
			logger := &testLogger{}
			provider, err := NewSecretsProvider(logger)
			if err != nil {
				t.Logf("Failed to create secrets provider: %v", err)
				return false
			}

			ctx := context.Background()

			// Try to validate a plain (unencrypted) passphrase
			err = provider.ValidatePassphrase(ctx, plainPassphrase)

			// Should fail validation because it's not SOPS-encrypted
			if err == nil {
				t.Log("Plain passphrase should not pass validation")
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v interface{}) bool {
			s := v.(string)
			// Ensure it doesn't look like SOPS encryption
			return !strings.HasPrefix(s, "ENC[") && !strings.HasPrefix(s, "sops:")
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_PassphraseStorageAndRetrieval tests passphrase storage and retrieval.
func TestProperty_PassphraseStorageAndRetrieval(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("encrypted passphrases can be stored and retrieved", prop.ForAll(
		func(barbicanKeyID string, filePath string) bool {
			// Create secrets provider
			logger := &testLogger{}
			provider, err := NewSecretsProvider(logger)
			if err != nil {
				t.Logf("Failed to create secrets provider: %v", err)
				return false
			}

			ctx := context.Background()

			// Generate and encrypt passphrase
			passphrase, err := provider.GeneratePassphrase(ctx)
			if err != nil {
				t.Logf("Failed to generate passphrase: %v", err)
				return false
			}

			encryptedPassphrase, err := provider.EncryptPassphrase(ctx, passphrase, barbicanKeyID)
			if err != nil {
				t.Logf("Failed to encrypt passphrase: %v", err)
				return false
			}

			// Store encrypted passphrase
			err = provider.StoreEncryptedPassphrase(ctx, encryptedPassphrase, filePath)
			if err != nil {
				t.Logf("Failed to store encrypted passphrase: %v", err)
				return false
			}

			// Load encrypted passphrase
			loadedPassphrase, err := provider.LoadEncryptedPassphrase(ctx, filePath)
			if err != nil {
				t.Logf("Failed to load encrypted passphrase: %v", err)
				return false
			}

			// Verify loaded passphrase is not empty
			if loadedPassphrase == "" {
				t.Log("Loaded passphrase is empty")
				return false
			}

			// Verify loaded passphrase has SOPS format
			if !strings.HasPrefix(loadedPassphrase, "ENC[") && !strings.HasPrefix(loadedPassphrase, "sops:") {
				t.Logf("Loaded passphrase does not have SOPS format: %s", loadedPassphrase)
				return false
			}

			return true
		},
		genBarbicanKeyID(),
		genFilePath(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genBarbicanKeyID generates valid Barbican key IDs.
func genBarbicanKeyID() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		return len(v.(string)) > 0
	})
}

// genFilePath generates valid file paths.
func genFilePath() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		return len(v.(string)) > 0
	})
}
