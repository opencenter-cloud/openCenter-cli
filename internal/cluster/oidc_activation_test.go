package cluster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveKubernetesOIDCSettingsPrefersKubeAPIOverrides(t *testing.T) {
	cfg := mustNewClusterTestConfig("oidc-settings", "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled = true
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.IssuerURL = "https://identity.example/realms/default"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.ClientID = "default-client"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL = "https://identity.example/realms/kubernetes/"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID = "kubernetes-client"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernamePrefix = "user:"

	got, err := resolveKubernetesOIDCSettings(&cfg)
	if err != nil {
		t.Fatalf("resolveKubernetesOIDCSettings() error = %v", err)
	}
	if got.IssuerURL != "https://identity.example/realms/kubernetes" || got.ClientID != "kubernetes-client" {
		t.Fatalf("unexpected issuer/client settings: %#v", got)
	}
	if got.UsernamePrefix != "user:" || got.GroupsPrefix != "oidc:" {
		t.Fatalf("unexpected prefix settings: %#v", got)
	}
}

func TestInternalIdentityEnablesKubernetesOIDC(t *testing.T) {
	cfg := mustNewClusterTestConfig("internal-oidc", "openstack")
	cfg.OpenCenter.Cluster.ClusterFQDN = "internal-oidc.sjc3.k8s.opencenter.cloud"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled = false
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.OpenCenter.Identity.OIDC.Source = "internal"
	cfg.OpenCenter.Identity.OIDC.Provider = "keycloak"

	if !shouldActivateKubernetesOIDC(&cfg) {
		t.Fatal("internal identity OIDC must activate Kubernetes API OIDC")
	}
	got, err := resolveKubernetesOIDCSettings(&cfg)
	if err != nil {
		t.Fatalf("resolveKubernetesOIDCSettings() error = %v", err)
	}
	if got.IssuerURL != "https://auth.internal-oidc.dfw3.k8s.opencenter.cloud/realms/opencenter" {
		t.Fatalf("issuer URL = %q", got.IssuerURL)
	}
	if got.ClientID != "opencenter" {
		t.Fatalf("client ID = %q", got.ClientID)
	}
}

func TestWriteKubernetesOIDCActivationFiles(t *testing.T) {
	dir := t.TempDir()
	playbookPath := filepath.Join(dir, "activate.yml")
	varsPath := filepath.Join(dir, "vars.json")
	settings := kubernetesOIDCSettings{IssuerURL: "https://auth.example/realms/opencenter", ClientID: "opencenter", CAFile: "/etc/ssl/certs/ca-certificates.crt", UsernameClaim: "preferred_username", UsernamePrefix: "oidc:", GroupsClaim: "groups", GroupsPrefix: "oidc:", APIPort: 443}

	if err := writeKubernetesOIDCActivationFiles(playbookPath, varsPath, settings); err != nil {
		t.Fatalf("writeKubernetesOIDCActivationFiles() error = %v", err)
	}
	playbook, err := os.ReadFile(playbookPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"serial: 1", "/var/backups/opencenter-static-pods", "kube-apiserver.yaml.*~", "until: oidc_flag_count.stdout | int == 7"} {
		if !strings.Contains(string(playbook), required) {
			t.Fatalf("playbook missing %q", required)
		}
	}
	encoded, err := os.ReadFile(varsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got kubernetesOIDCSettings
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode vars: %v", err)
	}
	if got != settings {
		t.Fatalf("vars = %#v, want %#v", got, settings)
	}
	for _, path := range []string{playbookPath, varsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", path, info.Mode().Perm())
		}
	}
}

func TestOIDCActivationPlaybookKeepsManifestArgumentIndentation(t *testing.T) {
	var plays []struct {
		Tasks []map[string]any `yaml:"tasks"`
	}
	if err := yaml.Unmarshal([]byte(oidcActivationPlaybook), &plays); err != nil {
		t.Fatalf("parse OIDC activation playbook: %v", err)
	}
	for _, task := range plays[0].Tasks {
		module, ok := task["ansible.builtin.blockinfile"].(map[string]any)
		if !ok {
			continue
		}
		block, _ := module["block"].(string)
		if !strings.HasPrefix(block, "    - --oidc-issuer-url=") {
			t.Fatalf("OIDC manifest block has unsafe indentation: %q", block)
		}
		return
	}
	t.Fatal("OIDC activation playbook has no blockinfile task")
}
