// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const (
	structKind = reflect.Struct
	stringKind = reflect.String
)

// certManagerCredentialData holds the template context for rendering a single
// cert-manager credential secret and its corresponding issuer.
type certManagerCredentialData struct {
	// Name is the credential identifier (map key from config).
	Name string

	// Provider is "aws" or "cloudflare".
	Provider string

	// AWS-specific fields
	AWSAccessKey       string
	AWSSecretAccessKey string
	Region             string

	// Cloudflare-specific fields
	APIToken string

	// DNSZones for the issuer selector.
	DNSZones []string

	// Cluster-level fields needed by the issuer template.
	ClusterName       string
	ClusterFQDN       string
	LetsEncryptServer string
	Email             string
}

// renderCertManagerDynamicFiles renders the dynamic cert-manager credential secrets
// and issuer files based on enabled credentials in the configuration.
// It also renders the kustomization.yaml that references all generated files.
//
// This function is called as part of the cert-manager service rendering pipeline
// after the descriptor-driven static files have been written.
func renderCertManagerDynamicFiles(cfg v2.Config, targetDir string, workspace *GitOpsWorkspace) error {
	certManagerSvc, exists := cfg.OpenCenter.Services["cert-manager"]
	if !exists || IsServiceDisabled(certManagerSvc) {
		return nil
	}

	// Validate enabled credentials have required fields
	if err := validateCertManagerCredentials(cfg); err != nil {
		return err
	}

	awsCreds := cfg.EnabledCertManagerAWSCredentials()
	cfCreds := cfg.EnabledCertManagerCloudflareCredentials()

	// Collect all credential data for rendering
	var credentials []certManagerCredentialData
	for name, cred := range awsCreds {
		credentials = append(credentials, certManagerCredentialData{
			Name:               name,
			Provider:           "aws",
			AWSAccessKey:       cred.AWSAccessKey,
			AWSSecretAccessKey: cred.AWSSecretAccessKey,
			Region:             cred.Region,
			DNSZones:           cred.DNSZones,
		})
	}
	for name, cred := range cfCreds {
		credentials = append(credentials, certManagerCredentialData{
			Name:     name,
			Provider: "cloudflare",
			APIToken: cred.APIToken,
			DNSZones: cred.DNSZones,
		})
	}

	// Sort for deterministic output
	sort.Slice(credentials, func(i, j int) bool {
		if credentials[i].Provider != credentials[j].Provider {
			return credentials[i].Provider < credentials[j].Provider
		}
		return credentials[i].Name < credentials[j].Name
	})

	// Populate cluster-level fields from config
	certManagerConfig := extractCertManagerConfig(cfg)
	for i := range credentials {
		credentials[i].ClusterName = cfg.ClusterName()
		credentials[i].ClusterFQDN = cfg.OpenCenter.Cluster.ClusterFQDN
		credentials[i].LetsEncryptServer = certManagerConfig.letsEncryptServer
		credentials[i].Email = certManagerConfig.email
		// Use credential-level region if set, otherwise fall back to service-level
		if credentials[i].Region == "" {
			credentials[i].Region = certManagerConfig.region
		}
	}

	// Render each credential secret + issuer
	for _, cred := range credentials {
		if err := renderCredentialSecret(cred, targetDir, workspace); err != nil {
			return fmt.Errorf("rendering credential secret %s/%s: %w", cred.Provider, cred.Name, err)
		}
		if err := renderCredentialIssuer(cred, targetDir, workspace); err != nil {
			return fmt.Errorf("rendering credential issuer %s/%s: %w", cred.Provider, cred.Name, err)
		}
	}

	// Render designate issuer if DNS provider is designate
	dnsProvider, _ := getStringField(certManagerSvc, "DNSProvider")
	if dnsProvider == "designate" {
		issuerData := certManagerCredentialData{
			Name:              "designate",
			Provider:          "designate",
			ClusterName:       cfg.ClusterName(),
			ClusterFQDN:       cfg.OpenCenter.Cluster.ClusterFQDN,
			LetsEncryptServer: certManagerConfig.letsEncryptServer,
			Email:             certManagerConfig.email,
		}
		if err := renderInlineTemplate(issuerDesignateTemplate, "letsencrypt-designate-issuer.yaml", issuerData, targetDir, workspace); err != nil {
			return fmt.Errorf("rendering designate issuer: %w", err)
		}
	}

	// Render the kustomization.yaml that references all generated files
	return renderCertManagerKustomization(cfg, credentials, targetDir, workspace)
}

// certManagerServiceConfig holds extracted cert-manager service configuration.
type certManagerServiceConfig struct {
	letsEncryptServer string
	email             string
	region            string
}

// validateCertManagerCredentials checks that all enabled credentials have their
// required secret fields populated. Returns an error listing all missing fields.
func validateCertManagerCredentials(cfg v2.Config) error {
	var errors []string

	for name, cred := range cfg.Secrets.CertManager.AWS {
		if !cred.Enabled {
			continue
		}
		if cred.AWSAccessKey == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_access_key is required when enabled", name))
		}
		if cred.AWSSecretAccessKey == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_secret_access_key is required when enabled", name))
		}
	}

	for name, cred := range cfg.Secrets.CertManager.Cloudflare {
		if !cred.Enabled {
			continue
		}
		if cred.APIToken == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.cloudflare.%s.api_token is required when enabled", name))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cert-manager credential validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}
	return nil
}

// extractCertManagerConfig extracts cert-manager service config fields using reflection
// since the service config is stored as any in the ServiceMap.
func extractCertManagerConfig(cfg v2.Config) certManagerServiceConfig {
	result := certManagerServiceConfig{
		letsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
		email:             "mpk-support@rackspace.com",
	}

	svc, exists := cfg.OpenCenter.Services["cert-manager"]
	if !exists {
		return result
	}

	if v, ok := getStringField(svc, "LetsEncryptServer"); ok && v != "" {
		result.letsEncryptServer = v
	}
	if v, ok := getStringField(svc, "Email"); ok && v != "" {
		result.email = v
	}
	if v, ok := getStringField(svc, "Region"); ok && v != "" {
		result.region = v
	}

	return result
}

// getStringField extracts a string field from a struct by name using reflection.
func getStringField(obj any, fieldName string) (string, bool) {
	if obj == nil {
		return "", false
	}
	val := derefValue(obj)
	if !val.IsValid() || val.Kind() != structKind {
		return "", false
	}
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != stringKind {
		return "", false
	}
	return field.String(), true
}

// derefValue dereferences pointers and interfaces to get the underlying value.
func derefValue(obj any) reflect.Value {
	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return reflect.Value{}
		}
		val = val.Elem()
	}
	return val
}

const awsSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-aws-credentials-secret-{{ .Name }}
type: Opaque
stringData:
  access-key-id: {{ .AWSAccessKey }}
  secret-access-key: {{ .AWSSecretAccessKey }}
`

const cloudflareSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-cloudflare-credentials-secret-{{ .Name }}
type: Opaque
stringData:
  api-token: {{ .APIToken }}
`

const issuerAWSTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          route53:
            region: {{ .Region }}
            accessKeyIDSecretRef:
              name: "opencenter-aws-credentials-secret-{{ .Name }}"
              key: access-key-id
            secretAccessKeySecretRef:
              name: "opencenter-aws-credentials-secret-{{ .Name }}"
              key: secret-access-key
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const issuerCloudflareTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          cloudflare:
            apiTokenSecretRef:
              name: "opencenter-cloudflare-credentials-secret-{{ .Name }}"
              key: api-token
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const issuerDesignateTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          webhook:
            groupName: acme.syseleven.de
            solverName: designatedns
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const kustomizationTemplate = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
{{- if .HasDesignate }}
  - "./opencenter-openstack-designate-credentials-secret.yaml"
  - "./letsencrypt-designate-issuer.yaml"
{{- end }}
{{- range .Credentials }}
{{- if eq .Provider "aws" }}
  - "./opencenter-aws-credentials-secret-{{ .Name }}.yaml"
{{- end }}
{{- if eq .Provider "cloudflare" }}
  - "./opencenter-cloudflare-credentials-secret-{{ .Name }}.yaml"
{{- end }}
  - "./letsencrypt-{{ .Name }}-issuer.yaml"
{{- end }}
secretGenerator:
  - name: cert-manager-values-override
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
`

func renderCredentialSecret(cred certManagerCredentialData, targetDir string, workspace *GitOpsWorkspace) error {
	var tmplStr string
	var filename string

	switch cred.Provider {
	case "aws":
		tmplStr = awsSecretTemplate
		filename = fmt.Sprintf("opencenter-aws-credentials-secret-%s.yaml", cred.Name)
	case "cloudflare":
		tmplStr = cloudflareSecretTemplate
		filename = fmt.Sprintf("opencenter-cloudflare-credentials-secret-%s.yaml", cred.Name)
	default:
		return fmt.Errorf("unsupported provider %q", cred.Provider)
	}

	return renderInlineTemplate(tmplStr, filename, cred, targetDir, workspace)
}

func renderCredentialIssuer(cred certManagerCredentialData, targetDir string, workspace *GitOpsWorkspace) error {
	var tmplStr string

	switch cred.Provider {
	case "aws":
		tmplStr = issuerAWSTemplate
	case "cloudflare":
		tmplStr = issuerCloudflareTemplate
	default:
		return fmt.Errorf("unsupported provider %q", cred.Provider)
	}

	filename := fmt.Sprintf("letsencrypt-%s-issuer.yaml", cred.Name)
	return renderInlineTemplate(tmplStr, filename, cred, targetDir, workspace)
}

// kustomizationData holds the template context for the cert-manager kustomization.yaml.
type kustomizationData struct {
	Credentials  []certManagerCredentialData
	HasDesignate bool
}

func renderCertManagerKustomization(cfg v2.Config, credentials []certManagerCredentialData, targetDir string, workspace *GitOpsWorkspace) error {
	// Check if designate is the DNS provider
	certManagerConfig := extractCertManagerConfig(cfg)
	hasDesignate := false
	if dnsProvider, ok := getStringField(cfg.OpenCenter.Services["cert-manager"], "DNSProvider"); ok {
		hasDesignate = dnsProvider == "designate"
	}
	_ = certManagerConfig // used above via getStringField

	data := kustomizationData{
		Credentials:  credentials,
		HasDesignate: hasDesignate,
	}
	return renderInlineTemplate(kustomizationTemplate, "kustomization.yaml", data, targetDir, workspace)
}

// renderInlineTemplate parses and executes an inline Go template string, writing
// the result to the specified file within the target directory.
func renderInlineTemplate(tmplStr, filename string, data any, targetDir string, workspace *GitOpsWorkspace) error {
	funcMap := sprig.TxtFuncMap()
	t, err := template.New(filename).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parsing template for %s: %w", filename, err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template for %s: %w", filename, err)
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		return nil
	}

	dst := filepath.Join(targetDir, filename)
	relPath, err := filepath.Rel(workspace.RootDir, dst)
	if err != nil {
		return fmt.Errorf("computing relative path for %s: %w", filename, err)
	}

	writer := NewAtomicWriter(workspace)
	return writer.WriteFileString(relPath, buf.String(), 0o644)
}
