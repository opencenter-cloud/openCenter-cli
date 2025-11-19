package pulumi

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: talos-openstack-provider, Property 22: Swift backend initialization
// For any new cluster initialization, a dedicated Swift container should be created
// with versioning and server-side encryption enabled.
// Validates: Requirements 10.1, 10.2, 10.3
func TestProperty_SwiftBackendInitialization(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("Swift backend is initialized with versioning and encryption", prop.ForAll(
		func(containerName string, prefix string) bool {
			// Generate valid Swift backend configuration
			config := &SwiftBackendConfig{
				Container:         containerName,
				Prefix:            prefix,
				VersioningEnabled: true,
				EncryptionEnabled: true,
				AccessKey:         "test-access-key",
				SecretKey:         "test-secret-key",
				AuthURL:           "https://auth.example.com",
				Region:            "RegionOne",
			}

			// Create logger for testing
			logger := &testLogger{}

			// Create Swift backend
			backend, err := NewSwiftBackend(config, logger)
			if err != nil {
				t.Logf("Failed to create Swift backend: %v", err)
				return false
			}

			// Initialize backend
			ctx := context.Background()
			err = backend.Initialize(ctx)
			if err != nil {
				t.Logf("Failed to initialize Swift backend: %v", err)
				return false
			}

			// Verify configuration
			backendConfig := backend.GetConfig()
			if backendConfig.Container != containerName {
				t.Logf("Container name mismatch: expected %s, got %s", containerName, backendConfig.Container)
				return false
			}

			if backendConfig.Prefix != prefix {
				t.Logf("Prefix mismatch: expected %s, got %s", prefix, backendConfig.Prefix)
				return false
			}

			if !backendConfig.VersioningEnabled {
				t.Log("Versioning not enabled")
				return false
			}

			if !backendConfig.EncryptionEnabled {
				t.Log("Encryption not enabled")
				return false
			}

			// Verify backend URL format
			backendURL := backend.GetBackendURL()
			if prefix != "" {
				expectedURL := "s3://" + containerName + "/" + prefix
				if backendURL != expectedURL {
					t.Logf("Backend URL mismatch: expected %s, got %s", expectedURL, backendURL)
					return false
				}
			} else {
				expectedURL := "s3://" + containerName
				if backendURL != expectedURL {
					t.Logf("Backend URL mismatch: expected %s, got %s", expectedURL, backendURL)
					return false
				}
			}

			return true
		},
		genValidContainerName(),
		genValidPrefix(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genValidContainerName generates valid Swift container names.
func genValidContainerName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		// Container names must be 1-256 characters
		return len(s) > 0 && len(s) <= 256
	})
}

// genValidPrefix generates valid Swift prefixes.
func genValidPrefix() gopter.Gen {
	return gen.OneConstOf("", "prod/", "dev/", "staging/", "test/cluster/")
}

// testLogger is a simple logger implementation for testing.
type testLogger struct {
	messages []string
}

func (l *testLogger) Info(msg string, keysAndValues ...interface{}) {
	l.messages = append(l.messages, msg)
}

func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {
	l.messages = append(l.messages, msg)
}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.messages = append(l.messages, msg)
}
