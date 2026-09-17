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
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func gatewayTLSTestConfig(gw *services.GatewayConfig) v2.Config {
	return v2.Config{OpenCenter: v2.OpenCenterConfig{
		Cluster:  v2.ClusterConfig{ClusterFQDN: "rackai-dev.rax.io"},
		Services: v2.ServiceMap{"gateway": gw},
	}}
}

func TestGatewayDefaultTLSKeepsCertManagerAnnotation(t *testing.T) {
	cfg := gatewayTLSTestConfig(&services.GatewayConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	})
	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	gw := files["gateway.yaml"]
	require.Contains(t, gw, "cert-manager.io/cluster-issuer: rackspace-ca")
	require.Contains(t, gw, "name: keycloak-tls")
	require.Contains(t, gw, "name: harbor-tls")
	require.Contains(t, gw, "name: longhorn-tls")
}

func TestGatewayWildcardTLSDropsCertManagerAndUsesOneSecret(t *testing.T) {
	cfg := gatewayTLSTestConfig(&services.GatewayConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
		TLS: &services.GatewayTLSConfig{
			WildcardSecretName: "star-rax-io-tls",
			SecretNamespace:    "rackspace-system",
		},
	})
	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	gw := files["gateway.yaml"]

	require.NotContains(t, gw, "cert-manager.io/cluster-issuer")
	// Every HTTPS listener terminates with the wildcard secret.
	require.Equal(t, 8, strings.Count(gw, "name: star-rax-io-tls"),
		"expected all HTTPS listeners to use the wildcard secret")
	require.Contains(t, gw, "namespace: rackspace-system")
	// No hardcoded per-service leaf secret survives.
	require.NotContains(t, gw, "name: keycloak-tls")
	require.NotContains(t, gw, "name: harbor-tls")
}

func TestGatewayPerListenerBYOSecretOverride(t *testing.T) {
	cfg := gatewayTLSTestConfig(&services.GatewayConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
		TLS: &services.GatewayTLSConfig{
			PerListenerSecrets: map[string]string{
				"harbor": "harbor-byo-tls",
			},
		},
	})
	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	gw := files["gateway.yaml"]

	// BYO set -> cert-manager annotation dropped.
	require.NotContains(t, gw, "cert-manager.io/cluster-issuer")
	// Overridden listener uses its BYO secret.
	require.Contains(t, gw, "name: harbor-byo-tls")
	require.NotContains(t, gw, "name: harbor-tls")
	// Non-overridden listeners keep their built-in default secret.
	require.Contains(t, gw, "name: keycloak-tls")
	require.Contains(t, gw, "name: longhorn-tls")
}
