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

package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// dnsTestConfig returns a config with a typed, enabled cert-manager service so
// provider resolution sees it.
func dnsTestConfig(t *testing.T, dnsProvider string) *Config {
	t.Helper()
	cfg, err := NewV2DefaultWithRegion("doritos", "openstack", "IAD3")
	if err != nil {
		t.Fatalf("NewV2DefaultWithRegion() error = %v", err)
	}
	certManager, ok := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if !ok || certManager == nil {
		t.Fatal("cert-manager service config missing from defaults")
	}
	certManager.Enabled = true
	certManager.DNSProvider = dnsProvider
	return cfg
}

func TestResolvedDNSProviderNormalizesNoneAliases(t *testing.T) {
	for _, alias := range []string{"none", "off", "disabled", "internal", "NONE", " Off "} {
		cfg := dnsTestConfig(t, alias)
		if got := cfg.ResolvedDNSProvider(); got != string(services.DNSProviderNone) {
			t.Errorf("ResolvedDNSProvider() with dns_provider %q = %q, want %q",
				alias, got, services.DNSProviderNone)
		}
		if !cfg.DNSProviderDisabled() {
			t.Errorf("DNSProviderDisabled() = false for dns_provider %q", alias)
		}
	}
}

// TestResolvedDNSProviderNeverAutoSelectsNone pins the sentinel as opt-in: an
// unset dns_provider must keep resolving to a real provider, or every existing
// cluster would silently lose public certificates.
func TestResolvedDNSProviderNeverAutoSelectsNone(t *testing.T) {
	cfg := dnsTestConfig(t, "")
	if got := cfg.ResolvedDNSProvider(); got == "" || got == string(services.DNSProviderNone) {
		t.Fatalf("ResolvedDNSProvider() with unset dns_provider = %q, want an auto-selected real provider", got)
	}
	if cfg.DNSProviderDisabled() {
		t.Fatal("DNSProviderDisabled() = true for an unset dns_provider; ACME would silently turn off")
	}
}

func TestResolvedDNSCredentialPrecedence(t *testing.T) {
	cfg := dnsTestConfig(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]CertManagerCloudflareCredential{
		"alpha":    {Enabled: true, APIToken: "tok-a", DNSZones: []string{"alpha.example.com"}},
		"zulu":     {Enabled: true, APIToken: "tok-z", DNSZones: []string{"zulu.example.com"}},
		"disabled": {Enabled: false, APIToken: "tok-d"},
	}

	// No override: the lowest-sorted name preserves the historical pick.
	if got := cfg.ResolvedDNSCredential("cloudflare", "", ""); got != "alpha" {
		t.Errorf("default resolution = %q, want %q (lowest-sorted, historical behaviour)", got, "alpha")
	}

	// Cluster default beats collation.
	if certManager, ok := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig); ok {
		certManager.DefaultCredential = "zulu"
	}
	if got := cfg.ResolvedDNSCredential("cloudflare", "", ""); got != "zulu" {
		t.Errorf("cluster default_credential = %q, want %q", got, "zulu")
	}

	// A per-consumer override beats the cluster default.
	if got := cfg.ResolvedDNSCredential("cloudflare", "alpha", ""); got != "alpha" {
		t.Errorf("per-consumer dns_credential = %q, want %q", got, "alpha")
	}

	// Zone coverage wins over collation when no explicit name is given: naming
	// an issuer whose selector cannot solve for the zone produces a Certificate
	// that never becomes Ready.
	if got := cfg.ResolvedDNSCredential("cloudflare", "", "grafana.zulu.example.com"); got != "zulu" {
		t.Errorf("zone-matched credential = %q, want %q", got, "zulu")
	}

	// A disabled credential is never selectable.
	if got := cfg.ResolvedDNSCredential("cloudflare", "disabled", ""); got == "disabled" {
		t.Error("resolver selected a disabled credential")
	}
}

func TestResolvedDNSCredentialEmptyForNoneAndDesignate(t *testing.T) {
	for _, provider := range []string{"none", "designate"} {
		cfg := dnsTestConfig(t, provider)
		cfg.Secrets.CertManager.Cloudflare = map[string]CertManagerCloudflareCredential{
			"cf": {Enabled: true, APIToken: "tok"},
		}
		if got := cfg.ResolvedDNSCredential(provider, "cf", ""); got != "" {
			t.Errorf("ResolvedDNSCredential(%q) = %q, want empty", provider, got)
		}
	}
}

// TestResolvedDNSCredentialScansAcrossProviders pins the fix for a real
// regression: an enabled AWS credential with dns_provider unset resolves to the
// auto-selected cloudflare. Filtering strictly by provider found nothing and
// dropped the Gateway to the internal CA.
func TestResolvedDNSCredentialScansAcrossProviders(t *testing.T) {
	cfg := dnsTestConfig(t, "")
	cfg.Secrets.CertManager.AWS = map[string]CertManagerAWSCredential{
		"ed-2": {Enabled: true, AWSAccessKey: "AKIA", AWSSecretAccessKey: "s", Region: "us-east-1"},
	}
	if got := cfg.ResolvedDNSCredential("cloudflare", "", ""); got != "ed-2" {
		t.Errorf("ResolvedDNSCredential() = %q, want %q (AWS credential under an auto-selected cloudflare)", got, "ed-2")
	}
}

func TestZoneCoversHostname(t *testing.T) {
	cases := []struct {
		zone, hostname string
		want           bool
	}{
		{"k8s.opensnack.cloud", "auth.doritos.iad3.k8s.opensnack.cloud", true},
		{"k8s.opensnack.cloud", "k8s.opensnack.cloud", true},
		{"k8s.opensnack.cloud", "auth.doritos.dfw3.k8s.opencenter.cloud", false},
		// A zone that is only a string suffix of the hostname's *label* is not a
		// DNS parent zone.
		{"opensnack.cloud", "auth.notopensnack.cloud", false},
		{"", "auth.example.com", false},
	}
	for _, tc := range cases {
		if got := zoneCoversHostname([]string{tc.zone}, tc.hostname); got != tc.want {
			t.Errorf("zoneCoversHostname(%q, %q) = %v, want %v", tc.zone, tc.hostname, got, tc.want)
		}
	}
}
