package v2

import "testing"

func TestOpenStackProviderAllowsOpenTofuManagedNetwork(t *testing.T) {
	cfg := &InfrastructureConfig{
		Provider: "openstack",
		Cloud: CloudConfig{
			OpenStack: &OpenStackCloudConfig{
				AuthURL:   "https://identity.example.com/v3",
				Region:    "dfw3",
				ProjectID: "project-123",
				ImageID:   "image-456",
			},
		},
	}

	if err := (&OpenStackProvider{}).ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig() should allow OpenTofu-managed network: %v", err)
	}
}

func validMagnumInfrastructureConfig() *InfrastructureConfig {
	return &InfrastructureConfig{
		Provider: "magnum",
		Cloud: CloudConfig{
			Magnum: &MagnumCloudConfig{
				AuthURL:                     "https://identity.example.com/v3",
				Region:                      "dfw3",
				ProjectID:                   "project-123",
				ApplicationCredentialID:     "app-cred-id",
				ApplicationCredentialSecret: "app-cred-secret",
				ClusterTemplate:             "kubernetes-template",
			},
		},
	}
}

func TestMagnumProviderValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*InfrastructureConfig)
		wantErr bool
	}{
		{name: "success"},
		{
			name: "missing cluster template",
			mutate: func(cfg *InfrastructureConfig) {
				cfg.Cloud.Magnum.ClusterTemplate = ""
			},
			wantErr: true,
		},
		{
			name: "incomplete application credentials",
			mutate: func(cfg *InfrastructureConfig) {
				cfg.Cloud.Magnum.ApplicationCredentialSecret = ""
			},
			wantErr: true,
		},
		{
			name: "wrong provider config",
			mutate: func(cfg *InfrastructureConfig) {
				cfg.Cloud.OpenStack = &OpenStackCloudConfig{AuthURL: "https://identity.example.com/v3"}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validMagnumInfrastructureConfig()
			if tt.mutate != nil {
				tt.mutate(cfg)
			}

			err := (&MagnumProvider{}).ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMagnumProviderRejectsProviderMismatch(t *testing.T) {
	cfg := validMagnumInfrastructureConfig()
	cfg.Provider = "openstack"

	if err := (&MagnumProvider{}).ValidateConfig(cfg); err == nil {
		t.Fatal("ValidateConfig() should reject a provider mismatch")
	}
}

func TestGetProviderMagnum(t *testing.T) {
	provider, err := GetProvider("magnum")
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if provider.GetProviderName() != "magnum" {
		t.Fatalf("GetProvider().GetProviderName() = %q, want magnum", provider.GetProviderName())
	}
	if _, ok := provider.(*MagnumProvider); !ok {
		t.Fatalf("GetProvider() type = %T, want *MagnumProvider", provider)
	}
}
