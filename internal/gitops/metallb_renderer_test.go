package gitops

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func metallbTestConfig(service *services.MetalLBConfig) v2.Config {
	return v2.Config{OpenCenter: v2.OpenCenterConfig{
		Services: v2.ServiceMap{"metallb": service},
	}}
}

func TestMetalLBOverlayFilesRenderer(t *testing.T) {
	cfg := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "metallb-system"},
		IPAddressPools: []services.IPAddressPool{
			{Name: "public-pool", Addresses: []string{"72.4.119.48/28"}},
			{Name: "private-pool", Addresses: []string{"10.97.6.61/32"}},
		},
		L2Advertisements: []services.L2Advertisement{{
			Name:           "public-pool-l2",
			IPAddressPools: []string{"public-pool"},
			Interfaces:     []string{"metal.105"},
		}},
	})

	files, err := metallbOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"ipaddresspool.yaml", "l2advertisement.yaml"}, mapKeys(files))
	require.Contains(t, files["ipaddresspool.yaml"], "kind: IPAddressPool")
	require.Contains(t, files["ipaddresspool.yaml"], "name: public-pool")
	require.Contains(t, files["ipaddresspool.yaml"], "addresses:")
	require.Contains(t, files["l2advertisement.yaml"], "kind: L2Advertisement")
	require.Contains(t, files["l2advertisement.yaml"], "ipAddressPools:")
	require.Contains(t, files["l2advertisement.yaml"], "metal.105")
}

func TestMetalLBOverlayFilesRendererConditionalFiles(t *testing.T) {
	poolsOnly := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{{Name: "pool", Addresses: []string{"10.0.0.1/32"}}},
	})
	files, err := metallbOverlayFilesRenderer(poolsOnly)
	require.NoError(t, err)
	require.Equal(t, []string{"ipaddresspool.yaml"}, mapKeys(files))

	empty := metallbTestConfig(&services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true}})
	files, err = metallbOverlayFilesRenderer(empty)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestMetalLBOverlayFilesRendererRejectsInvalidConfig(t *testing.T) {
	cfg := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{{Name: "bad_name", Addresses: []string{"10.0.0.2-10.0.0.1"}}},
	})
	_, err := metallbOverlayFilesRenderer(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad_name")
}

func TestMetalLBOverlayFilesRendererRejectsMultipleDefaults(t *testing.T) {
	cfg := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{
			{Name: "public-pool", Addresses: []string{"72.4.119.48/28"}, Default: true},
			{Name: "private-pool", Addresses: []string{"10.97.6.61/32"}, Default: true},
		},
	})
	_, err := metallbOverlayFilesRenderer(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at most one pool may set default")
}

func TestMetalLBOverlayFilesRendererRejectsUnsupportedAdvertisementType(t *testing.T) {
	cfg := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{{Name: "pool", Addresses: []string{"10.0.0.1/32"}}},
		L2Advertisements: []services.L2Advertisement{
			{Name: "bgp-adv", Type: "bgp", IPAddressPools: []string{"pool"}},
		},
	})
	_, err := metallbOverlayFilesRenderer(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not supported")
}

func TestMetalLBOverlayFilesRendererDefaultAndExplicitL2Type(t *testing.T) {
	cfg := metallbTestConfig(&services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{{Name: "pool", Addresses: []string{"10.0.0.1/32"}}},
		L2Advertisements: []services.L2Advertisement{
			{Name: "implicit-l2", IPAddressPools: []string{"pool"}},
			{Name: "explicit-l2", Type: "l2", IPAddressPools: []string{"pool"}},
		},
	})
	files, err := metallbOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, files["l2advertisement.yaml"], "name: implicit-l2")
	require.Contains(t, files["l2advertisement.yaml"], "name: explicit-l2")
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
