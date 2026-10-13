package gitops

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// An HTTPRoute that names a sectionName attaches to exactly that listener on the
// parent Gateway. If no listener carries the name, the route is applied
// successfully but never attaches: Flux reports the Kustomization as ready while
// the route sits with a NoMatchingParent / unresolved-refs condition, so the
// service is silently unreachable.
//
// The longhorn HTTPRoute shipped pointing at sectionName "longhorn-https" while
// gatewayResourceTemplate defined no such listener. These tests keep generated
// routes and generated listeners in agreement.

type httpRouteParentRef struct {
	file        string
	route       string
	gateway     string // "namespace/name"
	sectionName string
}

func TestGeneratedHTTPRouteSectionNamesResolveToGatewayListeners(t *testing.T) {
	actions := planAllServicesEnabled(t)

	// gateway key -> set of listener names
	listeners := map[string]map[string]bool{}
	var refs []httpRouteParentRef

	for _, action := range actions {
		docs, err := decodeYAMLDocuments([]byte(action.Content))
		if err != nil {
			continue // non-YAML payloads (helm values, etc.)
		}

		for _, doc := range docs {
			switch doc["kind"] {
			case "Gateway":
				key := objectKey(doc)
				if listeners[key] == nil {
					listeners[key] = map[string]bool{}
				}
				for _, name := range gatewayListenerNames(doc) {
					listeners[key][name] = true
				}
			case "HTTPRoute":
				refs = append(refs, httpRouteParentRefs(action.Output, doc)...)
			}
		}
	}

	require.NotEmpty(t, listeners, "no Gateway was generated; this check would pass vacuously")
	require.NotEmpty(t, refs, "no HTTPRoute with a sectionName was generated; "+
		"this check would pass vacuously")

	for _, ref := range refs {
		known, ok := listeners[ref.gateway]
		require.Truef(t, ok, "%s: HTTPRoute %q references Gateway %q, which is not generated. "+
			"Known Gateways: %v", ref.file, ref.route, ref.gateway, sortedKeys(listeners))

		require.Truef(t, known[ref.sectionName],
			"%s: HTTPRoute %q attaches to sectionName %q on Gateway %q, but that Gateway "+
				"defines no such listener. The route will be applied but never attach, leaving "+
				"the service unreachable. Listeners present: %v",
			ref.file, ref.route, ref.sectionName, ref.gateway, sortedSet(known))
	}

	requiredMonitoringRoutes := map[string]string{
		"prometheus-gateway-route":   "prometheus-https",
		"alertmanager-gateway-route": "alertmanager-https",
		"grafana-gateway-route":      "grafana-https",
		"headlamp-gateway-route":     "headlamp-https",
		"weave-gitops-gateway-route": "gitops-https",
	}
	seenMonitoringRoutes := make(map[string]string, len(requiredMonitoringRoutes))
	for _, ref := range refs {
		if _, required := requiredMonitoringRoutes[ref.route]; required {
			seenMonitoringRoutes[ref.route] = ref.sectionName
		}
	}
	for route, sectionName := range requiredMonitoringRoutes {
		require.Equalf(t, sectionName, seenMonitoringRoutes[route],
			"Gateway listener %q must have its required monitoring HTTPRoute %q", sectionName, route)
	}

	t.Logf("checked %d HTTPRoute parentRef(s) against %d Gateway(s)", len(refs), len(listeners))
}

// TestLonghornListenerHostnameMatchesRoute pins that the listener added for
// longhorn and the longhorn HTTPRoute agree on the hostname. A Gateway listener
// only accepts routes whose hostnames intersect its own, so a mismatch fails to
// attach just as a missing listener does.
func TestLonghornListenerHostnameMatchesRoute(t *testing.T) {
	const customHostname = "storage.example.com"
	cfg := enableLonghorn(t, customHostname)

	gatewayFiles, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	routeFiles, err := longhornOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	gatewayDocs, err := decodeYAMLDocuments([]byte(gatewayFiles["gateway.yaml"]))
	require.NoError(t, err)

	var listenerHostname string
	for _, doc := range gatewayDocs {
		if doc["kind"] != "Gateway" {
			continue
		}
		for _, listener := range gatewayListeners(doc) {
			if listener["name"] == "longhorn-https" {
				listenerHostname, _ = listener["hostname"].(string)
			}
		}
	}

	require.Equal(t, customHostname, listenerHostname,
		"the longhorn-https listener must use the configured longhorn hostname")
	require.Contains(t, routeFiles["longhorn-http-route.yaml"], `"`+customHostname+`"`,
		"the longhorn HTTPRoute must use the same hostname as its listener")
}

// planAllServicesEnabled plans the full cluster-apps render with every default
// service turned on, so optional HTTPRoutes are included.
func planAllServicesEnabled(t *testing.T) []clusterAppAction {
	t.Helper()

	cfg, err := v2.NewV2Default("k8s-routes", "openstack")
	require.NoError(t, err)
	cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig).S3Endpoint = "https://harbor-s3.example"

	for serviceName, serviceCfg := range cfg.OpenCenter.Services {
		// OpenStack CCM owns LoadBalancer Services for this fixture. MetalLB is
		// mutually exclusive with it and is not needed to exercise route wiring.
		if serviceName == "metallb" {
			continue
		}
		if base := extractBaseConfig(serviceCfg); base != nil {
			base.Enabled = true
		}
	}

	actions, err := planClusterAppActions(*cfg)
	require.NoError(t, err)

	return actions
}

func objectKey(doc map[string]any) string {
	meta, _ := doc["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	return namespace + "/" + name
}

func gatewayListeners(doc map[string]any) []map[string]any {
	spec, _ := doc["spec"].(map[string]any)
	raw, _ := spec["listeners"].([]any)

	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		if listener, ok := entry.(map[string]any); ok {
			out = append(out, listener)
		}
	}
	return out
}

func gatewayListenerNames(doc map[string]any) []string {
	var names []string
	for _, listener := range gatewayListeners(doc) {
		if name, ok := listener["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func httpRouteParentRefs(file string, doc map[string]any) []httpRouteParentRef {
	spec, _ := doc["spec"].(map[string]any)
	raw, _ := spec["parentRefs"].([]any)
	routeMeta, _ := doc["metadata"].(map[string]any)
	routeName, _ := routeMeta["name"].(string)
	routeNamespace, _ := routeMeta["namespace"].(string)

	var refs []httpRouteParentRef
	for _, entry := range raw {
		parent, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		section, ok := parent["sectionName"].(string)
		if !ok || section == "" {
			// Without a sectionName the route attaches to any compatible
			// listener, so there is no specific name to verify.
			continue
		}

		name, _ := parent["name"].(string)
		namespace, _ := parent["namespace"].(string)
		if namespace == "" {
			// parentRef namespace defaults to the route's own namespace.
			namespace = routeNamespace
		}

		refs = append(refs, httpRouteParentRef{
			file:        file,
			route:       routeName,
			gateway:     namespace + "/" + name,
			sectionName: section,
		})
	}
	return refs
}

func sortedKeys(m map[string]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSet(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return fmt.Sprintf("[%s]", strings.Join(out, " "))
}
