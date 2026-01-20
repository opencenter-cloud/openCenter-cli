package config

import (
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

func TestValidateServiceSecretsSimple(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedErrs  []string
		shouldHaveErr bool
	}{
		{
			name: "cert-manager enabled without secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"cert-manager": &services.CertManagerConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{},
			},
			shouldHaveErr: true,
			expectedErrs: []string{
				"AWS credentials required for cert-manager",
			},
		},
		{
			name: "cert-manager enabled with service-specific secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"cert-manager": &services.CertManagerConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					CertManager: CertManagerSecrets{
						AWSAccessKey:       "test-access-key",
						AWSSecretAccessKey: "test-secret-key",
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "cert-manager enabled with global application secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"cert-manager": &services.CertManagerConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Global: GlobalSecrets{
						AWS: AWSGlobalSecrets{
							Application: AWSSecrets{
								AccessKey:       "test-access-key",
								SecretAccessKey: "test-secret-key",
							},
						},
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "loki enabled without secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"loki": &services.LokiConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{},
			},
			shouldHaveErr: true,
			expectedErrs: []string{
				"Loki requires either Swift password",
			},
		},
		{
			name: "loki enabled with Swift password",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"loki": &services.LokiConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Loki: LokiSecrets{
						SwiftPassword: "test-swift-password",
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "loki enabled with S3 credentials",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"loki": &services.LokiConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Loki: LokiSecrets{
						S3AccessKeyID:     "test-access-key",
						S3SecretAccessKey: "test-secret-key",
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "tempo enabled without secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{},
			},
			shouldHaveErr: true,
			expectedErrs: []string{
				"S3 credentials required for Tempo",
			},
		},
		{
			name: "tempo enabled with service-specific secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Tempo: TempoSecrets{
						AccessKey: "test-access-key",
						SecretKey: "test-secret-key",
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "tempo enabled with global application secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Global: GlobalSecrets{
						AWS: AWSGlobalSecrets{
							Application: AWSSecrets{
								AccessKey:       "test-access-key",
								SecretAccessKey: "test-secret-key",
							},
						},
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "tempo enabled with global infrastructure secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Global: GlobalSecrets{
						AWS: AWSGlobalSecrets{
							Infrastructure: AWSSecrets{
								AccessKey:       "test-access-key",
								SecretAccessKey: "test-secret-key",
							},
						},
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "keycloak enabled without secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{},
			},
			shouldHaveErr: true,
			expectedErrs: []string{
				"secrets.keycloak.admin_password is required",
			},
		},
		{
			name: "keycloak enabled with secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					Keycloak: KeycloakSecrets{
						AdminPassword: "test-admin-password",
					},
				},
			},
			shouldHaveErr: false,
		},
		{
			name: "multiple services enabled with mixed secrets",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"cert-manager": &services.CertManagerConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
				Secrets: Secrets{
					CertManager: CertManagerSecrets{
						AWSAccessKey:       "test-access-key",
						AWSSecretAccessKey: "test-secret-key",
					},
					// Tempo missing secrets
					Keycloak: KeycloakSecrets{
						AdminPassword: "test-admin-password",
					},
				},
			},
			shouldHaveErr: true,
			expectedErrs: []string{
				"S3 credentials required for Tempo",
			},
		},
		{
			name: "services disabled - no secrets required",
			config: Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: ServiceMap{
						"cert-manager": &services.CertManagerConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
						"tempo": &services.TempoConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
					},
				},
				Secrets: Secrets{},
			},
			shouldHaveErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateServiceSecretsSimple(tt.config)

			if tt.shouldHaveErr && len(errs) == 0 {
				t.Errorf("expected validation errors but got none")
			}

			if !tt.shouldHaveErr && len(errs) > 0 {
				t.Errorf("expected no validation errors but got: %v", errs)
			}

			// Check that expected error messages are present
			for _, expectedErr := range tt.expectedErrs {
				found := false
				for _, err := range errs {
					if containsSubstring(err, expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing '%s' but got errors: %v", expectedErr, errs)
				}
			}
		})
	}
}

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
