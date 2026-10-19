package openstack

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

func TestPlanFillsOnlyUnambiguousProviderFields(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"

	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:           "https://identity.example/v3",
		Region:            "RegionOne",
		ProjectID:         "project-1",
		ProjectName:       "project",
		UserDomainName:    "Default",
		ProjectDomainName: "Default",
		Images:            []cloudopenstack.Resource{{ID: "img-linux", Name: "Ubuntu 24.04"}},
		WindowsImages:     []cloudopenstack.Resource{{ID: "img-windows", Name: "Windows 2022"}},
		Networks:          []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
		AvailabilityZones: []cloudopenstack.Resource{{ID: "az1", Name: "az1"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want %q", result.Status, StatusPlanned)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.AuthURL != snapshot.AuthURL || osCfg.ProjectID != snapshot.ProjectID || osCfg.ImageID != "img-linux" || osCfg.NetworkID != "net-1" || osCfg.SubnetID != "subnet-1" {
		t.Fatalf("provider patch did not fill expected values: %#v", osCfg)
	}
	if osCfg.ApplicationCredentialSecret != "" {
		t.Fatal("provider plan imported a secret without --import-auth")
	}
	if len(result.RemoteActions) != 0 || !result.SensitiveValuesRedacted {
		t.Fatalf("unsafe result metadata: %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("structured result contains unexpected sensitive data: %s", encoded)
	}
}

func TestPlanBlocksAmbiguousSelectionWithoutMutation(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	before := *cfg.OpenCenter.Infrastructure.Cloud.OpenStack

	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{{ID: "img-b", Name: "Ubuntu B"}, {ID: "img-a", Name: "Ubuntu A"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || len(result.Selections) == 0 {
		t.Fatalf("result = %#v, want blocked selection", result)
	}
	after := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if after.AuthURL != before.AuthURL || after.ImageID != before.ImageID || after.NetworkID != before.NetworkID || after.SubnetID != before.SubnetID || len(after.AvailabilityZones) != len(before.AvailabilityZones) {
		t.Fatal("blocked plan mutated the provider config")
	}
	if got := result.Selections[0].Candidates; len(got) != 2 || got[0].ID != "img-a" || got[1].ID != "img-b" {
		t.Fatalf("candidates are not stable: %#v", got)
	}
	if got := strings.Join(result.Warnings, "\n"); !strings.Contains(got, "opencenter.infrastructure.cloud.openstack.image_id is ambiguous: 2 candidates discovered; specify --image-id with one of the listed IDs") {
		t.Fatalf("blocked plan did not state why the image is unresolved:\n%s", got)
	}
}

// TestPlanAmbiguousSelectionsNameResolvingSelector guards the exit-2 contract:
// every ambiguous field must emit a warning naming the config path, the
// candidate count, and the selector that resolves it, so a blocked plan is
// self-explanatory instead of a bare candidate dump.
func TestPlanAmbiguousSelectionsNameResolvingSelector(t *testing.T) {
	cases := []struct {
		name     string
		snapshot *cloudopenstack.DiscoverySnapshot
		options  Options
		field    string
		warning  string
	}{
		{
			name:     "windows image",
			snapshot: &cloudopenstack.DiscoverySnapshot{WindowsImages: []cloudopenstack.Resource{{ID: "win-a", Name: "Win A"}, {ID: "win-b", Name: "Win B"}}},
			field:    "windows image",
			warning:  "opencenter.infrastructure.cloud.openstack.image_id_windows is ambiguous: 2 candidates discovered; specify --windows-image-id",
		},
		{
			name: "network",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:   []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				Networks: []cloudopenstack.Resource{{ID: "network-a", Name: "a"}, {ID: "network-b", Name: "b"}},
			},
			options: Options{ImageID: "image"},
			field:   "network",
			warning: "opencenter.infrastructure.cloud.openstack.network_id is ambiguous: 2 candidates discovered; specify --network-id",
		},
		{
			name: "external network",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				ExternalNetworks: []cloudopenstack.Resource{{ID: "ext-a", Name: "PUBLICNET-A"}, {ID: "ext-b", Name: "PUBLICNET-B"}},
			},
			options: Options{ImageID: "image"},
			field:   "external network",
			warning: "opencenter.infrastructure.cloud.openstack.router_external_network_id is ambiguous: 2 candidates discovered; specify --external-network-id",
		},
		{
			name: "subnet",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:   []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				Networks: []cloudopenstack.Resource{{ID: "network", Name: "internal"}},
				Subnets: []cloudopenstack.Subnet{
					{Resource: cloudopenstack.Resource{ID: "subnet-a", Name: "a"}, NetworkID: "network"},
					{Resource: cloudopenstack.Resource{ID: "subnet-b", Name: "b"}, NetworkID: "network"},
				},
				ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "PUBLICNET"}},
			},
			options: Options{ImageID: "image", NetworkID: "network"},
			field:   "subnet",
			warning: "opencenter.infrastructure.cloud.openstack.subnet_id is ambiguous: 2 candidates discovered; specify --subnet-id",
		},
		{
			name: "availability zone",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:                     []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				AvailabilityZonesAvailable: true,
				AvailabilityZones:          []cloudopenstack.Resource{{ID: "az-a", Name: "az-a"}, {ID: "az-b", Name: "az-b"}},
			},
			options: Options{ImageID: "image"},
			field:   "availability zone",
			warning: "opencenter.infrastructure.cloud.openstack.availability_zone is ambiguous: 2 candidates discovered; specify --availability-zone",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := v2.NewV2Default("prod", "openstack")
			if err != nil {
				t.Fatal(err)
			}
			cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
			result, _, err := Plan(context.Background(), cfg, tc.snapshot, tc.options)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != StatusBlocked {
				t.Fatalf("status = %q, want blocked; warnings=%v", result.Status, result.Warnings)
			}
			var selected bool
			for _, selection := range result.Selections {
				selected = selected || selection.Field == tc.field
			}
			if !selected {
				t.Fatalf("selections missing %q: %#v", tc.field, result.Selections)
			}
			if warnings := strings.Join(result.Warnings, "\n"); !strings.Contains(warnings, tc.warning) {
				t.Errorf("warnings missing %q:\n%s", tc.warning, warnings)
			}
		})
	}
}

func TestPlanPreservesUppercaseDFW3Region(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "DFW3"
	snapshot := &cloudopenstack.DiscoverySnapshot{Region: "DFW3"}

	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.Region; got != "DFW3" {
		t.Fatalf("planned OpenStack region = %q, want DFW3", got)
	}
}

func TestPlanRequiresReplaceForExplicitConflictingSelection(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{ImageID: "old-image"}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:            []cloudopenstack.Resource{{ID: "new-image", Name: "Ubuntu"}},
		Networks:          []cloudopenstack.Resource{{ID: "network", Name: "internal"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet", Name: "internal"}, NetworkID: "network"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "external", Name: "public"}},
		AvailabilityZones: []cloudopenstack.Resource{{ID: "zone", Name: "zone"}},
	}

	result, _, err := Plan(context.Background(), cfg, snapshot, Options{ImageID: "new-image"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || len(result.Warnings) == 0 {
		t.Fatalf("result = %#v, want explicit replacement block", result)
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImageID: "new-image", Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID != "new-image" {
		t.Fatalf("replace did not apply conflict: result=%#v cfg=%q", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID)
	}
}

// TestPlanBlocksStaleStoredImageWithoutSelector verifies that a stored
// image_id matching no discovered resource is surfaced explicitly instead of
// being silently preserved (the previous behavior made `plan` report a
// misleading no-op for clusters seeded with baked-in default IDs).
func TestPlanBlocksStaleStoredImageWithoutSelector(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		ImageID:        "stale-image",
		ImageIDWindows: "stale-windows-image",
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:            []cloudopenstack.Resource{{ID: "img-a", Name: "Ubuntu A"}, {ID: "img-b", Name: "Ubuntu B"}},
		WindowsImages:     []cloudopenstack.Resource{{ID: "win-a", Name: "Win A"}, {ID: "win-b", Name: "Win B"}},
		Networks:          []cloudopenstack.Resource{{ID: "network", Name: "internal"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet", Name: "internal"}, NetworkID: "network"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "external", Name: "public"}},
		AvailabilityZones: []cloudopenstack.Resource{{ID: "zone", Name: "zone"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("status = %q, want blocked on stale images", result.Status)
	}
	warnings := strings.Join(result.Warnings, "\n")
	for _, want := range []string{"image_id \"stale-image\" does not match any discovered Linux image", "image_id_windows \"stale-windows-image\" does not match any discovered Windows image"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "stale-image" {
		t.Fatalf("blocked plan mutated ImageID: %q", got)
	}

	// Explicit selector plus --replace resolves the stale value.
	result, prospective, err = Plan(context.Background(), cfg, snapshot, Options{ImageID: "img-a", WindowsImageID: "win-a", Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if result.Status != StatusPlanned || osCfg.ImageID != "img-a" || osCfg.ImageIDWindows != "win-a" {
		t.Fatalf("replace did not clear stale images: status=%q image=%q windows=%q", result.Status, osCfg.ImageID, osCfg.ImageIDWindows)
	}

	// A stored ID that DOES match discovery stays silent and unchanged.
	cfg2, _ := v2.NewV2Default("prod", "openstack")
	cfg2.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		ImageID:        "img-a",
		ImageIDWindows: "win-a",
	}
	result, prospective, err = Plan(context.Background(), cfg2, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("valid stored images should not block: status=%q warnings=%v", result.Status, result.Warnings)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-a" {
		t.Fatalf("valid stored image was changed: %q", got)
	}
}

func TestPlanPreservesPopulatedClusterValuesDuringCreateInternalNetworkImport(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		AuthURL:                     "https://identity.example/v3",
		Region:                      "dfw3",
		ProjectID:                   "project-1",
		ProjectDomainName:           "default",
		ImageID:                     "discovered-linux",
		ImageIDWindows:              "discovered-windows",
		AvailabilityZone:            "existing-zone",
		ApplicationCredentialID:     "app-id",
		ApplicationCredentialSecret: "app-secret",
		RouterExternalNetworkID:     "external-current",
		FloatingNetworkID:           "external-current",
		FloatingIPPool:              "PUBLICNET",
		ExternalNetworkName:         "PUBLICNET",
		Networking: &v2.OpenStackNetworkingConfig{
			FloatingNetworkID:       "external-current",
			RouterExternalNetworkID: "external-current",
			FloatingIPPool:          "PUBLICNET",
		},
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:                    "https://identity.example/v3",
		Region:                     "RegionOne",
		ProjectID:                  "project-1",
		ProjectDomainName:          "Default",
		Images:                     []cloudopenstack.Resource{{ID: "discovered-linux", Name: "Ubuntu"}},
		WindowsImages:              []cloudopenstack.Resource{{ID: "discovered-windows", Name: "Windows"}},
		ExternalNetworks:           []cloudopenstack.Resource{{ID: "external-current", Name: "PUBLICNET"}},
		AvailabilityZonesAvailable: true,
		AvailabilityZones:          []cloudopenstack.Resource{{ID: "discovered-zone", Name: "zone"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{
		CreateInternalNetwork: true,
		ImportAuth:            &AuthImport{ApplicationCredentialID: "app-id", ApplicationCredentialSecret: "app-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned && result.Status != StatusNoOp {
		t.Fatalf("result = %#v, want planned or no-op tofu-managed result", result)
	}
	if len(result.Selections) != 0 {
		t.Fatalf("preserved values unexpectedly produced selections: %#v", result)
	}
	for _, change := range result.Changes {
		if change.Path != "opencenter.infrastructure.cloud.openstack.availability_zones" {
			t.Fatalf("preserved values unexpectedly produced change: %#v", result.Changes)
		}
	}
	warnings := strings.Join(result.Warnings, " ")
	for _, unwanted := range []string{"image_id", "image_id_windows", "project_domain_name", "region", "availability_zone", "--replace", "--image-id", "--availability-zone"} {
		if strings.Contains(warnings, unwanted) {
			t.Fatalf("warning %q unexpectedly present: %s", unwanted, warnings)
		}
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.Region != "dfw3" || osCfg.ProjectDomainName != "default" || osCfg.ImageID != "discovered-linux" || osCfg.ImageIDWindows != "discovered-windows" || osCfg.AvailabilityZone != "existing-zone" {
		t.Fatalf("populated cluster values were not preserved: %#v", osCfg)
	}
}

func TestPlanImportAuthRedactsResult(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL: "https://identity.example/v3",
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImportAuth: &AuthImport{ApplicationCredentialID: "app-id", ApplicationCredentialSecret: "super-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID != "app-id" || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret != "super-secret" {
		t.Fatal("--import-auth did not write profile credentials")
	}
	for _, change := range result.Changes {
		if change.Path == "opencenter.infrastructure.cloud.openstack.application_credential_secret" && change.New != "<redacted>" {
			t.Fatalf("secret was not redacted: %#v", change)
		}
	}
}

func TestApplyPersistenceWritesBackupOnlyForRealPatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	prospective := cloneProviderConfig(cfg)
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	result, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return nil }}).Apply(context.Background(), path, cfg, prospective, originalBytes, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusApplied {
		t.Fatalf("status = %q, want applied", result.Status)
	}
	backup, err := os.ReadFile(path + ".backup")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(originalBytes) {
		t.Fatalf("backup = %q, want original bytes", backup)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) == string(originalBytes) {
		t.Fatal("expected atomic write to replace original")
	}

	noOpPath := filepath.Join(dir, "noop.yaml")
	if err := os.WriteFile(noOpPath, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	noOp := Result{Status: StatusNoOp}
	if _, err := (ApplyPersistence{FileSystem: fileSystem}).Apply(context.Background(), noOpPath, cfg, cfg, originalBytes, noOp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(noOpPath + ".backup"); !os.IsNotExist(err) {
		t.Fatalf("no-op created backup: %v", err)
	}

}

func TestPlanExcludesExternalNetworksFromInternalSelection(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Networks:         []cloudopenstack.Resource{{ID: "public", Name: "PUBLICNET"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "public", Name: "PUBLICNET"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || prospective.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID != "" {
		t.Fatalf("external-only inventory selected an internal network: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
}

func TestPlanRequiresDiscoveryMatchForExistingExternalNetwork(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		ImageID:                 "image",
		RouterExternalNetworkID: "external-current",
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external-discovered", Name: "PUBLICNET"}},
	}
	result, _, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || !strings.Contains(strings.Join(result.Warnings, " "), "router_external_network_id") {
		t.Fatalf("result = %#v, want external-network discovery conflict", result)
	}
}

func TestPlanRejectsCrossNetworkSubnetSelector(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks:         []cloudopenstack.Resource{{ID: "internal", Name: "internal"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "other-subnet", Name: "other"}, NetworkID: "other-network"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "public"}},
	}
	_, _, err = Plan(context.Background(), cfg, snapshot, Options{NetworkID: "internal", SubnetID: "other-subnet", ImageID: "image", ExternalNetworkID: "external"})
	if err == nil || !strings.Contains(err.Error(), "subnet selector") {
		t.Fatalf("cross-network subnet was accepted: %v", err)
	}
}

func TestPlanWritesExternalNetworkNameToPoolMirrors(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks:         []cloudopenstack.Resource{{ID: "internal", Name: "internal"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet", Name: "subnet"}, NetworkID: "internal"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "PUBLICNET"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned", result.Status)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.FloatingIPPool != "PUBLICNET" || osCfg.Networking == nil || osCfg.Networking.FloatingIPPool != "PUBLICNET" || osCfg.FloatingNetworkID != "external" || osCfg.Networking.FloatingNetworkID != "external" {
		t.Fatalf("external compatibility mirrors incorrect: %#v", osCfg)
	}
}

func TestPlanDoesNotPersistTLSWithoutImportTLS(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{CA: "/workstation/ca.pem", Insecure: true}
	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.CA != "" || osCfg.Insecure {
		t.Fatalf("discovery TLS settings were persisted without --import-tls: %#v", osCfg)
	}
	_, prospective, err = Plan(context.Background(), cfg, snapshot, Options{ImportTLS: true, Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	osCfg = prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.CA != snapshot.CA || !osCfg.Insecure {
		t.Fatalf("explicit TLS import was not persisted: %#v", osCfg)
	}
}

func TestPlanPreservesAvailabilityZoneWhenInventoryUnavailable(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{AvailabilityZone: "existing-zone"}
	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone != "existing-zone" || result.Status == StatusBlocked && strings.Contains(strings.Join(result.Warnings, " "), "availability zone") {
		t.Fatalf("unavailable compute inventory changed or blocked existing AZ: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
}

func TestApplyPersistenceRejectsStaleSourceBeforeBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	_, err = (ApplyPersistence{FileSystem: fileSystem}).Apply(context.Background(), path, cfg, cloneProviderConfig(cfg), originalBytes, result)
	if err == nil || !strings.Contains(err.Error(), "changed since planning") {
		t.Fatalf("stale source was not rejected: %v", err)
	}
	if _, statErr := os.Stat(path + ".backup"); !os.IsNotExist(statErr) {
		t.Fatalf("stale source created backup: %v", statErr)
	}
}

func TestApplyPersistenceValidatesBeforeBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	originalBytes := []byte("original\n")
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	result := Result{Status: StatusPlanned, Changes: []Change{{Path: "provider", Old: "", New: "value"}}}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	_, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return errors.New("invalid prospective config") }}).Apply(context.Background(), path, cfg, cloneProviderConfig(cfg), originalBytes, result)
	if err == nil || !strings.Contains(err.Error(), "prospective configuration validation failed") {
		t.Fatalf("validation failure was not returned: %v", err)
	}
	if _, statErr := os.Stat(path + ".backup"); !os.IsNotExist(statErr) {
		t.Fatalf("validation failure created backup: %v", statErr)
	}
}

func TestPlanCreateInternalNetworkBypassesAmbiguityAndClearsMirrors(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		ImageID:                 "image",
		NetworkID:               "old-network",
		NetworkName:             "old-network-name",
		SubnetID:                "old-subnet",
		FloatingNetworkID:       "external",
		RouterExternalNetworkID: "external",
		FloatingIPPool:          "PUBLICNET",
		Networking: &v2.OpenStackNetworkingConfig{
			NetworkID:               "old-network",
			SubnetID:                "old-subnet",
			FloatingNetworkID:       "external",
			RouterExternalNetworkID: "external",
			FloatingIPPool:          "PUBLICNET",
		},
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks:         []cloudopenstack.Resource{{ID: "network-a", Name: "a"}, {ID: "network-b", Name: "b"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-a", Name: "a"}, NetworkID: "network-a"}, {Resource: cloudopenstack.Resource{ID: "subnet-b", Name: "b"}, NetworkID: "network-b"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "PUBLICNET"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{CreateInternalNetwork: true, Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned || result.InternalNetworkMode != "tofu-managed" {
		t.Fatalf("result = %#v, want planned tofu-managed result", result)
	}
	for _, selection := range result.Selections {
		if selection.Field == "network" || selection.Field == "subnet" {
			t.Fatalf("create mode emitted internal selection: %#v", result.Selections)
		}
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.NetworkID != "" || osCfg.NetworkName != "" || osCfg.SubnetID != "" || osCfg.Networking == nil || osCfg.Networking.NetworkID != "" || osCfg.Networking.SubnetID != "" {
		t.Fatalf("internal selections were not cleared atomically: %#v", osCfg)
	}
	paths := map[string]bool{}
	for _, change := range result.Changes {
		paths[change.Path] = true
	}
	for _, path := range []string{
		"opencenter.infrastructure.cloud.openstack.network_id",
		"opencenter.infrastructure.cloud.openstack.network_name",
		"opencenter.infrastructure.cloud.openstack.subnet_id",
		"opencenter.infrastructure.cloud.openstack.networking.network_id",
		"opencenter.infrastructure.cloud.openstack.networking.subnet_id",
	} {
		if !paths[path] {
			t.Fatalf("missing clear change for %s: %#v", path, result.Changes)
		}
	}
}

func TestPlanCreateInternalNetworkRequiresReplaceToClearPopulatedMirrors(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		NetworkID:  "old-network",
		SubnetID:   "old-subnet",
		Networking: &v2.OpenStackNetworkingConfig{NetworkID: "old-network", SubnetID: "old-subnet"},
	}
	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{}, Options{CreateInternalNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || !strings.Contains(strings.Join(result.Warnings, " "), "--replace") {
		t.Fatalf("result = %#v, want replace protection", result)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.NetworkID != "old-network" || osCfg.SubnetID != "old-subnet" || osCfg.Networking.NetworkID != "old-network" || osCfg.Networking.SubnetID != "old-subnet" {
		t.Fatalf("blocked create mode partially cleared selections: %#v", osCfg)
	}
}

func TestPlanCreateInternalNetworkRejectsSelectorsAndVLAN(t *testing.T) {
	cases := []struct {
		name    string
		options Options
		vlanID  string
		want    string
	}{
		{name: "selectors", options: Options{CreateInternalNetwork: true, NetworkID: "network"}, want: "--network-id"},
		{name: "subnet selector", options: Options{CreateInternalNetwork: true, SubnetID: "subnet"}, want: "--subnet-id"},
		{name: "vlan", options: Options{CreateInternalNetwork: true}, vlanID: "100", want: "networking.vlan.id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := v2.NewV2Default("prod", "openstack")
			if err != nil {
				t.Fatal(err)
			}
			cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{Networking: &v2.OpenStackNetworkingConfig{VLAN: v2.VLANConfigLegacy{ID: tc.vlanID}}}
			_, _, err = Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{}, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPlanWithoutCreateInternalNetworkPreservesSelectionBehavior(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{
		Images:   []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		Networks: []cloudopenstack.Resource{{ID: "network-a", Name: "a"}, {ID: "network-b", Name: "b"}},
	}, Options{ImageID: "image"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || result.InternalNetworkMode != "" || len(result.Selections) != 1 || result.Selections[0].Field != "network" {
		t.Fatalf("no-flag behavior changed: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
	}
}

func TestPlanCreateInternalNetworkNoOpReportsMode(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		ImageID:                 "image",
		FloatingIPPool:          "PUBLICNET",
		FloatingNetworkID:       "external",
		RouterExternalNetworkID: "external",
		Networking: &v2.OpenStackNetworkingConfig{
			FloatingIPPool:          "PUBLICNET",
			FloatingNetworkID:       "external",
			RouterExternalNetworkID: "external",
		},
	}
	result, _, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "external", Name: "PUBLICNET"}},
	}, Options{CreateInternalNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusNoOp || result.InternalNetworkMode != "tofu-managed" {
		t.Fatalf("no-op create mode result = %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"internal_network_mode":"tofu-managed"`) {
		t.Fatalf("structured result omitted internal network mode: %s", encoded)
	}
}
