package openstack

import (
	"context"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// planWithImages plans against a snapshot whose only content is the given
// images, so image resolution outcomes are isolated.
func planWithImages(t *testing.T, images []cloudopenstack.Resource, projectID string, opts Options) (Result, *v2.Config) {
	t.Helper()
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{
		AuthURL:           "https://identity.example/v3",
		Region:            "IAD3",
		ProjectID:         projectID,
		ProjectName:       "proj",
		UserDomainName:    "Default",
		ProjectDomainName: "Default",
		Images:            images,
		Networks:          []cloudopenstack.Resource{{ID: "net-1", Name: "cluster-net"}},
		Subnets:           []cloudopenstack.Subnet{{Resource: cloudopenstack.Resource{ID: "subnet-1", Name: "cluster-subnet"}, NetworkID: "net-1"}},
		ExternalNetworks:  []cloudopenstack.Resource{{ID: "public-1", Name: "public"}},
	}
	result, prospective, err := Plan(context.Background(), cfg, snapshot, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result, prospective
}

// TestPlanDefaultImageExactNameTieBreak pins Option A: when the case-insensitive
// default name matches several variants, exactly one byte-exact
// "Ubuntu 24.04" wins with a warning instead of blocking the plan.
func TestPlanDefaultImageExactNameTieBreak(t *testing.T) {
	images := []cloudopenstack.Resource{
		{ID: "img-upper", Name: "Ubuntu 24.04", Owner: "project-1"},
		{ID: "img-lower", Name: "ubuntu 24.04", Owner: "project-1"},
		{ID: "img-server", Name: "Ubuntu 24.04 server amd64", Owner: "project-1"},
	}
	result, prospective := planWithImages(t, images, "project-1", Options{})
	if sel := imageSelection(result); sel != nil {
		t.Fatalf("plan kept the image ambiguous despite an exact-name default match: %#v", sel)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-upper" {
		t.Fatalf("image_id = %q, want img-upper (exact-name match)", got)
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "exact-name match") || !strings.Contains(warnings, "img-upper") {
		t.Fatalf("warnings missing exact-name explanation:\n%s", warnings)
	}
}

// TestPlanDefaultImageOwnerTieBreak pins Option B: same-name candidates where
// exactly one is owned by the authenticated project resolve to the owned image.
func TestPlanDefaultImageOwnerTieBreak(t *testing.T) {
	images := []cloudopenstack.Resource{
		{ID: "img-community", Name: "Ubuntu 24.04", Owner: "someone-else"},
		{ID: "img-community2", Name: "Ubuntu 24.04", Owner: "another-project"},
		{ID: "img-ours", Name: "Ubuntu 24.04", Owner: "project-1"},
	}
	result, prospective := planWithImages(t, images, "project-1", Options{})
	if sel := imageSelection(result); sel != nil {
		t.Fatalf("plan kept the image ambiguous despite a single project-owned match: %#v", sel)
	}
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-ours" {
		t.Fatalf("image_id = %q, want img-ours (project-owned)", got)
	}
	if warnings := strings.Join(result.Warnings, "\n"); !strings.Contains(warnings, "project-owned") {
		t.Fatalf("warnings missing owner explanation:\n%s", warnings)
	}
}

// TestPlanDefaultImageExactNameBeatsOwner pins the chain order: a byte-exact
// name wins even when a differently-cased candidate is the project-owned one.
func TestPlanDefaultImageExactNameBeatsOwner(t *testing.T) {
	images := []cloudopenstack.Resource{
		{ID: "img-exact-foreign", Name: "Ubuntu 24.04", Owner: "someone-else"},
		{ID: "img-owned-lower", Name: "ubuntu 24.04", Owner: "project-1"},
	}
	_, prospective := planWithImages(t, images, "project-1", Options{})
	if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-exact-foreign" {
		t.Fatalf("image_id = %q, want img-exact-foreign (exact name outranks ownership)", got)
	}
}

// TestPlanDefaultImageTieBreakStaysBlocked pins the conservative floor: the
// plan still blocks when no tie-break step yields a single winner.
func TestPlanDefaultImageTieBreakStaysBlocked(t *testing.T) {
	t.Run("two exact-name duplicates", func(t *testing.T) {
		images := []cloudopenstack.Resource{
			{ID: "img-a", Name: "Ubuntu 24.04", Owner: "project-1"},
			{ID: "img-b", Name: "Ubuntu 24.04", Owner: "project-1"},
		}
		result, prospective := planWithImages(t, images, "project-1", Options{})
		if sel := imageSelection(result); sel == nil || sel.Field != "image" {
			t.Fatalf("duplicate exact names must keep the image ambiguous: %#v", sel)
		}
		if prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID != "" {
			t.Fatal("blocked image selection mutated image_id")
		}
	})
	t.Run("case variants resolve via exact name despite ownership", func(t *testing.T) {
		images := []cloudopenstack.Resource{
			{ID: "img-exact", Name: "Ubuntu 24.04", Owner: "someone-else"},
			{ID: "img-lower", Name: "ubuntu 24.04", Owner: "project-1"},
			{ID: "img-server", Name: "Ubuntu 24.04 server", Owner: "project-1"},
		}
		result, prospective := planWithImages(t, images, "project-1", Options{})
		if sel := imageSelection(result); sel != nil {
			t.Fatalf("case variants must resolve via exact-name match: %#v", sel)
		}
		if got := prospective.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID; got != "img-exact" {
			t.Fatalf("image_id = %q, want img-exact (exact name beats the project-owned variant)", got)
		}
	})
	t.Run("ownership metadata absent blocks", func(t *testing.T) {
		images := []cloudopenstack.Resource{
			{ID: "img-a", Name: "Ubuntu 24.04"},
			{ID: "img-b", Name: "Ubuntu 24.04"},
		}
		result, _ := planWithImages(t, images, "project-1", Options{})
		if sel := imageSelection(result); sel == nil || sel.Field != "image" {
			t.Fatalf("missing owner metadata must keep the image ambiguous: %#v", sel)
		}
	})
}

// TestPlanExplicitImageNameStaysStrict pins that the tie-break chain is
// default-only: an explicit ambiguous --image-name still errors.
func TestPlanExplicitImageNameStaysStrict(t *testing.T) {
	images := []cloudopenstack.Resource{
		{ID: "img-upper", Name: "Ubuntu 24.04", Owner: "project-1"},
		{ID: "img-lower", Name: "ubuntu 24.04", Owner: "project-1"},
	}
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	snapshot := &cloudopenstack.DiscoverySnapshot{Images: images}
	_, _, err = Plan(context.Background(), cfg, snapshot, Options{ImageName: "ubuntu 24.04"})
	if err == nil || !strings.Contains(err.Error(), "matches 2 discovered resources") {
		t.Fatalf("explicit ambiguous selector error = %v, want strict ambiguity error", err)
	}
}

// imageSelection returns the image selection from a plan result, or nil when
// the image resolved.
func imageSelection(result Result) *Selection {
	for i := range result.Selections {
		if result.Selections[i].Field == "image" {
			return &result.Selections[i]
		}
	}
	return nil
}
