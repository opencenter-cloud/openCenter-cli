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
