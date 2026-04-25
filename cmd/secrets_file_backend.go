package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

type configSecretPayloadKind string

const (
	configSecretScalar configSecretPayloadKind = "scalar"
	configSecretObject configSecretPayloadKind = "yaml-or-json"
)

type configSecretEntry struct {
	Name        string
	Type        string
	Location    string
	Description string
	PayloadKind configSecretPayloadKind
	Present     func(*v2.Config) bool
	Get         func(*v2.Config) interface{}
	Set         func(*v2.Config, []byte) error
	Delete      func(*v2.Config)
}

type configSecretMetadata struct {
	Name        string                  `json:"name" yaml:"name"`
	Type        string                  `json:"type" yaml:"type"`
	Location    string                  `json:"location" yaml:"location"`
	Description string                  `json:"description" yaml:"description"`
	PayloadKind configSecretPayloadKind `json:"payload_kind" yaml:"payload_kind"`
}

func configSecretCatalog() []configSecretEntry {
	return []configSecretEntry{
		{
			Name:        "cert-manager-aws-credentials",
			Type:        "aws-credentials",
			Location:    "config: secrets.cert_manager",
			Description: "cert-manager Route53 AWS credentials",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.CertManager.AWSAccessKey != "" || cfg.Secrets.CertManager.AWSSecretAccessKey != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.CertManager
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value v2.CertManagerSecrets
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse cert-manager credentials payload: %w", err)
				}
				cfg.Secrets.CertManager = value
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.CertManager = v2.CertManagerSecrets{}
			},
		},
		{
			Name:        "keycloak-admin-password",
			Type:        "password",
			Location:    "config: secrets.keycloak.admin_password",
			Description: "Keycloak admin password",
			PayloadKind: configSecretScalar,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.Keycloak.AdminPassword != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.Keycloak.AdminPassword
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				cfg.Secrets.Keycloak.AdminPassword = normalizeScalarSecret(payload)
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.Keycloak.AdminPassword = ""
			},
		},
		{
			Name:        "grafana-admin-password",
			Type:        "password",
			Location:    "config: secrets.grafana.admin_password",
			Description: "Grafana admin password",
			PayloadKind: configSecretScalar,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.Grafana.AdminPassword != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.Grafana.AdminPassword
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				cfg.Secrets.Grafana.AdminPassword = normalizeScalarSecret(payload)
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.Grafana.AdminPassword = ""
			},
		},
		{
			Name:        "headlamp-oidc-client-secret",
			Type:        "oidc-secret",
			Location:    "config: secrets.headlamp.oidc_client_secret",
			Description: "Headlamp OIDC client secret",
			PayloadKind: configSecretScalar,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.Headlamp.OIDCClientSecret != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.Headlamp.OIDCClientSecret
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				cfg.Secrets.Headlamp.OIDCClientSecret = normalizeScalarSecret(payload)
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.Headlamp.OIDCClientSecret = ""
			},
		},
		{
			Name:        "weave-gitops-password",
			Type:        "password",
			Location:    "config: secrets.weave_gitops.password",
			Description: "Weave GitOps admin password",
			PayloadKind: configSecretScalar,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.WeaveGitOps.Password != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.WeaveGitOps.Password
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				cfg.Secrets.WeaveGitOps.Password = normalizeScalarSecret(payload)
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.WeaveGitOps.Password = ""
			},
		},
		{
			Name:        "vsphere-csi-credentials",
			Type:        "credentials",
			Location:    "config: secrets.vsphere_csi",
			Description: "vSphere CSI credentials and connection settings",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				secret := cfg.Secrets.VSphereCsi
				return secret.VCenterHost != "" || secret.Username != "" || secret.Password != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.VSphereCsi
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value v2.VSphereCsiSecrets
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse vSphere CSI credentials payload: %w", err)
				}
				cfg.Secrets.VSphereCsi = value
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.VSphereCsi = v2.VSphereCsiSecrets{}
			},
		},
		{
			Name:        "alert-proxy-credentials",
			Type:        "credentials",
			Location:    "config: secrets.alert_proxy",
			Description: "Alert proxy service credentials",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				secret := cfg.Secrets.AlertProxy
				return secret.CoreDeviceId != "" || secret.AccountServiceToken != "" || secret.CoreAccountNumber != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.AlertProxy
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value v2.AlertProxySecrets
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse alert-proxy credentials payload: %w", err)
				}
				cfg.Secrets.AlertProxy = value
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.AlertProxy = v2.AlertProxySecrets{}
			},
		},
		{
			Name:        "loki-s3-credentials",
			Type:        "s3-credentials",
			Location:    "config: secrets.loki",
			Description: "Loki S3 credentials",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.Loki.S3AccessKeyID != "" || cfg.Secrets.Loki.S3SecretAccessKey != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return struct {
					S3AccessKeyID     string `yaml:"s3_access_key_id" json:"s3_access_key_id"`
					S3SecretAccessKey string `yaml:"s3_secret_access_key" json:"s3_secret_access_key"`
				}{
					S3AccessKeyID:     cfg.Secrets.Loki.S3AccessKeyID,
					S3SecretAccessKey: cfg.Secrets.Loki.S3SecretAccessKey,
				}
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value struct {
					S3AccessKeyID     string `yaml:"s3_access_key_id" json:"s3_access_key_id"`
					S3SecretAccessKey string `yaml:"s3_secret_access_key" json:"s3_secret_access_key"`
				}
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse Loki S3 credentials payload: %w", err)
				}
				cfg.Secrets.Loki.S3AccessKeyID = value.S3AccessKeyID
				cfg.Secrets.Loki.S3SecretAccessKey = value.S3SecretAccessKey
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.Loki.S3AccessKeyID = ""
				cfg.Secrets.Loki.S3SecretAccessKey = ""
			},
		},
		{
			Name:        "tempo-s3-credentials",
			Type:        "s3-credentials",
			Location:    "config: secrets.tempo",
			Description: "Tempo S3 credentials",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.Tempo.AccessKey != "" || cfg.Secrets.Tempo.SecretKey != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.Tempo
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value v2.TempoSecrets
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse Tempo credentials payload: %w", err)
				}
				cfg.Secrets.Tempo = value
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.Tempo = v2.TempoSecrets{}
			},
		},
		{
			Name:        "ssh-key",
			Type:        "ssh-key",
			Location:    "config: secrets.ssh_key",
			Description: "Cluster SSH private and public key material",
			PayloadKind: configSecretObject,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.SSHKey.Private != "" || cfg.Secrets.SSHKey.Public != "" || cfg.Secrets.SSHKey.Cypher != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.SSHKey
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				var value v2.SSHKeyConfig
				if err := yaml.Unmarshal(payload, &value); err != nil {
					return fmt.Errorf("failed to parse SSH key payload: %w", err)
				}
				cfg.Secrets.SSHKey = value
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.SSHKey = v2.SSHKeyConfig{}
			},
		},
		{
			Name:        "sops-age-key",
			Type:        "age-key",
			Location:    "config: secrets.sops_age_key_file",
			Description: "Path to the SOPS Age key file",
			PayloadKind: configSecretScalar,
			Present: func(cfg *v2.Config) bool {
				return cfg.Secrets.SopsAgeKeyFile != ""
			},
			Get: func(cfg *v2.Config) interface{} {
				return cfg.Secrets.SopsAgeKeyFile
			},
			Set: func(cfg *v2.Config, payload []byte) error {
				cfg.Secrets.SopsAgeKeyFile = normalizeScalarSecret(payload)
				return nil
			},
			Delete: func(cfg *v2.Config) {
				cfg.Secrets.SopsAgeKeyFile = ""
			},
		},
	}
}

func normalizeScalarSecret(payload []byte) string {
	return strings.TrimRight(string(payload), "\r\n")
}

func findConfigSecretEntry(name string) (*configSecretEntry, error) {
	for _, entry := range configSecretCatalog() {
		if entry.Name == name {
			entryCopy := entry
			return &entryCopy, nil
		}
	}
	return nil, fmt.Errorf("unknown config-backed secret %q", name)
}

func listConfigMappedSecrets(out io.Writer, cfg *v2.Config, format string) error {
	var secretsList []configSecretMetadata
	for _, entry := range configSecretCatalog() {
		if !entry.Present(cfg) {
			continue
		}
		secretsList = append(secretsList, configSecretMetadata{
			Name:        entry.Name,
			Type:        entry.Type,
			Location:    entry.Location,
			Description: entry.Description,
			PayloadKind: entry.PayloadKind,
		})
	}

	switch format {
	case "json":
		return json.NewEncoder(out).Encode(secretsList)
	case "yaml":
		return yaml.NewEncoder(out).Encode(secretsList)
	default:
		w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tLOCATION")
		for _, secret := range secretsList {
			fmt.Fprintf(w, "%s\t%s\t%s\n", secret.Name, secret.Type, secret.Location)
		}
		return w.Flush()
	}
}

func describeConfigSecret(out io.Writer, cfg *v2.Config, name string, format string) error {
	entry, err := findConfigSecretEntry(name)
	if err != nil {
		return err
	}
	if !entry.Present(cfg) {
		return fmt.Errorf("secret %q is not set", name)
	}

	metadata := configSecretMetadata{
		Name:        entry.Name,
		Type:        entry.Type,
		Location:    entry.Location,
		Description: entry.Description,
		PayloadKind: entry.PayloadKind,
	}

	switch format {
	case "json":
		return json.NewEncoder(out).Encode(metadata)
	case "yaml":
		return yaml.NewEncoder(out).Encode(metadata)
	default:
		if _, err := fmt.Fprintf(out, "Name: %s\n", metadata.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Type: %s\n", metadata.Type); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Location: %s\n", metadata.Location); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Description: %s\n", metadata.Description); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "Payload: %s\n", metadata.PayloadKind)
		return err
	}
}

func getConfigSecret(cfg *v2.Config, name string, outputFile string, show bool) error {
	entry, err := findConfigSecretEntry(name)
	if err != nil {
		return err
	}
	if !entry.Present(cfg) {
		return fmt.Errorf("secret %q is not set", name)
	}

	payload, err := marshalConfigSecretPayload(entry, cfg)
	if err != nil {
		return err
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, payload, 0o600); err != nil {
			return err
		}
		fmt.Printf("Secret '%s' saved to %s\n", name, outputFile)
	}
	if show {
		if outputFile == "" {
			fmt.Fprintln(os.Stderr, "Warning: Printing secret to stdout is insecure.")
		} else {
			fmt.Println("--- Secret Content ---")
		}
		fmt.Println(string(payload))
	}
	return nil
}

func setConfigSecret(ctx context.Context, cfg *v2.Config, name string, payload []byte) error {
	entry, err := findConfigSecretEntry(name)
	if err != nil {
		return err
	}
	if err := entry.Set(cfg, payload); err != nil {
		return err
	}
	return saveConfig(ctx, *cfg)
}

func deleteConfigSecret(ctx context.Context, cfg *v2.Config, name string) error {
	entry, err := findConfigSecretEntry(name)
	if err != nil {
		return err
	}
	entry.Delete(cfg)
	return saveConfig(ctx, *cfg)
}

func marshalConfigSecretPayload(entry *configSecretEntry, cfg *v2.Config) ([]byte, error) {
	value := entry.Get(cfg)
	switch entry.PayloadKind {
	case configSecretScalar:
		return []byte(fmt.Sprint(value)), nil
	case configSecretObject:
		data, err := yaml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal secret payload: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported payload kind %q", entry.PayloadKind)
	}
}
