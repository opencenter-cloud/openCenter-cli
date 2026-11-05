package v2

import (
	"reflect"
	"sort"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// TestDefaultServiceMapTypesMatchRegistry asserts that every entry in
// defaultServiceMap uses the concrete config type the service registry declares.
//
// ServiceMap.UnmarshalYAML builds services from registry.GetServiceConfigType,
// so when defaultServiceMap hardcodes a different type the same service behaves
// differently depending on how the config was produced: a config loaded from
// YAML gets the registered type, a freshly constructed default gets whatever was
// hardcoded here.
//
// That divergence is not cosmetic. Go templates resolve fields against the
// concrete type, so a template reading a type-specific field (e.g.
// `(index .OpenCenter.Services "longhorn").Hostname`) fails outright when the
// default map supplies DefaultServiceConfig instead:
//
//	can't evaluate field Hostname in type *services.DefaultServiceConfig
//
// longhorn shipped in exactly that state. gateway, metallb, vsphere-csi and
// opentelemetry-kube-stack were mis-wired the same way but had not yet been hit
// because no template referenced their type-specific fields.
func TestDefaultServiceMapTypesMatchRegistry(t *testing.T) {
	serviceMap := defaultServiceMap("cluster.example.com")

	names := make([]string, 0, len(serviceMap))
	for name := range serviceMap {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			actual := concreteType(serviceMap[name])
			expected := registry.GetServiceConfigType(name)

			if expected == nil {
				t.Fatalf("%q is in defaultServiceMap but not registered via "+
					"registry.RegisterServiceConfig; ServiceMap.UnmarshalYAML would decode "+
					"it as DefaultServiceConfig, disagreeing with the default map's %s",
					name, actual.Name())
			}

			if actual != expected {
				t.Errorf("defaultServiceMap wires %q as %s but the registry declares %s.\n"+
					"Loaded configs would get %s while freshly constructed defaults get %s, "+
					"and templates reading %s-specific fields will fail on the default.",
					name, actual.Name(), expected.Name(),
					expected.Name(), actual.Name(), expected.Name())
			}
		})
	}
}

// TestDefaultServiceMapHostnameServicesExposeHostname guards the specific field
// that broke longhorn: services whose overlay templates render a hostname must
// be wired to a type that actually has a Hostname field.
func TestDefaultServiceMapHostnameServicesExposeHostname(t *testing.T) {
	// Services referenced as `(index .OpenCenter.Services "<name>").Hostname` by
	// templates in internal/gitops/overlay_files_renderers.go.
	hostnameServices := []string{
		"harbor",
		"headlamp",
		"keycloak",
		"kube-prometheus-stack",
		"longhorn",
	}

	serviceMap := defaultServiceMap("cluster.example.com")

	for _, name := range hostnameServices {
		t.Run(name, func(t *testing.T) {
			serviceCfg, ok := serviceMap[name]
			if !ok {
				t.Fatalf("%q missing from defaultServiceMap", name)
			}

			typ := concreteType(serviceCfg)
			if _, found := typ.FieldByName("Hostname"); !found {
				t.Fatalf("%q is wired as %s, which has no Hostname field. A Gateway "+
					"listener or HTTPRoute template reads .Hostname for this service and "+
					"will fail template execution.", name, typ.Name())
			}
		})
	}
}

// TestDefaultServiceMapEntriesEmbedBaseConfig ensures every service exposes the
// shared BaseConfig used for public desired-state fields such as Enabled and
// Namespace; rendering metadata is owned by the GitOps render catalog.
func TestDefaultServiceMapEntriesEmbedBaseConfig(t *testing.T) {
	serviceMap := defaultServiceMap("cluster.example.com")

	for name, serviceCfg := range serviceMap {
		typ := concreteType(serviceCfg)
		field, found := typ.FieldByName("BaseConfig")
		if !found {
			t.Errorf("%q (%s) does not embed BaseConfig", name, typ.Name())
			continue
		}
		if field.Type != reflect.TypeOf(services.BaseConfig{}) {
			t.Errorf("%q (%s) embeds %s, expected services.BaseConfig",
				name, typ.Name(), field.Type)
		}
	}
}

// concreteType returns the dereferenced struct type behind a service config.
func concreteType(serviceCfg any) reflect.Type {
	typ := reflect.TypeOf(serviceCfg)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}
