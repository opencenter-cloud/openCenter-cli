// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package secrets

import (
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/stretchr/testify/require"
)

// dedicatedSecretPaths returns each dedicated Secret's overlay-relative path
// through the planner's own matcher, so a test cannot pin a filename the planner
// no longer produces.
func dedicatedSecretPaths(t *testing.T) map[string]string {
	t.Helper()
	cases := map[string]string{
		"kube-prometheus-stack":  secretartifacts.GrafanaAdminPasswordSecretFilename,
		"oidc-security-policies": secretartifacts.GatewayOIDCSecretFilename,
	}
	paths := make(map[string]string, len(cases))
	for dir, filename := range cases {
		relative := "services/" + dir + "/" + filename
		_, matched, ok := secretartifacts.DedicatedSecretFilename(relative)
		require.Truef(t, ok, "planner no longer claims %s; update this test with the renderer's path", relative)
		require.Equal(t, filename, matched)
		paths[relative] = filename
	}
	return paths
}

// A dedicated Secret is named after its own file, because that is the name the
// Helm chart existingSecret / extraSecretMounts and the Envoy SecurityPolicy were
// rendered with. Naming it from the logical owner instead produced
// opencenter-kube-prometheus-stack-secret for BOTH Grafana Secrets: one
// Kubernetes identity declared twice in one Kustomize build, which kustomize
// rejects ("may not add resource with an already registered id"), leaving
// kube-prometheus-stack-override permanently un-applied and every monitoring
// consumer blocked behind it on dependsOn.
func TestGenerateSecretManifestNamesDedicatedSecretsAfterTheirFile(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	for relative, filename := range dedicatedSecretPaths(t) {
		t.Run(relative, func(t *testing.T) {
			// The sync loop passes the logical owner (grafana-admin,
			// grafana-oauth, gateway-oidc) as the service, and creates the file
			// with no existing metadata to preserve. Both facts are load-bearing:
			// the name must come from the path, not either of those.
			manifest := manager.generateSecretManifest("grafana-oauth",
				map[string]interface{}{"client_secret": "s"},
				nil,
				filepath.Join("overlay-root", filepath.FromSlash(relative)))

			metadata, ok := manifest["metadata"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, trimYAMLExtension(filename), metadata["name"])
		})
	}
}

// The two Grafana Secrets share one overlay directory, so a naming rule that
// derives from the directory or the service gives them the same name. Their
// distinct identities are the only thing keeping the kube-prometheus-stack
// overlay buildable.
func TestGenerateSecretManifestGivesGrafanaSecretsDistinctNames(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	admin := manager.generateSecretManifest("grafana-admin",
		map[string]interface{}{"admin_password": "p"}, nil,
		filepath.FromSlash("overlay-root/services/kube-prometheus-stack/"+secretartifacts.GrafanaAdminPasswordSecretFilename))
	oauth := manager.generateSecretManifest("grafana-oauth",
		map[string]interface{}{"client_secret": "s"}, nil,
		filepath.FromSlash("overlay-root/services/kube-prometheus-stack/"+secretartifacts.GrafanaOAuthSecretFilename))

	adminName := manifestName(t, admin)
	oauthName := manifestName(t, oauth)
	require.Equal(t, "grafana-admin-password", adminName)
	require.Equal(t, "grafana-oauth-secret", oauthName)
	require.NotEqual(t, adminName, oauthName)
}

// For a file-named Secret the path is the contract, so an existing name does NOT
// win. This is what makes the corruption self-healing instead of permanent: the
// buggy fallback wrote opencenter-kube-prometheus-stack-secret into both Grafana
// files, and a preserve-first rule re-read that name back out of the file on every
// later sync, --force included, so the overlay could never be repaired from the
// CLI. Other metadata is still preserved -- only the name is re-derived.
func TestGenerateSecretManifestCorrectsExistingDedicatedSecretName(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	for _, existing := range []string{
		"opencenter-kube-prometheus-stack-secret", // what the bug wrote
		"hand-authored", // and any other drift
	} {
		manifest := manager.generateSecretManifest("grafana-oauth",
			map[string]interface{}{"client_secret": "s"},
			map[string]interface{}{"metadata": map[string]interface{}{
				"name": existing,
				"labels": map[string]interface{}{
					"app.kubernetes.io/part-of": "kube-prometheus-stack",
				},
			}},
			filepath.FromSlash("overlay-root/services/kube-prometheus-stack/"+secretartifacts.GrafanaOAuthSecretFilename))

		require.Equalf(t, "grafana-oauth-secret", manifestName(t, manifest),
			"sync must re-derive the name from the path, not keep %q", existing)
		metadata, ok := manifest["metadata"].(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, map[string]interface{}{
			"app.kubernetes.io/part-of": "kube-prometheus-stack",
		}, metadata["labels"], "non-name metadata must survive the rewrite")
	}
}

// The path must be resolved from its services root, not its basename: a
// dedicated filename sitting outside the layer that renders it is not that
// manifest, and claiming its name would name a Secret no consumer reads.
func TestRelativeArtifactPathResolvesServicesRoot(t *testing.T) {
	require.Equal(t,
		"services/kube-prometheus-stack/grafana-oauth-secret.yaml",
		secretartifacts.RelativeArtifactPath("/repo/applications/overlays/c1/services/kube-prometheus-stack/grafana-oauth-secret.yaml"))
	require.Equal(t,
		"services/kube-prometheus-stack/grafana-oauth-secret.yaml",
		secretartifacts.RelativeArtifactPath(filepath.FromSlash("services/kube-prometheus-stack/grafana-oauth-secret.yaml")))
	require.Equal(t,
		"managed-services/alert-proxy/secret.yaml",
		secretartifacts.RelativeArtifactPath("/repo/applications/overlays/c1/managed-services/alert-proxy/secret.yaml"))

	// No services root: returned in slash form so the caller's match fails
	// rather than guessing an overlay location.
	require.Equal(t, "grafana-oauth-secret.yaml",
		secretartifacts.RelativeArtifactPath("grafana-oauth-secret.yaml"))
	// A foreign directory that happens to hold the filename is not the rendered manifest.
	require.Equal(t, "services/keycloak/gateway-oidc-secret.yaml",
		secretartifacts.RelativeArtifactPath("/repo/services/keycloak/gateway-oidc-secret.yaml"))
}

func trimYAMLExtension(name string) string {
	return name[:len(name)-len(".yaml")]
}

func manifestName(t *testing.T, manifest map[string]interface{}) string {
	t.Helper()
	metadata, ok := manifest["metadata"].(map[string]interface{})
	require.True(t, ok)
	name, _ := metadata["name"].(string)
	return name
}
