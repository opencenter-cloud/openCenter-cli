package gitops

import (
	"sort"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// Regression guard for generated FluxCD Kustomization manifests.
//
// The Kustomization CRD does not set x-kubernetes-preserve-unknown-fields, so
// any field we emit under spec that is not part of the schema is rejected by
// the API server. That makes `kubectl apply --dry-run=server` (and Flux's own
// reconciliation) fail on the whole manifest.
//
// A previous bug emitted spec.cluster.name on the two-stage "-base"
// Kustomization, which is not a Kustomization field at all. These tests keep
// that from coming back and catch any other invalid spec field we might add.

const fluxKustomizeAPIGroup = "kustomize.toolkit.fluxcd.io/"

// fluxKustomizationV1SpecFields is the complete set of spec fields accepted by
// kustomize.toolkit.fluxcd.io/v1 Kustomization.
//
// Source: https://fluxcd.io/flux/components/kustomize/api/v1/ (KustomizationSpec).
// If you are adding a field here, confirm it against that reference first --
// the point of this list is to fail when we invent fields that Flux does not
// have.
var fluxKustomizationV1SpecFields = map[string]struct{}{
	"buildMetadata":           {},
	"commonMetadata":          {},
	"components":              {},
	"decryption":              {},
	"deletionPolicy":          {},
	"dependsOn":               {},
	"force":                   {},
	"healthCheckExprs":        {},
	"healthChecks":            {},
	"ignore":                  {},
	"ignoreMissingComponents": {},
	"images":                  {},
	"interval":                {},
	"kubeConfig":              {},
	"namePrefix":              {},
	"nameSuffix":              {},
	"patches":                 {},
	"path":                    {},
	"postBuild":               {},
	"prune":                   {},
	"retryInterval":           {},
	"serviceAccountName":      {},
	"sourceRef":               {},
	"suspend":                 {},
	"targetNamespace":         {},
	"timeout":                 {},
	"wait":                    {},
}

// assertFluxKustomizationSpecFields checks every Flux Kustomization document in
// content and reports how many were inspected. The count lets callers assert
// that the check was not vacuous.
func assertFluxKustomizationSpecFields(t *testing.T, source, content string) int {
	t.Helper()

	docs, err := decodeYAMLDocuments([]byte(content))
	require.NoErrorf(t, err, "%s: rendered output is not valid YAML", source)

	checked := 0
	for i, doc := range docs {
		apiVersion, _ := doc["apiVersion"].(string)
		kind, _ := doc["kind"].(string)
		if kind != "Kustomization" || !strings.HasPrefix(apiVersion, fluxKustomizeAPIGroup) {
			continue
		}
		checked++

		spec, ok := doc["spec"].(map[string]any)
		require.Truef(t, ok, "%s: doc %d (%s) has no spec map", source, i, fluxObjectName(doc))

		var invalid []string
		for field := range spec {
			if _, valid := fluxKustomizationV1SpecFields[field]; !valid {
				invalid = append(invalid, field)
			}
		}
		sort.Strings(invalid)

		require.Emptyf(t, invalid, "%s: Kustomization %q has spec field(s) %v that do not exist in %s. "+
			"Unknown fields are rejected by the Flux CRD and break kubectl apply --dry-run=server. "+
			"Verify against https://fluxcd.io/flux/components/kustomize/api/v1/",
			source, fluxObjectName(doc), invalid, apiVersion)
	}

	return checked
}

func fluxObjectName(doc map[string]any) string {
	meta, ok := doc["metadata"].(map[string]any)
	if !ok {
		return "<unnamed>"
	}
	name, _ := meta["name"].(string)
	if name == "" {
		return "<unnamed>"
	}
	return name
}

// TestAutoDescriptorFluxKustomizationSpecFields renders every auto-descriptor
// template shape and validates the Kustomization spec fields against the CRD.
func TestAutoDescriptorFluxKustomizationSpecFields(t *testing.T) {
	cases := []struct {
		name string
		ctx  autoServiceContext
	}{
		{
			name: "two stage",
			ctx: autoServiceContext{
				ServiceName:       "sealed-secrets",
				Namespace:         "sealed-secrets",
				SourceName:        "opencenter-sealed-secrets",
				BasePath:          "applications/base/services/sealed-secrets",
				HasOverrideValues: true,
			},
		},
		{
			name: "two stage with extra and override dependencies",
			ctx: autoServiceContext{
				ServiceName:       "weave-gitops",
				Namespace:         "flux-system",
				SourceName:        "opencenter-weave-gitops",
				BasePath:          "applications/base/services/weave-gitops",
				HasOverrideValues: true,
				ExtraDependencies: []string{"sources"},
				OverrideDependsOn: []string{"sources", "envoy-gateway-api-base"},
				Force:             true,
				Suspend:           true,
			},
		},
		{
			name: "base only",
			ctx: autoServiceContext{
				ServiceName: "external-snapshotter",
				Namespace:   "external-snapshotter",
				SourceName:  "opencenter-external-snapshotter",
				BasePath:    "applications/base/services/external-snapshotter",
				BaseOnly:    true,
			},
		},
		{
			name: "single stage",
			ctx: autoServiceContext{
				ServiceName:       "gateway",
				Namespace:         "gateway",
				SourceName:        "opencenter-gateway",
				BasePath:          "applications/base/services/gateway",
				SingleStage:       true,
				ExtraDependencies: []string{"envoy-gateway-api-base"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			ctx.ClusterName = "k8s-schema"
			ctx.BaseRepoURL = "https://github.com/rackerlabs/openCenter-gitops-base.git"
			ctx.RepoBranch = "main"
			ctx.GitopsAuthMethod = gitopsAuthMethodToken
			ctx.FluxInterval = "15m"

			actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
			require.NoError(t, err)

			checked := 0
			for _, action := range actions {
				checked += assertFluxKustomizationSpecFields(t, action.Output, action.Content)
			}
			require.NotZerof(t, checked, "no Flux Kustomization documents were rendered for %q; "+
				"the schema check would pass vacuously", tc.name)
		})
	}
}

// TestFluxKustomizationsHaveNoSpecClusterField pins the specific field that
// caused the original dry-run failures.
func TestFluxKustomizationsHaveNoSpecClusterField(t *testing.T) {
	templates := map[string]string{
		"autoFluxTwoStageTemplate":    autoFluxTwoStageTemplate,
		"autoFluxBaseOnlyTemplate":    autoFluxBaseOnlyTemplate,
		"autoFluxSingleStageTemplate": autoFluxSingleStageTemplate,
	}

	for name, tmpl := range templates {
		t.Run(name, func(t *testing.T) {
			// Asserted on a bool rather than with NotContains so the failure
			// message stays readable instead of dumping the whole template.
			require.Falsef(t, strings.Contains(tmpl, "cluster:"),
				"%s emits a spec.cluster field; Kustomization has no such field and the "+
					"Flux CRD rejects it, breaking kubectl apply --dry-run=server. "+
					"Use commonMetadata.labels if the cluster name must appear on the object.", name)
		})
	}
}

// TestPlannedFluxKustomizationSpecFields walks the full `cluster generate`
// planning path -- explicit descriptors plus auto-descriptors -- and validates
// every Flux Kustomization it produces.
//
// The service set comes from v2.NewV2Default rather than a hand-written list, so
// any service added to the default service map is picked up automatically
// instead of silently escaping coverage.
func TestPlannedFluxKustomizationSpecFields(t *testing.T) {
	t.Run("default services", func(t *testing.T) {
		cfg := newDefaultServiceConfig(t)

		checked, files := planAndValidateFluxKustomizations(t, cfg)
		require.NotZero(t, checked, "planClusterAppActions produced no Flux Kustomization "+
			"documents; the schema check would pass vacuously")
		t.Logf("validated %d Flux Kustomization documents across %d planned files", checked, files)
	})

	// Default-disabled services (metallb, sealed-secrets, weave-gitops, ...) are
	// only rendered when turned on, so enable everything to reach their
	// Kustomizations too. Every service in the default map must render; a failure
	// here means a service is mis-wired, not that it should be skipped.
	t.Run("all services enabled", func(t *testing.T) {
		cfg := newDefaultServiceConfig(t)
		useMetalLBTestOwnership(&cfg)

		var enabled []string
		for name, serviceCfg := range cfg.OpenCenter.Services {
			if name == "openstack-ccm" {
				continue
			}
			if base := extractBaseConfig(serviceCfg); base != nil {
				base.Enabled = true
				enabled = append(enabled, name)
			}
		}
		require.Len(t, enabled, len(cfg.OpenCenter.Services)-1,
			"every compatible default service should expose a BaseConfig")

		checked, files := planAndValidateFluxKustomizations(t, cfg)
		require.NotZero(t, checked)
		t.Logf("validated %d Flux Kustomization documents across %d planned files (%d services enabled)",
			checked, files, len(enabled))
	})
}

// newDefaultServiceConfig returns a config carrying the real default service
// map, with GitOps settings filled in so descriptors can render.
func newDefaultServiceConfig(t *testing.T) v2.Config {
	t.Helper()

	cfg, err := v2.NewV2Default("k8s-plan", "openstack")
	require.NoError(t, err)
	require.NotEmpty(t, cfg.OpenCenter.Services, "default service map is empty")
	cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig).S3Endpoint = "https://harbor-s3.example"

	return *cfg
}

func planAndValidateFluxKustomizations(t *testing.T, cfg v2.Config) (checked, files int) {
	t.Helper()

	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	for _, action := range actions {
		if !strings.HasSuffix(action.Output, ".yaml") {
			continue
		}
		checked += assertFluxKustomizationSpecFields(t, action.Output, action.Content)
	}

	return checked, len(actions)
}
