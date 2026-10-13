package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const oidcActivationPlaybook = `---
- name: Activate Kubernetes API OIDC authentication
  hosts: kube_control_plane
  become: true
  serial: 1
  tasks:
    - name: Wait for the configured OIDC issuer
      ansible.builtin.uri:
        url: "{{ oidc_issuer_url }}/.well-known/openid-configuration"
        method: GET
        return_content: true
        validate_certs: true
        ca_path: "{{ oidc_ca_file }}"
        status_code: 200
      register: oidc_discovery
      retries: 180
      delay: 10
      until:
        - oidc_discovery.status == 200
        - oidc_discovery.json.issuer == oidc_issuer_url

    - name: Create OpenCenter API server backup directory
      ansible.builtin.file:
        path: /var/backups/opencenter-static-pods
        state: directory
        owner: root
        group: root
        mode: "0700"

    - name: Find unsafe API server backups in the static-pod directory
      ansible.builtin.find:
        paths: /etc/kubernetes/manifests
        file_type: file
        patterns:
          - "kube-apiserver.yaml.*~"
          - "kube-apiserver.yaml.bak*"
      register: oidc_static_pod_backups

    - name: Move API server backups outside the static-pod directory
      ansible.builtin.command:
        argv:
          - mv
          - "{{ item.path }}"
          - /var/backups/opencenter-static-pods/
      loop: "{{ oidc_static_pod_backups.files }}"
      loop_control:
        label: "{{ item.path }}"

    - name: Create OpenCenter staging directory
      ansible.builtin.file:
        path: /var/lib/opencenter
        state: directory
        owner: root
        group: root
        mode: "0700"

    - name: Stage the current API server manifest outside the watched directory
      ansible.builtin.copy:
        src: /etc/kubernetes/manifests/kube-apiserver.yaml
        dest: /var/lib/opencenter/kube-apiserver.yaml.oidc
        remote_src: true
        owner: root
        group: root
        mode: "0600"

    - name: Remove prior OIDC arguments from the staged manifest
      ansible.builtin.lineinfile:
        path: /var/lib/opencenter/kube-apiserver.yaml.oidc
        regexp: '^\s*-\s+--oidc-(issuer-url|client-id|ca-file|username-claim|username-prefix|groups-claim|groups-prefix)=.*$'
        state: absent

    - name: Add the configured OIDC arguments to the staged manifest
      ansible.builtin.blockinfile:
        path: /var/lib/opencenter/kube-apiserver.yaml.oidc
        insertafter: '^\s+- kube-apiserver$'
        marker: '    # {mark} OPENCENTER MANAGED OIDC'
        block: |2
              - --oidc-issuer-url={{ oidc_issuer_url }}
              - --oidc-client-id={{ oidc_client_id }}
              - --oidc-ca-file={{ oidc_ca_file }}
              - --oidc-username-claim={{ oidc_username_claim }}
              - '--oidc-username-prefix={{ oidc_username_prefix }}'
              - --oidc-groups-claim={{ oidc_groups_claim }}
              - '--oidc-groups-prefix={{ oidc_groups_prefix }}'

    - name: Validate the staged API server manifest
      ansible.builtin.command:
        argv:
          - kubectl
          - create
          - --dry-run=client
          - --validate=false
          - -f
          - /var/lib/opencenter/kube-apiserver.yaml.oidc
          - -o
          - json
      changed_when: false

    - name: Install the OIDC-enabled API server manifest
      ansible.builtin.copy:
        src: /var/lib/opencenter/kube-apiserver.yaml.oidc
        dest: /etc/kubernetes/manifests/kube-apiserver.yaml
        remote_src: true
        owner: root
        group: root
        mode: "0600"

    - name: Wait for the rolled API server to become ready
      ansible.builtin.uri:
        url: "https://127.0.0.1:{{ kubernetes_api_port }}/readyz"
        method: GET
        validate_certs: false
        status_code: 200
      register: oidc_api_ready
      retries: 36
      delay: 5
      until: oidc_api_ready.status == 200

    - name: Verify all OIDC arguments are active in the API server container
      ansible.builtin.shell: |
        set -o pipefail
        container_id="$(crictl ps --name kube-apiserver -q | head -n 1)"
        test -n "$container_id"
        crictl inspect "$container_id" | grep -c -- '--oidc-'
      args:
        executable: /bin/bash
      register: oidc_flag_count
      changed_when: false
      retries: 12
      delay: 5
      until: oidc_flag_count.stdout | int == 7
`

type kubernetesOIDCSettings struct {
	IssuerURL      string `json:"oidc_issuer_url"`
	ClientID       string `json:"oidc_client_id"`
	CAFile         string `json:"oidc_ca_file"`
	UsernameClaim  string `json:"oidc_username_claim"`
	UsernamePrefix string `json:"oidc_username_prefix"`
	GroupsClaim    string `json:"oidc_groups_claim"`
	GroupsPrefix   string `json:"oidc_groups_prefix"`
	APIPort        int    `json:"kubernetes_api_port"`
}

func (p *openstackBootstrapProvider) buildKubernetesOIDCActivationStep(cfg *v2.Config, lifecycle *kubesprayLifecycle) (bootstrapStep, error) {
	settings, err := resolveKubernetesOIDCSettings(cfg)
	if err != nil {
		return bootstrapStep{}, err
	}
	playbookPath := filepath.Join(lifecycle.stateDir, "activate-kubernetes-oidc.yml")
	varsPath := filepath.Join(lifecycle.stateDir, "activate-kubernetes-oidc-vars.json")
	ansiblePlaybook := filepath.Join(lifecycle.venvPath, "bin", "ansible-playbook")
	inventoryPath := filepath.Join(lifecycle.inventoryPath, "inventory.yaml")

	return bootstrapStep{
		ID:          "kubespray-activate-oidc",
		Description: "Activate Kubernetes OIDC authentication after the issuer is ready",
		Plan: BootstrapPlanStep{
			ID:          "kubespray-activate-oidc",
			Action:      "Wait for the OIDC issuer and roll API servers serially with OIDC enabled",
			WorkingDir:  lifecycle.stateDir,
			Commands:    []BootstrapPlanCommand{commandPlan(ansiblePlaybook, "-i", inventoryPath, playbookPath, "-e", "@"+varsPath)},
			Environment: envPlanFromMap(lifecycle.environment(), nil),
			Reads:       []string{inventoryPath},
			Writes:      []string{playbookPath, varsPath, "/etc/kubernetes/manifests/kube-apiserver.yaml on each control plane"},
			Notes:       []string{"The issuer discovery document must be reachable and match the configured issuer. Control planes are changed and health-checked one at a time; manifest backups are kept outside the kubelet static-pod directory."},
		},
		Run: func(ctx context.Context) error {
			if err := lifecycle.loadOutputs(); err != nil {
				return fmt.Errorf("load Kubespray outputs for OIDC activation: %w", err)
			}
			settings.APIPort = lifecycle.outputs.APIPort
			if settings.APIPort == 0 {
				settings.APIPort = 6443
			}
			if err := writeKubernetesOIDCActivationFiles(playbookPath, varsPath, settings); err != nil {
				return err
			}
			if _, err := p.runner.Run(ctx, lifecycle.stateDir, lifecycle.environment(), ansiblePlaybook, "-i", inventoryPath, playbookPath, "-e", "@"+varsPath); err != nil {
				return fmt.Errorf("activate Kubernetes OIDC authentication: %w", err)
			}
			return nil
		},
	}, nil
}

func resolveKubernetesOIDCSettings(cfg *v2.Config) (kubernetesOIDCSettings, error) {
	if cfg == nil {
		return kubernetesOIDCSettings{}, fmt.Errorf("configuration is required")
	}
	oidc := cfg.OpenCenter.Cluster.Kubernetes.OIDC
	internalIssuerURL, internalClientID := internalOIDCSettings(cfg)
	settings := kubernetesOIDCSettings{
		IssuerURL:      firstNonEmpty(oidc.KubeOIDCURL, oidc.IssuerURL, internalIssuerURL),
		ClientID:       firstNonEmpty(oidc.KubeOIDCClientID, oidc.ClientID, internalClientID),
		CAFile:         firstNonEmpty(oidc.KubeOIDCCAFile, "/etc/ssl/certs/ca-certificates.crt"),
		UsernameClaim:  firstNonEmpty(oidc.KubeOIDCUsernameClaim, oidc.UsernameClaim, "preferred_username"),
		UsernamePrefix: firstNonEmpty(oidc.KubeOIDCUsernamePrefix, "oidc:"),
		GroupsClaim:    firstNonEmpty(oidc.KubeOIDCGroupsClaim, oidc.GroupsClaim, "groups"),
		GroupsPrefix:   firstNonEmpty(oidc.KubeOIDCGroupsPrefix, "oidc:"),
	}
	settings.IssuerURL = strings.TrimRight(settings.IssuerURL, "/")
	if settings.IssuerURL == "" {
		return kubernetesOIDCSettings{}, fmt.Errorf("OIDC issuer URL is required")
	}
	if settings.ClientID == "" {
		return kubernetesOIDCSettings{}, fmt.Errorf("OIDC client ID is required")
	}
	return settings, nil
}

func shouldActivateKubernetesOIDC(cfg *v2.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled {
		return true
	}
	identity := cfg.OpenCenter.Identity.OIDC
	source := firstNonEmpty(identity.Source, v2.OIDCSourceInternal)
	provider := firstNonEmpty(identity.Provider, v2.OIDCProviderKeycloak)
	return identity.Enabled && strings.EqualFold(source, v2.OIDCSourceInternal) &&
		strings.EqualFold(provider, v2.OIDCProviderKeycloak)
}

func internalOIDCSettings(cfg *v2.Config) (string, string) {
	if !shouldActivateKubernetesOIDC(cfg) || cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled {
		return "", ""
	}
	hostname := ""
	realm := "opencenter"
	clientID := "opencenter"
	if keycloak, ok := cfg.OpenCenter.Services["keycloak"].(*configservices.KeycloakConfig); ok {
		hostname = firstNonEmpty(keycloak.Hostname, strings.TrimPrefix(strings.TrimRight(keycloak.FrontendURL, "/"), "https://"))
		realm = firstNonEmpty(keycloak.Realm, realm)
		clientID = firstNonEmpty(keycloak.ClientID, clientID)
	}
	if headlamp, ok := cfg.OpenCenter.Services["headlamp"].(*configservices.HeadlampConfig); ok {
		if headlamp.OIDCIssuerURL != "" {
			return strings.TrimRight(headlamp.OIDCIssuerURL, "/"), firstNonEmpty(headlamp.OIDCClientID, clientID)
		}
		clientID = firstNonEmpty(headlamp.OIDCClientID, clientID)
	}
	hostname = firstNonEmpty(hostname, "auth."+cfg.OpenCenter.Cluster.ClusterFQDN)
	return "https://" + hostname + "/realms/" + realm, clientID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func writeKubernetesOIDCActivationFiles(playbookPath, varsPath string, settings kubernetesOIDCSettings) error {
	if err := os.MkdirAll(filepath.Dir(playbookPath), 0o700); err != nil {
		return fmt.Errorf("create OIDC activation state directory: %w", err)
	}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OIDC activation variables: %w", err)
	}
	for path, content := range map[string][]byte{
		playbookPath: []byte(oidcActivationPlaybook),
		varsPath:     encoded,
	} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return fmt.Errorf("write OIDC activation file %s: %w", path, err)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("secure OIDC activation file %s: %w", path, err)
		}
	}
	return nil
}
