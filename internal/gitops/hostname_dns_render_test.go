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
	"os"
	"path/filepath"
	"strings"
	"testing"

	registrydefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestGatewayHostnamesFollowClusterFQDNAfterBaseDomainChange is the end-to-end
// statement of the bug: a cluster created with `cluster init --base-domain`
// whose service hostnames were baked from the placeholder domain must still
// render a single-domain Gateway, because loader stage 4c heals the hostnames
// before the renderer ever sees them.
//
// It round-trips through the loader rather than hand-building a config so a
// regression in the wiring (not only in the sweep) fails here.
func TestGatewayHostnamesFollowClusterFQDNAfterBaseDomainChange(t *testing.T) {
	cfg := newDefault("doritos")
	cfg.OpenCenter.Meta.Region = "IAD3"
	cfg.OpenCenter.Cluster.BaseDomain = "k8s.opensnack.cloud"
	cfg.OpenCenter.Cluster.ClusterFQDN = "doritos.iad3.k8s.opensnack.cloud"

	// The state `cluster init --base-domain k8s.opensnack.cloud` leaves behind:
	// identity fields moved, hostname scalars still naming the placeholder.
	stale := "doritos." + strings.ToLower(v2.InitDefaultRegion) + "." + v2.DefaultBaseDomain()
	if kc, ok := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig); ok {
		kc.Hostname = "auth." + stale
	} else {
		t.Fatalf("keycloak service config has unexpected type %T", cfg.OpenCenter.Services["keycloak"])
	}
	if hc, ok := cfg.OpenCenter.Services["headlamp"].(*services.HeadlampConfig); ok {
		hc.Hostname = "dashboard." + stale
	} else {
		t.Fatalf("headlamp service config has unexpected type %T", cfg.OpenCenter.Services["headlamp"])
	}

	data, err := v2.MarshalPublicConfig(&cfg)
	if err != nil {
		t.Fatalf("MarshalPublicConfig() error = %v", err)
	}
	loaded, err := v2.NewConfigLoader(registrydefaults.NewRegistry()).LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes() error = %v", err)
	}
	if len(loaded.RepairedHostnameFields) == 0 {
		t.Fatal("loader repaired no hostnames; stage 4c is not wired in")
	}

	files, err := gatewayOverlayFilesRenderer(*loaded)
	if err != nil {
		t.Fatalf("gatewayOverlayFilesRenderer() error = %v", err)
	}
	gateway := files["gateway.yaml"]

	const wantFQDN = "doritos.iad3.k8s.opensnack.cloud"
	for _, want := range []string{
		"auth." + wantFQDN,
		"dashboard." + wantFQDN,
		"gitops." + wantFQDN,
	} {
		if !strings.Contains(gateway, want) {
			t.Errorf("gateway.yaml missing hostname %q\nGot:\n%s", want, gateway)
		}
	}
	if strings.Contains(gateway, v2.DefaultBaseDomain()) {
		t.Errorf("gateway.yaml still references the placeholder domain\nGot:\n%s", gateway)
	}
}

// TestGatewayDefaultIssuerHonoursNoneSentinel pins that dns_provider: none is a
// deliberate choice for the internal CA, not an accident of fallthrough.
func TestGatewayDefaultIssuerHonoursNoneSentinel(t *testing.T) {
	cfg := newDefault("doritos")
	certManager, ok := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if !ok || certManager == nil {
		t.Fatal("cert-manager service config missing")
	}
	certManager.Enabled = true
	certManager.DNSProvider = "none"
	// A credential is present on purpose: `none` must still win, because the
	// operator turned ACME off rather than forgot to configure it.
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cf": {Enabled: true, APIToken: "tok"},
	}

	if got := gatewayDefaultIssuer(cfg); got != "rackspace-ca" {
		t.Errorf("gatewayDefaultIssuer() with dns_provider none = %q, want %q", got, "rackspace-ca")
	}

	actions, err := planCertManagerDynamicActions(cfg, nil)
	if err != nil {
		t.Fatalf("planCertManagerDynamicActions() error = %v", err)
	}
	for _, action := range actions {
		if strings.Contains(action.Output, "letsencrypt-") {
			t.Errorf("dns_provider none still rendered ACME issuer %q", action.Output)
		}
	}
}

func TestGatewayDefaultIssuerUsesResolvedCredential(t *testing.T) {
	cfg := newDefault("doritos")
	certManager, ok := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if !ok || certManager == nil {
		t.Fatal("cert-manager service config missing")
	}
	certManager.Enabled = true
	certManager.DNSProvider = "cloudflare"
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"alpha": {Enabled: true, APIToken: "tok-a", DNSZones: []string{"alpha.example"}},
		"zulu":  {Enabled: true, APIToken: "tok-z", DNSZones: []string{"zulu.example"}},
	}

	// Collation default, unchanged from before the resolver existed.
	if got, want := gatewayDefaultIssuer(cfg), "letsencrypt-alpha"; got != want {
		t.Errorf("gatewayDefaultIssuer() = %q, want %q", got, want)
	}

	// The cluster default now steers the Gateway annotation instead of the
	// issuer name happening to sort first.
	certManager.DefaultCredential = "zulu"
	if got, want := gatewayDefaultIssuer(cfg), "letsencrypt-zulu"; got != want {
		t.Errorf("gatewayDefaultIssuer() with default_credential = %q, want %q", got, want)
	}
}

// TestKeycloakCertificateAndGatewayAgreeOnIssuer covers the disagreement that
// made a listener hold a self-signed leaf where a public certificate was
// expected: the Gateway annotation followed dns_provider while the service
// Certificate stayed pinned to rackspace-ca.
func TestKeycloakCertificateAndGatewayAgreeOnIssuer(t *testing.T) {
	cfg := newDefault("issuer-agree")
	certManager, ok := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if !ok || certManager == nil {
		t.Fatal("cert-manager service config missing")
	}
	certManager.Enabled = true
	certManager.DNSProvider = "cloudflare"
	certManager.Email = "ops@example.com"
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cf": {Enabled: true, APIToken: "tok", DNSZones: []string{"k8s.opensnack.cloud"}},
	}

	want := gatewayDefaultIssuer(cfg)
	if want == "rackspace-ca" {
		t.Fatalf("fixture produced no ACME issuer; dns_provider wiring regressed (issuer %q)", want)
	}

	dst := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	path := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(),
		"services", "keycloak", "20-keycloak", "keycloak-certificate.yaml")
	certificate, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(certificate), "name: "+want) {
		t.Errorf("keycloak Certificate issuer does not follow the Gateway issuer %q\nGot:\n%s", want, certificate)
	}
}
