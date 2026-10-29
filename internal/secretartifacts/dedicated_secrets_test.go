package secretartifacts

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func dedicatedConfig() *v2.Config {
	return &v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Services: v2.ServiceMap{
				"keycloak":              &services.BaseConfig{Enabled: true},
				"kube-prometheus-stack": &services.BaseConfig{Enabled: true},
			},
			Identity: v2.IdentityConfig{OIDC: v2.IdentityOIDCConfig{Enabled: true}},
		},
		Secrets: v2.SecretsConfig{
			Grafana: v2.GrafanaSecrets{AdminUser: "admin", AdminPassword: "grafana-pass"},
			Keycloak: v2.KeycloakSecrets{
				ClientSecret:        "keycloak-client-pass",
				GrafanaClientSecret: "grafana-oauth-pass",
			},
		},
	}
}

func artifactPaths(artifacts []Artifact) map[string]Artifact {
	byPath := make(map[string]Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byPath[artifact.Path] = artifact
	}
	return byPath
}

// The three renderer-emitted Secrets carry credentials interpolated from the
// cluster config. If the planner stops claiming them, `cluster validate` fails
// on plaintext material that `secrets sync` reports as unchanged, leaving the
// operator no way to pass validation.
func TestPlanIncludesDedicatedSecretArtifacts(t *testing.T) {
	byPath := artifactPaths(mustPlan(t, dedicatedConfig()))

	admin, ok := byPath["services/kube-prometheus-stack/"+GrafanaAdminPasswordSecretFilename]
	require.True(t, ok, "grafana admin Secret must be sync-owned")
	require.Equal(t, map[string]interface{}{"admin-user": "admin", "admin-password": "grafana-pass"}, admin.Payload)

	oauth, ok := byPath["services/kube-prometheus-stack/"+GrafanaOAuthSecretFilename]
	require.True(t, ok, "grafana oauth Secret must be sync-owned")
	require.Equal(t, "grafana-oauth", oauth.Payload["client_id"], "Keycloak rejects any other client id")
	require.Equal(t, "grafana-oauth-pass", oauth.Payload["client_secret"])

	gateway, ok := byPath["services/oidc-security-policies/"+GatewayOIDCSecretFilename]
	require.True(t, ok, "gateway OIDC Secret must be sync-owned")
	require.Equal(t, map[string]interface{}{"client-secret": "keycloak-client-pass"}, gateway.Payload)
}

// The gateway Secret is rendered into services/oidc-security-policies by the
// keycloak descriptor, not into services/keycloak. A path that does not match
// the renderer leaves the rendered plaintext file in place and adds an orphan.
func TestDedicatedArtifactPathsMatchRenderers(t *testing.T) {
	for _, tc := range []struct{ path, dir, filename string }{
		{"services/kube-prometheus-stack/grafana-admin-password.yaml", "kube-prometheus-stack", GrafanaAdminPasswordSecretFilename},
		{"services/kube-prometheus-stack/grafana-oauth-secret.yaml", "kube-prometheus-stack", GrafanaOAuthSecretFilename},
		{"services/oidc-security-policies/gateway-oidc-secret.yaml", "oidc-security-policies", GatewayOIDCSecretFilename},
	} {
		dir, filename, ok := DedicatedSecretFilename(tc.path)
		require.True(t, ok, tc.path)
		require.Equal(t, tc.dir, dir)
		require.Equal(t, tc.filename, filename)
		require.True(t, SafeArtifactPath(tc.path), tc.path+" must be a sync-ownable path")
	}
}

func TestDedicatedSecretFilenameRejectsOtherPaths(t *testing.T) {
	for _, path := range []string{
		"services/gateway/gateway.yaml",
		"infrastructure/clusters/kit-kat/secrets.yaml",
		"services/../gateway-oidc-secret.yaml",
		"managed-services/oidc-security-policies/gateway-oidc-secret.yaml",
		"services/a/b/gateway-oidc-secret.yaml",
		// The directory is part of the identity: the same filename in another
		// service's directory is not the rendered manifest.
		"services/keycloak/gateway-oidc-secret.yaml",
		"services/gateway/grafana-admin-password.yaml",
	} {
		_, _, ok := DedicatedSecretFilename(path)
		require.False(t, ok, path)
	}

	// The generic per-service manifest is not a dedicated Secret, but it remains
	// sync-ownable through its own shape.
	generic := "services/kube-prometheus-stack/secret.yaml"
	_, _, ok := DedicatedSecretFilename(generic)
	require.False(t, ok)
	require.True(t, SafeArtifactPath(generic))
}

// A placeholder credential must not be materialized: the renderer omits the
// manifest too, so planning one would create an orphan and commit a stub.
func TestPlanOmitsDedicatedSecretsWithoutRealCredentials(t *testing.T) {
	t.Run("grafana password unset", func(t *testing.T) {
		cfg := dedicatedConfig()
		cfg.Secrets.Grafana.AdminPassword = ""
		byPath := artifactPaths(mustPlan(t, cfg))
		_, ok := byPath["services/kube-prometheus-stack/"+GrafanaAdminPasswordSecretFilename]
		require.False(t, ok, "an empty admin password must not produce a Secret Flux reconciles over the bootstrap one")
	})

	t.Run("oidc client secret placeholder", func(t *testing.T) {
		cfg := dedicatedConfig()
		cfg.Secrets.Keycloak.ClientSecret = v2.PlaceholderSecret
		byPath := artifactPaths(mustPlan(t, cfg))
		_, ok := byPath["services/oidc-security-policies/"+GatewayOIDCSecretFilename]
		require.False(t, ok)
		_, ok = byPath["services/kube-prometheus-stack/"+GrafanaOAuthSecretFilename]
		require.False(t, ok, "both client secrets gate the grafana oauth Secret")
	})

	t.Run("oidc disabled", func(t *testing.T) {
		cfg := dedicatedConfig()
		cfg.OpenCenter.Identity.OIDC.Enabled = false
		byPath := artifactPaths(mustPlan(t, cfg))
		require.NotContains(t, byPath, "services/oidc-security-policies/"+GatewayOIDCSecretFilename)
	})

	t.Run("hosting service disabled", func(t *testing.T) {
		cfg := dedicatedConfig()
		cfg.OpenCenter.Services["kube-prometheus-stack"] = &services.BaseConfig{Enabled: false}
		byPath := artifactPaths(mustPlan(t, cfg))
		require.NotContains(t, byPath, "services/kube-prometheus-stack/"+GrafanaAdminPasswordSecretFilename)
		require.NotContains(t, byPath, "services/kube-prometheus-stack/"+GrafanaOAuthSecretFilename)
		require.NotContains(t, byPath, "services/oidc-security-policies/"+GatewayOIDCSecretFilename)

		cfg = dedicatedConfig()
		cfg.OpenCenter.Services["keycloak"] = &services.BaseConfig{Enabled: false}
		byPath = artifactPaths(mustPlan(t, cfg))
		require.NotContains(t, byPath, "services/oidc-security-policies/"+GatewayOIDCSecretFilename)
		require.Contains(t, byPath, "services/kube-prometheus-stack/"+GrafanaOAuthSecretFilename)
	})
}

// A dedicated Secret is not a service target: services/oidc-security-policies is
// a descriptor layer, so the service-topology check must not reject it.
func TestValidateTargetsAcceptsDedicatedSecrets(t *testing.T) {
	cfg := dedicatedConfig()
	cfg.OpenCenter.Services["no-such-service"] = &services.BaseConfig{Enabled: true}
	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	require.NoError(t, ValidateTargets(cfg, artifacts))
}

// The admin Secret must carry both keys: kube-prometheus-stack's chart requires
// admin-user alongside admin-password, and legacy configs may omit the user.
func TestDedicatedGrafanaAdminSecretDefaultsUser(t *testing.T) {
	cfg := dedicatedConfig()
	cfg.Secrets.Grafana.AdminUser = ""
	byPath := artifactPaths(mustPlan(t, cfg))
	admin := byPath["services/kube-prometheus-stack/"+GrafanaAdminPasswordSecretFilename]
	require.Equal(t, "admin", admin.Payload["admin-user"])
}

func mustPlan(t *testing.T, cfg *v2.Config) []Artifact {
	t.Helper()
	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	return artifacts
}
