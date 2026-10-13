package v2

import (
	"strings"
	"testing"
)

func TestValidateRejectsManagedFQDNFromDifferentOpenStackRegion(t *testing.T) {
	cfg, err := NewV2DefaultForRegion("mixed-region", "openstack", "SJC3")
	if err != nil {
		t.Fatalf("NewV2DefaultForRegion() error = %v", err)
	}
	cfg.OpenCenter.Cluster.ClusterFQDN = "mixed-region.dfw3.k8s.opencenter.cloud"

	err = NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "was derived for region DFW3") {
		t.Fatalf("Validate() error = %v, want mixed-region FQDN error", err)
	}
}

func TestValidateAllowsCustomFQDNWithoutRegionLabel(t *testing.T) {
	cfg, err := NewV2DefaultForRegion("custom-domain", "openstack", "SJC3")
	if err != nil {
		t.Fatalf("NewV2DefaultForRegion() error = %v", err)
	}
	cfg.OpenCenter.Cluster.ClusterFQDN = "kubernetes.example.com"

	if err := NewValidator().Validate(cfg); err != nil {
		t.Fatalf("Validate() rejected custom FQDN: %v", err)
	}
}

func TestValidateRejectsOpenStackCloudRegionMismatch(t *testing.T) {
	cfg, err := NewV2DefaultForRegion("mixed-cloud", "openstack", "SJC3")
	if err != nil {
		t.Fatalf("NewV2DefaultForRegion() error = %v", err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "DFW3"

	err = NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "does not match opencenter.meta.region") {
		t.Fatalf("Validate() error = %v, want cloud region mismatch", err)
	}
}
