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
	if got := strings.Join(result.Warnings, "\n"); !strings.Contains(got, "opencenter.infrastructure.cloud.openstack.image_id is ambiguous: 2 candidates discovered; specify --image-name with one of the listed names or IDs") {
		t.Fatalf("blocked plan did not state why the image is unresolved:\n%s", got)
	}
}

// TestPlanSelectorsResolveByName pins the operator-facing contract: `--image-name`
// and friends take the name printed by the candidate list, and the stored value
// is still the resolved ID because that is what OpenTofu consumes.
func TestPlanSelectorsResolveByName(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{
			{ID: "img-ubuntu", Name: "Ubuntu 24.04"},
			{ID: "img-alma", Name: "AlmaLinux 9"},
		},
		WindowsImages: []cloudopenstack.Resource{{ID: "win-2022", Name: "Windows Server 2022"}},
		Networks:      []cloudopenstack.Resource{{ID: "net-a", Name: "cluster-net"}, {ID: "net-b", Name: "other-net"}},
		Subnets: []cloudopenstack.Subnet{
			{Resource: cloudopenstack.Resource{ID: "subnet-a", Name: "cluster-subnet"}, NetworkID: "net-a"},
			{Resource: cloudopenstack.Resource{ID: "subnet-b", Name: "other-subnet"}, NetworkID: "net-a"},
		},
		ExternalNetworks:           []cloudopenstack.Resource{{ID: "ext-a", Name: "PUBLICNET"}, {ID: "ext-b", Name: "PUBLICNET2"}},
		AvailabilityZonesAvailable: true,
		AvailabilityZones:          []cloudopenstack.Resource{{ID: "az-1", Name: "az1"}, {ID: "az-2", Name: "az2"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{
		ImageName:         "Ubuntu 24.04",
		WindowsImageName:  "Windows Server 2022",
		NetworkID:         "cluster-net",
		SubnetID:          "cluster-subnet",
		ExternalNetworkID: "PUBLICNET",
		AvailabilityZone:  "az2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned; warnings=%v selections=%#v", result.Status, result.Warnings, result.Selections)
	}
	if len(result.Selections) != 0 {
		t.Fatalf("name selectors left unresolved selections: %#v", result.Selections)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.ImageID != "img-ubuntu" || osCfg.ImageName != "Ubuntu 24.04" || osCfg.ImageIDWindows != "win-2022" {
		t.Fatalf("images not resolved from names: %#v", osCfg)
	}
	if osCfg.NetworkID != "net-a" || osCfg.NetworkName != "cluster-net" || osCfg.SubnetID != "subnet-a" || osCfg.Networking.SubnetID != "subnet-a" {
		t.Fatalf("internal network not resolved from names: %#v", osCfg)
	}
	if osCfg.RouterExternalNetworkID != "ext-a" || osCfg.ExternalNetworkName != "PUBLICNET" || osCfg.Networking.FloatingIPPool != "PUBLICNET" {
		t.Fatalf("external network not resolved from name: %#v", osCfg)
	}
	// The stored value is the zone ID, not the typed name.
	if osCfg.AvailabilityZone != "az-2" {
		t.Fatalf("availability zone not resolved from name: %q", osCfg.AvailabilityZone)
	}
}

// TestPlanNameSelectorNeedsReplaceForPopulatedValue pins that name resolution
// changes only *how* a selector is matched, never the write guard: a populated
// value still requires --replace, whether the selector is a name or an ID.
func TestPlanNameSelectorNeedsReplaceForPopulatedValue(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{ImageID: "old-image", ImageName: "Ubuntu 22.04"}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{{ID: "old-image", Name: "Ubuntu 22.04"}, {ID: "new-image", Name: "Ubuntu 24.04"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImageName: "Ubuntu 24.04"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || !strings.Contains(strings.Join(result.Warnings, " "), "--replace") {
		t.Fatalf("populated image rewritten without --replace: result=%#v", result)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "old-image" {
		t.Fatalf("blocked plan mutated image_id: %q", got)
	}

	result, prospective, err = Plan(context.Background(), cfg, snapshot, Options{ImageName: "Ubuntu 24.04", Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	// The snapshot has no networks, so the plan stays blocked on those fields;
	// what matters is that the name selector wrote both image values.
	if osCfg.ImageID != "new-image" || osCfg.ImageName != "Ubuntu 24.04" {
		t.Fatalf("name selector with --replace did not apply: status=%q cfg=%#v", result.Status, osCfg)
	}

	// An unresolvable selector is a hard error, never a silent no-op.
	if _, _, err := Plan(context.Background(), cfg, snapshot, Options{ImageName: "Ubuntu 30.10"}); err == nil || !strings.Contains(err.Error(), "was not found in OpenStack discovery by ID or name") {
		t.Fatalf("error = %v, want not-found-by-ID-or-name", err)
	}
}

// TestPlanDefaultsToDefaultLinuxImageNameWhenDiscovered pins the preferred
// behavior: with no --image-name, plan resolves DefaultLinuxImageName against
// live inventory and says so, instead of blocking on 20+ candidates.
func TestPlanDefaultsToDefaultLinuxImageNameWhenDiscovered(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{
			{ID: "img-ubuntu", Name: "Ubuntu 24.04"},
			{ID: "img-alma", Name: "AlmaLinux 9"},
			{ID: "img-debian", Name: "Debian 12"},
		},
		Networks:         []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "ext-1", Name: "PUBLICNET"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned; warnings=%v selections=%#v", result.Status, result.Warnings, result.Selections)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.ImageID != "img-ubuntu" || osCfg.ImageName != "Ubuntu 24.04" {
		t.Fatalf("default image name not resolved: %#v", osCfg)
	}
	if warnings := strings.Join(result.Warnings, "\n"); !strings.Contains(warnings, `defaulting to "Ubuntu 24.04" (img-ubuntu)`) {
		t.Fatalf("defaulting was not reported to the operator:\n%s", warnings)
	}
}

// TestPlanDefaultLinuxImageNameFallsBackToCandidateList verifies the default is
// a preference, never a fabricated value: a project without that image keeps the
// ordinary ambiguous behavior and prints every candidate.
func TestPlanDefaultLinuxImageNameFallsBackToCandidateList(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{{ID: "img-alma", Name: "AlmaLinux 9"}, {ID: "img-debian", Name: "Debian 12"}},
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("status = %q, want blocked", result.Status)
	}
	if len(result.Selections) != 1 || result.Selections[0].Field != "image" || len(result.Selections[0].Candidates) != 2 {
		t.Fatalf("candidate list not reported: %#v", result.Selections)
	}
	if warnings := strings.Join(result.Warnings, "\n"); strings.Contains(warnings, "defaulting") {
		t.Fatalf("absent default was reported as applied:\n%s", warnings)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "" {
		t.Fatalf("blocked plan wrote image_id %q", got)
	}
}

// TestPlanSkipsWindowsImageWithoutWindowsWorkers pins that a Linux-only cluster
// is no longer blocked on a Windows image it will never boot, while Windows
// workers bring the requirement back.
func TestPlanSkipsWindowsImageWithoutWindowsWorkers(t *testing.T) {
	newCfg := func() *v2.Config {
		cfg, err := v2.NewV2Default("prod", "openstack")
		if err != nil {
			t.Fatal(err)
		}
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
		return cfg
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images:           []cloudopenstack.Resource{{ID: "img-ubuntu", Name: "Ubuntu 24.04"}},
		WindowsImages:    []cloudopenstack.Resource{{ID: "win-a", Name: "Win A"}, {ID: "win-b", Name: "Win B"}},
		Networks:         []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "ext-1", Name: "PUBLICNET"}},
	}

	result, prospective, err := Plan(context.Background(), newCfg(), snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("Linux-only cluster blocked on Windows image: status=%q warnings=%v selections=%#v", result.Status, result.Warnings, result.Selections)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageIDWindows; got != "" {
		t.Fatalf("Windows image selected for a cluster with no Windows workers: %q", got)
	}

	// Windows workers present: the ambiguity is back, as it must be.
	withWindows := newCfg()
	withWindows.OpenCenter.Infrastructure.Compute.WorkerCountWindows = 2
	result, _, err = Plan(context.Background(), withWindows, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("Windows cluster not blocked on ambiguous Windows image: status=%q", result.Status)
	}
}

// TestPlanDefaultLinuxImageNameDoesNotOverrideStoredValue keeps the default from
// silently rewriting a cluster that already has an image selected.
func TestPlanDefaultLinuxImageNameDoesNotOverrideStoredValue(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{ImageID: "img-alma", ImageName: "AlmaLinux 9"}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{{ID: "img-ubuntu", Name: "Ubuntu 24.04"}, {ID: "img-alma", Name: "AlmaLinux 9"}},
	}

	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.ImageID != "img-alma" {
		t.Fatalf("stored image was replaced by the default: %q", osCfg.ImageID)
	}
}

// TestPlanAmbiguousNameSelectorIsRejectedByNameAndID guards against
// first-match-wins: Glance names are not unique, so a name matching several
// resources must name every candidate instead of picking one.
func TestPlanAmbiguousNameSelectorIsRejectedByNameAndID(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Images: []cloudopenstack.Resource{
			{ID: "img-rebuilt", Name: "Ubuntu 24.04"},
			{ID: "img-golden", Name: "ubuntu 24.04"},
			{ID: "img-other", Name: "AlmaLinux 9"},
		},
	}

	_, _, err = Plan(context.Background(), cfg, snapshot, Options{ImageName: "Ubuntu 24.04"})
	if err == nil {
		t.Fatal("duplicate-name selector accepted")
	}
	wanted := `image selector "Ubuntu 24.04" matches 2 discovered resources: Ubuntu 24.04 (img-rebuilt), ubuntu 24.04 (img-golden); specify exactly one`
	if err.Error() != wanted {
		t.Fatalf("error =\n%q\nwant\n%q", err.Error(), wanted)
	}

	// Disambiguating by ID still works.
	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImageName: "img-golden"})
	if err != nil {
		t.Fatal(err)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-golden" {
		t.Fatalf("ID selector resolved to %q", got)
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

// init derives auth_url from its own region guess, so it bakes a real,
// fully-formed keystone endpoint for a region the cluster may never use. When
// the binding moved the region to IAD3 but left that endpoint pointing at DFW3,
// kit-kat provisioned in IAD3 while its CCM cloud.conf and cinder CSI
// cloud.conf authenticated against DFW3 - keystone rejected the IAD3-issued
// application credential with 404 "Could not find Application Credential", the
// CCM never initialized a node, the cloud-provider taint was never removed, and
// every add-on stayed Pending while deploy reported success. An endpoint that is
// exactly what init would generate for its default region is a guess, not an
// operator decision, so the binding must repoint it.
func TestPlanReplacesInitDerivedAuthURL(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	osCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.AuthURL != v2.KeystoneAuthURL("DFW3") {
		t.Fatalf("precondition: init auth_url = %q, want the DFW3 keystone", osCfg.AuthURL)
	}

	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{
		Region:  "IAD3",
		AuthURL: v2.KeystoneAuthURL("IAD3"),
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL
	if got != v2.KeystoneAuthURL("IAD3") {
		t.Fatalf("auth_url = %q, want the discovered region's endpoint", got)
	}
	// The whole point: region and endpoint must agree in the planned config.
	if !strings.Contains(got, strings.ToLower(prospective.OpenCenter.Infrastructure.Cloud.OpenStack.Region)) {
		t.Errorf("planned auth_url %q disagrees with planned region %q", got, prospective.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
	}
	reported := false
	for _, change := range result.Changes {
		if change.Path == "opencenter.infrastructure.cloud.openstack.auth_url" {
			reported = true
		}
	}
	if !reported {
		t.Errorf("plan did not report the auth_url rewrite: %#v", result.Changes)
	}
}

// An endpoint the operator pointed somewhere else - a private keystone, a
// different vendor - is a decision, and the binding must not silently rewrite it
// just because it disagrees with the discovered region.
func TestPlanPreservesOperatorChosenAuthURL(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.corp.private.net:5000/v3/"

	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{
		Region:  "IAD3",
		AuthURL: v2.KeystoneAuthURL("IAD3"),
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL; !strings.Contains(got, "corp.private.net") {
		t.Fatalf("operator auth_url was overwritten with %q", got)
	}
	if result.Status == StatusApplied {
		t.Errorf("plan reported %s while an operator endpoint was left in place", result.Status)
	}
}

// TestPlanPropagatesDiscoveredRegionToClusterFields covers the init→plan→apply
// contract: `cluster init` bakes meta.region from the CLI default long before
// any cloud is chosen, so binding the cluster to an IAD3 cloud must repoint
// every region-derived field, not just the provider block.
func TestPlanPropagatesDiscoveredRegionToClusterFields(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenCenter.Meta.Region != "DFW3" {
		t.Fatalf("precondition: init default meta.region = %q, want DFW3", cfg.OpenCenter.Meta.Region)
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Meta.Region != "IAD3" {
		t.Fatalf("meta.region = %q, want discovered IAD3", prospective.OpenCenter.Meta.Region)
	}
	if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.Region != "IAD3" {
		t.Fatalf("cloud.openstack.region = %q, want discovered IAD3", prospective.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
	}
	if prospective.OpenCenter.Secrets.Barbican.Region != "IAD3" {
		t.Fatalf("barbican.region = %q, want discovered IAD3", prospective.OpenCenter.Secrets.Barbican.Region)
	}
	if cfg.OpenCenter.Meta.Region != "DFW3" {
		t.Fatalf("Plan mutated the original meta.region to %q", cfg.OpenCenter.Meta.Region)
	}
	if cfg.OpenCenter.Secrets.Barbican.Region != "" {
		t.Fatalf("Plan mutated the original barbican.region to %q", cfg.OpenCenter.Secrets.Barbican.Region)
	}
	found := false
	for _, change := range result.Changes {
		if change.Path == "opencenter.meta.region" && change.Old == "DFW3" && change.New == "IAD3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changes do not report the meta.region rewrite: %#v", result.Changes)
	}
	// NTP hosts are region-specific (time.<region>.rackspace.com). init baked
	// the DFW3 pair; binding to IAD3 must repoint them, or every node
	// time-syncs against a host in the wrong region.
	wantNTP := v2.DefaultNTPServers("openstack", "IAD3")
	gotNTP := prospective.OpenCenter.Infrastructure.Networking.NTPServers
	if strings.Join(gotNTP, ",") != strings.Join(wantNTP, ",") {
		t.Fatalf("ntp_servers = %v, want the discovered region's %v", gotNTP, wantNTP)
	}
	if !strings.Contains(strings.Join(gotNTP, ","), "iad3") {
		t.Errorf("ntp_servers %v still points at the init-guessed region", gotNTP)
	}
	// Flavors are region-derived compute defaults. They happen to be identical
	// across the currently registered regions, so assert the field is populated
	// with the discovered region's value rather than a literal.
	wantFlavors := v2.DefaultFlavorsForRegion("openstack", "IAD3")
	compute := prospective.OpenCenter.Infrastructure.Compute
	for role, current := range map[string]string{
		"master": compute.FlavorMaster,
		"worker": compute.FlavorWorker,
	} {
		if current != wantFlavors[role] {
			t.Errorf("compute flavor %q = %q, want %q", role, current, wantFlavors[role])
		}
	}
	// The original config must be untouched by a plan.
	if !strings.Contains(strings.Join(cfg.OpenCenter.Infrastructure.Networking.NTPServers, ","), "dfw3") {
		t.Errorf("Plan mutated the original ntp_servers to %v", cfg.OpenCenter.Infrastructure.Networking.NTPServers)
	}
}

// Operator-managed time sources are a deliberate choice: a cluster on an
// isolated network, or one subject to a compliance clock policy, points NTP at
// its own servers. The binding must block and name --replace rather than
// silently reverting to the public Rackspace hosts, which would leave the nodes
// unable to reach any time source.
func TestPlanBlocksOperatorChosenNTPServers(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Networking.NTPServers = []string{"chrony.corp.internal"}

	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("status = %s, want blocked", result.Status)
	}
	got := prospective.OpenCenter.Infrastructure.Networking.NTPServers
	if len(got) != 1 || got[0] != "chrony.corp.internal" {
		t.Fatalf("operator ntp_servers were replaced with %v", got)
	}
	warned := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "ntp_servers") && strings.Contains(warning, "IAD3") && strings.Contains(warning, "--replace") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected an ntp_servers warning naming the region and --replace, got %v", result.Warnings)
	}
}

// An empty flavor is not an operator choice: init leaves it empty for a region
// the provider registry does not know, and a cluster rendered with no flavor
// has no bootable instance type. The binding must fill it.
func TestPlanFillsEmptyFlavorsFromDiscoveredRegion(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Compute.FlavorMaster = ""
	cfg.OpenCenter.Infrastructure.Compute.FlavorWorker = ""
	cfg.OpenCenter.Infrastructure.Compute.FlavorBastion = ""

	_, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := v2.DefaultFlavorsForRegion("openstack", "IAD3")
	compute := prospective.OpenCenter.Infrastructure.Compute
	for role, current := range map[string]string{
		"bastion": compute.FlavorBastion,
		"master":  compute.FlavorMaster,
		"worker":  compute.FlavorWorker,
	} {
		if current != want[role] {
			t.Errorf("empty %q flavor = %q, want the region default %q", role, current, want[role])
		}
	}
}

// A flavor the operator chose is protected the same way a region is: the
// binding warns and leaves it alone rather than silently reverting a decision.
func TestPlanPreservesOperatorChosenFlavor(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Compute.FlavorWorker = "some.custom.flavor"

	result, prospective, err := Plan(context.Background(), cfg, &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prospective.OpenCenter.Infrastructure.Compute.FlavorWorker; got != "some.custom.flavor" {
		t.Fatalf("operator flavor was overwritten with %q", got)
	}
	warned := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "flavor_worker") && strings.Contains(warning, "--replace") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected a --replace warning for flavor_worker, got %v", result.Warnings)
	}
}

// The provider-defaults registry asserted a gp.5.* family that no Rackspace
// region actually offers; a cluster left on that default died in `tofu apply`
// with Nova's "Unable to find flavor with name gp.5.4.8" after provisioning had
// already begun. Discovery lists the region's real flavors, so the plan must use
// them: a flavor the cloud does not have blocks the plan and names the ones it
// does offer.
func TestPlanBlocksFlavorMissingFromCloud(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	// An operator-chosen flavor the cloud does not offer. No region default
	// competes with it, so it survives into the validation pass - which is the
	// case that must be caught: the config renders, OpenTofu starts, and Nova
	// rejects the boot.
	cfg.OpenCenter.Infrastructure.Compute.FlavorBastion = "m1.large"

	// Live IAD3 inventory: the gp.0.* family only.
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Region: "IAD3",
		Flavors: []cloudopenstack.Resource{
			{ID: "1", Name: "gp.0.2.4"},
			{ID: "2", Name: "gp.0.4.8"},
			{ID: "3", Name: "gp.0.4.16"},
		},
	}

	result, _, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("status = %s, want blocked for a flavor the cloud does not offer", result.Status)
	}
	var message string
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "does not exist in the discovered cloud region") {
			message = warning
		}
	}
	if message == "" {
		t.Fatalf("no missing-flavor warning in %v", result.Warnings)
	}
	// The warning has to be actionable: name the bad value and the real ones.
	for _, want := range []string{"m1.large", "IAD3", "gp.0.4.16"} {
		if !strings.Contains(message, want) {
			t.Errorf("warning %q does not mention %q", message, want)
		}
	}
}

// A flavor the cloud does offer must pass, and an unavailable flavor inventory
// must not block: the compute service can be unreachable or the credential may
// lack the flavor-list policy, and that must not fail a binding that would work.
func TestPlanAcceptsFlavorsPresentInCloud(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Region: "IAD3",
		Flavors: []cloudopenstack.Resource{
			{ID: "1", Name: "gp.0.2.4"},
			{ID: "2", Name: "gp.0.4.8"},
			{ID: "3", Name: "gp.0.4.16"},
		},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "does not exist in the discovered cloud region") {
			t.Errorf("valid flavor rejected: %s", warning)
		}
	}
	if got := prospective.OpenCenter.Infrastructure.Compute.FlavorWorker; got != "gp.0.4.16" {
		t.Errorf("flavor_worker = %q, want the region default gp.0.4.16", got)
	}
}

// TestPlanBlocksPopulatedMetaRegionWithoutReplace guards the identity fields:
// a region the operator set deliberately is never silently rewritten by a
// provider binding, and the plan refuses to half-propagate.
func TestPlanBlocksPopulatedMetaRegionWithoutReplace(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Meta.Region = "SJC3"
	cfg.OpenCenter.Secrets.Barbican.Region = "SJC3"
	snapshot := &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked {
		t.Fatalf("status = %q, want blocked", result.Status)
	}
	if prospective.OpenCenter.Meta.Region != "SJC3" {
		t.Fatalf("meta.region = %q, want preserved SJC3", prospective.OpenCenter.Meta.Region)
	}
	if prospective.OpenCenter.Secrets.Barbican.Region != "SJC3" {
		t.Fatalf("barbican.region = %q, want unchanged SJC3: the follower set is all-or-nothing, so a protected meta.region must not leave barbican half-repointed", prospective.OpenCenter.Secrets.Barbican.Region)
	}
	var named bool
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "opencenter.meta.region") && strings.Contains(warning, "--replace") {
			named = true
		}
	}
	if !named {
		t.Fatalf("warnings do not name the protected region: %#v", result.Warnings)
	}
}

// TestPlanReplacesPopulatedMetaRegionWithReplace verifies --replace repoints
// the whole cluster at the discovered region.
func TestPlanReplacesPopulatedMetaRegionWithReplace(t *testing.T) {
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Meta.Region = "SJC3"
	cfg.OpenCenter.Secrets.Barbican.Region = "SJC3"
	snapshot := &cloudopenstack.DiscoverySnapshot{Region: "IAD3"}

	_, prospective, err := Plan(context.Background(), cfg, snapshot, Options{Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if prospective.OpenCenter.Meta.Region != "IAD3" || prospective.OpenCenter.Secrets.Barbican.Region != "IAD3" {
		t.Fatalf("--replace did not repoint region fields: meta=%q barbican=%q", prospective.OpenCenter.Meta.Region, prospective.OpenCenter.Secrets.Barbican.Region)
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

	result, _, err := Plan(context.Background(), cfg, snapshot, Options{ImageName: "new-image"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || len(result.Warnings) == 0 {
		t.Fatalf("result = %#v, want explicit replacement block", result)
	}

	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{ImageName: "new-image", Replace: true})
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
	result, prospective, err = Plan(context.Background(), cfg, snapshot, Options{ImageName: "img-a", WindowsImageName: "win-a", Replace: true})
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
	// The region fields are the exception to "preserved": the discovered
	// region is authoritative and repoints the provider block, meta.region, the
	// Barbican backend, the region-specific NTP hosts, and everything derived
	// from cluster_fqdn, even without --replace (meta.region and the NTP pair
	// were the init defaults here). The FQDN-derived set is the same class of
	// value: init bakes those hostnames from the guessed region, so leaving them
	// behind would put the Gateway's listeners in two DNS domains. Everything
	// else must stay untouched.
	regionPaths := map[string]bool{
		"opencenter.infrastructure.cloud.openstack.availability_zones": true,
		"opencenter.infrastructure.cloud.openstack.region":             true,
		"opencenter.infrastructure.networking.ntp_servers":             true,
		"opencenter.meta.region":                                       true,
		"opencenter.secrets.barbican.region":                           true,
		"opencenter.cluster.cluster_fqdn":                              true,
		"opencenter.services.keycloak.hostname":                        true,
		"opencenter.services.headlamp.hostname":                        true,
		"opencenter.services.longhorn.hostname":                        true,
		"opencenter.managed_services.alert-proxy.http_route_fqdn":      true,
	}
	for _, change := range result.Changes {
		if !regionPaths[change.Path] {
			t.Fatalf("preserved values unexpectedly produced change: %#v", result.Changes)
		}
	}
	warnings := strings.Join(result.Warnings, " ")
	for _, unwanted := range []string{"image_id", "image_id_windows", "project_domain_name", "availability_zone", "--replace", "--image-name", "--availability-zone"} {
		if strings.Contains(warnings, unwanted) {
			t.Fatalf("warning %q unexpectedly present: %s", unwanted, warnings)
		}
	}
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	if osCfg.Region != "RegionOne" || prospective.OpenCenter.Meta.Region != "RegionOne" || osCfg.ProjectDomainName != "default" || osCfg.ImageID != "discovered-linux" || osCfg.ImageIDWindows != "discovered-windows" || osCfg.AvailabilityZone != "existing-zone" {
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

// TestApplyPersistencePersistsPropagatedRegion proves the end of the
// init→plan→apply chain: the discovered region must land in the written
// cluster file, in both meta.region and the provider block.
func TestApplyPersistencePersistsPropagatedRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	// Empty provider block: the init-baked floating_ip_pool default would
	// otherwise block the plan on a populated value unrelated to the region
	// contract under test.
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	originalBytes, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:          "https://identity.example/v3",
		Region:           "IAD3",
		ProjectID:        "project-1",
		Images:           []cloudopenstack.Resource{{ID: "img-linux", Name: "Ubuntu 24.04"}},
		Networks:         []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned: %#v", result.Status, result.Warnings)
	}
	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	result, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return nil }}).Apply(context.Background(), path, cfg, prospective, originalBytes, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusApplied {
		t.Fatalf("status = %q, want applied", result.Status)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := v2.DecodePublicConfig(written)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OpenCenter.Meta.Region != "IAD3" {
		t.Fatalf("persisted meta.region = %q, want IAD3", reloaded.OpenCenter.Meta.Region)
	}
	if reloaded.OpenCenter.Infrastructure.Cloud.OpenStack.Region != "IAD3" {
		t.Fatalf("persisted cloud.openstack.region = %q, want IAD3", reloaded.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
	}
}

// TestPlanPropagatesLinuxImageToBastionAndPools pins the end-to-end image
// contract of 'cluster init --os-cloud': the single resolved Linux image must
// land in every consumer that stores its own copy — the provider block, the
// bastion, and every enabled Linux worker pool — and must be persisted by
// apply, so no instance ever boots from the empty/placeholder image baked at
// init time.
func TestPlanPropagatesLinuxImageToBastionAndPools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	cfg, err := v2.NewV2Default("fritos", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = true
	cfg.OpenCenter.Infrastructure.Bastion.Image = ""
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []v2.WorkerPoolConfig{
		{Name: "gpu-workers", Count: 2, Flavor: "gpu-flavor", Image: ""},
		{Name: "disabled-pool", Count: 0, Flavor: "worker", Image: ""},
	}
	originalBytes, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, originalBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:          "https://identity.example/v3",
		Region:           "IAD3",
		ProjectID:        "project-1",
		Images:           []cloudopenstack.Resource{{ID: "img-ubuntu", Name: "Ubuntu 24.04"}},
		Networks:         []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusPlanned {
		t.Fatalf("status = %q, want planned: warnings=%v", result.Status, result.Warnings)
	}
	if prospective.OpenCenter.Infrastructure.Bastion.Image != "img-ubuntu" {
		t.Fatalf("bastion image = %q, want img-ubuntu", prospective.OpenCenter.Infrastructure.Bastion.Image)
	}
	pools := prospective.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker
	if pools[0].Image != "img-ubuntu" {
		t.Fatalf("enabled pool image = %q, want img-ubuntu", pools[0].Image)
	}
	if pools[1].Image != "" {
		t.Fatalf("disabled pool image = %q, want untouched", pools[1].Image)
	}
	// The caller's config must not be mutated by the propagation.
	if cfg.OpenCenter.Infrastructure.Bastion.Image != "" {
		t.Fatalf("plan mutated the caller's bastion image: %q", cfg.OpenCenter.Infrastructure.Bastion.Image)
	}
	if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[0].Image != "" {
		t.Fatal("plan mutated the caller's pool image")
	}

	fileSystem := fs.NewDefaultFileSystem(utilerrors.NewDefaultErrorHandlerWithoutMasking())
	result, err = (ApplyPersistence{FileSystem: fileSystem, Validate: func(context.Context, *v2.Config) error { return nil }}).Apply(context.Background(), path, cfg, prospective, originalBytes, result)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := v2.DecodePublicConfig(written)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OpenCenter.Infrastructure.Bastion.Image != "img-ubuntu" {
		t.Fatalf("persisted bastion.image = %q, want img-ubuntu", reloaded.OpenCenter.Infrastructure.Bastion.Image)
	}
	if reloaded.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[0].Image != "img-ubuntu" {
		t.Fatalf("persisted pool image = %q, want img-ubuntu", reloaded.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[0].Image)
	}
}

// TestPlanPreservesDeliberateBastionAndPoolImagesWithoutReplace proves the
// propagation only rewrites init-baked (empty/placeholder) follower images:
// an image the operator set deliberately survives a rebind unless --replace.
func TestPlanPreservesDeliberateBastionAndPoolImagesWithoutReplace(t *testing.T) {
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = true
	cfg.OpenCenter.Infrastructure.Bastion.Image = "my-custom-image"
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []v2.WorkerPoolConfig{
		{Name: "gpu-workers", Count: 1, Flavor: "gpu-flavor", Image: "my-pool-image"},
	}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		Region:           "IAD3",
		Images:           []cloudopenstack.Resource{{ID: "img-ubuntu", Name: "Ubuntu 24.04"}},
		Networks:         []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:          []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks: []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prospective.OpenCenter.Infrastructure.Bastion.Image; got != "my-custom-image" {
		t.Fatalf("deliberate bastion image overwritten without --replace: %q", got)
	}
	if got := prospective.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[0].Image; got != "my-pool-image" {
		t.Fatalf("deliberate pool image overwritten without --replace: %q", got)
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "opencenter.infrastructure.bastion.image is populated with \"my-custom-image\"") {
		t.Fatalf("warnings do not name the protected bastion image:\n%s", warnings)
	}

	// With --replace the followers are repointed to the resolved image.
	result, prospective, err = Plan(context.Background(), cfg, snapshot, Options{Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status == StatusBlocked {
		t.Fatalf("replace plan blocked: %v", result.Warnings)
	}
	if got := prospective.OpenCenter.Infrastructure.Bastion.Image; got != "img-ubuntu" {
		t.Fatalf("bastion image with --replace = %q, want img-ubuntu", got)
	}
	if got := prospective.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[0].Image; got != "img-ubuntu" {
		t.Fatalf("pool image with --replace = %q, want img-ubuntu", got)
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
	_, _, err = Plan(context.Background(), cfg, snapshot, Options{NetworkID: "internal", SubnetID: "other-subnet", ImageName: "image", ExternalNetworkID: "external"})
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
	}, Options{ImageName: "image"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusBlocked || result.InternalNetworkMode != "" || len(result.Selections) != 1 || result.Selections[0].Field != "network" {
		t.Fatalf("no-flag behavior changed: result=%#v config=%#v", result, prospective.OpenCenter.Infrastructure.Cloud.OpenStack)
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
		windows  bool
		field    string
		warning  string
	}{
		{
			name:     "windows image",
			snapshot: &cloudopenstack.DiscoverySnapshot{WindowsImages: []cloudopenstack.Resource{{ID: "win-a", Name: "Win A"}, {ID: "win-b", Name: "Win B"}}},
			windows:  true,
			field:    "windows image",
			warning:  "opencenter.infrastructure.cloud.openstack.image_id_windows is ambiguous: 2 candidates discovered; specify --windows-image-name",
		},
		{
			name: "network",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:   []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				Networks: []cloudopenstack.Resource{{ID: "network-a", Name: "a"}, {ID: "network-b", Name: "b"}},
			},
			options: Options{ImageName: "image"},
			field:   "network",
			warning: "opencenter.infrastructure.cloud.openstack.network_id is ambiguous: 2 candidates discovered; specify --network-id",
		},
		{
			name: "external network",
			snapshot: &cloudopenstack.DiscoverySnapshot{
				Images:           []cloudopenstack.Resource{{ID: "image", Name: "Ubuntu"}},
				ExternalNetworks: []cloudopenstack.Resource{{ID: "ext-a", Name: "PUBLICNET-A"}, {ID: "ext-b", Name: "PUBLICNET-B"}},
			},
			options: Options{ImageName: "image"},
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
			options: Options{ImageName: "image", NetworkID: "network"},
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
			options: Options{ImageName: "image"},
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
			if tc.windows {
				cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows = 1
			}
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
