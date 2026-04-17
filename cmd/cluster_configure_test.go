package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	servicescfg "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestClusterConfigureRequiresGuidedFlag(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterConfigureCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"guided-openstack"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster configure to require --guided")
	}
	if want := "--guided is required"; !bytes.Contains([]byte(err.Error()), []byte(want)) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func TestClusterConfigureGuidedCreatesOpenStackCluster(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	answers := guidedCreateAnswers()

	encoded, err := json.Marshal(answers)
	if err != nil {
		t.Fatalf("marshal guided answers: %v", err)
	}
	t.Setenv("OPENCENTER_GUIDED_ANSWERS", string(encoded))

	cmd := newClusterConfigureCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"guided-openstack", "--guided", "--org", "opencenter", "--type", "openstack"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster configure failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	cfg, err := loadCanonicalConfig("guided-openstack")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}

	if cfg.OpenCenter.GitOps.GitURL != "https://github.com/example/platform-clusters.git" {
		t.Fatalf("git_url = %q", cfg.OpenCenter.GitOps.GitURL)
	}
	if cfg.OpenCenter.GitOps.GitTokenProvider != "github" {
		t.Fatalf("git_token_provider = %q", cfg.OpenCenter.GitOps.GitTokenProvider)
	}
	if cfg.OpenCenter.GitOps.GitToken == "" {
		t.Fatal("expected git_token path to be set")
	}
	if cfg.OpenCenter.GitOps.GitSSHKey != "" || cfg.OpenCenter.GitOps.GitSSHPub != "" {
		t.Fatalf("expected SSH git fields to be cleared, got %q and %q", cfg.OpenCenter.GitOps.GitSSHKey, cfg.OpenCenter.GitOps.GitSSHPub)
	}

	tokenBytes, err := os.ReadFile(cfg.OpenCenter.GitOps.GitToken)
	if err != nil {
		t.Fatalf("read managed git token file: %v", err)
	}
	if string(tokenBytes) != "ghp_test_token\n" {
		t.Fatalf("unexpected managed git token contents: %q", string(tokenBytes))
	}

	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstackCfg == nil {
		t.Fatal("expected openstack cloud config to be present")
	}
	if openstackCfg.Networking == nil {
		t.Fatal("expected openstack networking compatibility block to be present")
	}
	if openstackCfg.NetworkID != "net-123" || openstackCfg.Networking.NetworkID != "net-123" {
		t.Fatalf("expected synced network id, got top-level=%q nested=%q", openstackCfg.NetworkID, openstackCfg.Networking.NetworkID)
	}
	if openstackCfg.SubnetID != "subnet-123" || openstackCfg.Networking.SubnetID != "subnet-123" {
		t.Fatalf("expected synced subnet id, got top-level=%q nested=%q", openstackCfg.SubnetID, openstackCfg.Networking.SubnetID)
	}
	if openstackCfg.FloatingNetworkID != "ext-net-123" || openstackCfg.Networking.FloatingNetworkID != "ext-net-123" {
		t.Fatalf("expected synced floating network id, got top-level=%q nested=%q", openstackCfg.FloatingNetworkID, openstackCfg.Networking.FloatingNetworkID)
	}
	if openstackCfg.RouterExternalNetworkID != "ext-net-123" || openstackCfg.Networking.RouterExternalNetworkID != "ext-net-123" {
		t.Fatalf("expected synced router external network id, got top-level=%q nested=%q", openstackCfg.RouterExternalNetworkID, openstackCfg.Networking.RouterExternalNetworkID)
	}

	certManagerAny, ok := cfg.OpenCenter.Services["cert-manager"]
	if !ok {
		t.Fatal("expected cert-manager service to be present")
	}
	certManager, ok := certManagerAny.(*servicescfg.CertManagerConfig)
	if !ok {
		t.Fatalf("expected cert-manager config type, got %T", certManagerAny)
	}
	if certManager.DNSProvider != "route53" {
		t.Fatalf("dns_provider = %q", certManager.DNSProvider)
	}
	if certManager.Region != "us-east-1" {
		t.Fatalf("cert-manager region = %q", certManager.Region)
	}
	if cfg.Secrets.Global.AWS.Application.AccessKey != "AKIAGUIDEDTEST" {
		t.Fatalf("application access key = %q", cfg.Secrets.Global.AWS.Application.AccessKey)
	}
	if cfg.Secrets.Global.AWS.Application.SecretAccessKey != "guided-secret-key" {
		t.Fatalf("application secret access key = %q", cfg.Secrets.Global.AWS.Application.SecretAccessKey)
	}
}

func TestClusterConfigureGuidedCancelLeavesConfigUnwritten(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	answers := guidedCreateAnswers()
	answers["review.confirm"] = "false"
	setGuidedAnswers(t, answers)

	cmd := newClusterConfigureCmd()
	cmd.SetArgs([]string{"cancel-openstack", "--guided", "--org", "opencenter", "--type", "openstack"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected configure cancellation to return an error")
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".cancel-openstack-config.yaml")
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no config file on cancel, stat err = %v", statErr)
	}
}

func TestClusterConfigureGuidedUpdatesExistingCluster(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	setGuidedAnswers(t, guidedCreateAnswers())
	createCmd := newClusterConfigureCmd()
	createCmd.SetArgs([]string{"update-openstack", "--guided", "--org", "opencenter", "--type", "openstack"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("initial cluster configure failed: %v", err)
	}

	updateAnswers := map[string]string{
		"openstack.image_id":                   "ubuntu-2404-rev2",
		"openstack.flavor_bastion":             "gp.1.4",
		"openstack.flavor_master":              "gp.1.8",
		"openstack.flavor_worker":              "gp.1.8",
		"openstack.master_count":               "3",
		"openstack.worker_count":               "5",
		"openstack.network_id":                 "net-123",
		"openstack.subnet_id":                  "subnet-123",
		"openstack.floating_network_id":        "ext-net-123",
		"openstack.router_external_network_id": "ext-net-123",
		"openstack.availability_zone":          "az1",
		"review.confirm":                       "true",
	}
	setGuidedAnswers(t, updateAnswers)
	resetCommandStateForTests()

	updateCmd := newClusterConfigureCmd()
	updateCmd.SetArgs([]string{"update-openstack", "--guided"})
	if err := updateCmd.Execute(); err != nil {
		t.Fatalf("update cluster configure failed: %v", err)
	}

	resetCommandStateForTests()
	cfg, err := loadCanonicalConfig("update-openstack")
	if err != nil {
		t.Fatalf("load canonical config after update: %v", err)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID != "ubuntu-2404-rev2" {
		t.Fatalf("image_id = %q", cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID)
	}
	if cfg.OpenCenter.Infrastructure.Compute.FlavorWorker != "gp.1.8" {
		t.Fatalf("worker flavor = %q", cfg.OpenCenter.Infrastructure.Compute.FlavorWorker)
	}
	if cfg.OpenCenter.Infrastructure.Compute.WorkerCount != 5 {
		t.Fatalf("worker count = %d", cfg.OpenCenter.Infrastructure.Compute.WorkerCount)
	}
}

func guidedCreateAnswers() map[string]string {
	return map[string]string{
		"openstack.auth_url":                      "https://identity.api.example.com/v3",
		"openstack.region":                        "sjc3",
		"openstack.project_id":                    "project-123",
		"openstack.project_name":                  "project-name",
		"openstack.domain":                        "rackspace",
		"openstack.application_credential_id":     "app-cred-id",
		"openstack.application_credential_secret": "app-cred-secret",
		"openstack.insecure":                      "true",
		"openstack.image_id":                      "ubuntu-2404",
		"openstack.flavor_bastion":                "gp.1.2",
		"openstack.flavor_master":                 "gp.1.4",
		"openstack.flavor_worker":                 "gp.1.4",
		"openstack.master_count":                  "3",
		"openstack.worker_count":                  "2",
		"openstack.network_id":                    "net-123",
		"openstack.subnet_id":                     "subnet-123",
		"openstack.floating_network_id":           "ext-net-123",
		"openstack.router_external_network_id":    "ext-net-123",
		"openstack.availability_zone":             "az1",
		"git.url":                                 "https://github.com/example/platform-clusters.git",
		"git.token_provider":                      "github",
		"git.token":                               "ghp_test_token",
		"dns.provider":                            "route53",
		"dns.route53.region":                      "us-east-1",
		"dns.route53.access_key":                  "AKIAGUIDEDTEST",
		"dns.route53.secret_key":                  "guided-secret-key",
		"review.confirm":                          "true",
	}
}

func setGuidedAnswers(t *testing.T, answers map[string]string) {
	t.Helper()
	encoded, err := json.Marshal(answers)
	if err != nil {
		t.Fatalf("marshal guided answers: %v", err)
	}
	t.Setenv("OPENCENTER_GUIDED_ANSWERS", string(encoded))
}
