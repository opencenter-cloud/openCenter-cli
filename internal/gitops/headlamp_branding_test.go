package gitops

import (
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

const (
	headlampPluginsDir         = "/headlamp/plugins"
	headlampFluxPluginImage    = "ghcr.io/headlamp-k8s/headlamp-plugin-flux"
	headlampBrandingPluginName = "headlamp-branding-plugins"
)

func renderHeadlampValues(t *testing.T, cfg v2.Config) map[string]any {
	t.Helper()
	values, err := templateRenderer(headlampTemplate)(cfg)
	if err != nil {
		t.Fatalf("render Headlamp values: %v", err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal([]byte(values), &decoded); err != nil {
		t.Fatalf("parse Headlamp values: %v\n%s", err, values)
	}
	return decoded
}

func headlampInitContainers(t *testing.T, values map[string]any) []map[string]any {
	t.Helper()
	raw, ok := values["initContainers"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("Headlamp values must declare init containers to populate the plugins dir: %#v", values["initContainers"])
	}
	containers := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		ic, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("init container entry is not a mapping: %#v", item)
		}
		containers = append(containers, ic)
	}
	return containers
}

func initContainerNamed(t *testing.T, values map[string]any, name string) map[string]any {
	t.Helper()
	for _, ic := range headlampInitContainers(t, values) {
		if ic["name"] == name {
			return ic
		}
	}
	t.Fatalf("no init container named %q in %#v", name, headlampInitContainers(t, values))
	return nil
}

func firstVolumeMount(t *testing.T, container map[string]any) map[string]any {
	t.Helper()
	mounts, ok := container["volumeMounts"].([]any)
	if !ok || len(mounts) == 0 {
		t.Fatalf("init container %v needs its own volumeMounts; the chart does not inherit top-level volumeMounts into init containers", container["name"])
	}
	mount, ok := mounts[0].(map[string]any)
	if !ok {
		t.Fatalf("volumeMount entry is not a mapping: %#v", mounts[0])
	}
	return mount
}

// TestRenderHeadlampPluginsAreAdditive pins the delivery contract: the upstream
// Flux UI plugin and the OpenCenter branding plugin both populate the same
// plugins volume, neither replacing the other.
func TestRenderHeadlampPluginsAreAdditive(t *testing.T) {
	values := renderHeadlampValues(t, newDefault("headlamp-additive"))

	flux := initContainerNamed(t, values, "headlamp-plugins")
	if image, _ := flux["image"].(string); !strings.HasPrefix(image, headlampFluxPluginImage) {
		t.Errorf("Flux UI plugin init container image = %q, want it to remain on %s", image, headlampFluxPluginImage)
	}
	branding := initContainerNamed(t, values, headlampBrandingPluginName)
	if image, _ := branding["image"].(string); !strings.Contains(image, "opencenter-headlamp-plugin-branding") {
		t.Errorf("branding init container image = %q, want the OpenCenter branding plugin", image)
	}
	if got := len(headlampInitContainers(t, values)); got != 2 {
		t.Errorf("init container count = %d, want 2 (Flux UI plus branding)", got)
	}

	// Same destination, distinct plugin subdirectories: the upstream image ships
	// /plugins/flux and the branding image ships /plugins/branding, so the two
	// copies land side by side instead of overwriting each other.
	fluxMount := firstVolumeMount(t, flux)
	brandingMount := firstVolumeMount(t, branding)
	for name, mount := range map[string]map[string]any{"flux": fluxMount, "branding": brandingMount} {
		if mount["mountPath"] != headlampPluginsDir {
			t.Errorf("%s init container mountPath = %v, want %s to match config.pluginsDir", name, mount["mountPath"], headlampPluginsDir)
		}
	}
	if fluxMount["name"] != brandingMount["name"] {
		t.Errorf("both plugin init containers must share one volume: flux=%v branding=%v", fluxMount["name"], brandingMount["name"])
	}
}

// TestRenderHeadlampPluginsAreLoadableByHeadlamp guards the ways a plugin can
// silently fail to load in the browser. The chart templates .Values.initContainers
// verbatim and applies top-level volumeMounts only to the main container, so every
// init container needs its own mount; the plugins emptyDir is provisioned
// root-owned while the branding container runs as gid 101, so fsGroup is what
// makes the copy writable without depending on container order; and Headlamp only
// reads the directory named by config.pluginsDir.
func TestRenderHeadlampPluginsAreLoadableByHeadlamp(t *testing.T) {
	values := renderHeadlampValues(t, newDefault("headlamp-branding"))

	if got := values["config"].(map[string]any)["pluginsDir"]; got != headlampPluginsDir {
		t.Errorf("config.pluginsDir = %v, want %s (Headlamp loads <pluginsDir>/*/main.js)", got, headlampPluginsDir)
	}

	for _, ic := range headlampInitContainers(t, values) {
		if got := firstVolumeMount(t, ic)["mountPath"]; got != headlampPluginsDir {
			t.Errorf("init container %v mountPath = %v, want %s", ic["name"], got, headlampPluginsDir)
		}
	}

	mainMounts, ok := values["volumeMounts"].([]any)
	if !ok || len(mainMounts) == 0 || mainMounts[0].(map[string]any)["mountPath"] != headlampPluginsDir {
		t.Fatalf("main container must mount %s: %#v", headlampPluginsDir, values["volumeMounts"])
	}
	sharedName := firstVolumeMount(t, initContainerNamed(t, values, headlampBrandingPluginName))["name"]
	if mainMounts[0].(map[string]any)["name"] != sharedName {
		t.Errorf("init and main containers must share one volume: %v vs %v", sharedName, mainMounts[0].(map[string]any)["name"])
	}
	volumes, ok := values["volumes"].([]any)
	if !ok {
		t.Fatal("plugins volume must be declared")
	}
	var found bool
	for _, raw := range volumes {
		v := raw.(map[string]any)
		if v["name"] == sharedName {
			found = true
			if _, isEmptyDir := v["emptyDir"]; !isEmptyDir {
				t.Errorf("plugins volume %v must be an emptyDir: %#v", sharedName, v)
			}
		}
	}
	if !found {
		t.Errorf("volume %v referenced by both containers is not declared", sharedName)
	}

	// The branding container runs as the Headlamp container's uid/gid, so it can
	// only write the emptyDir when fsGroup grants the group write bit. Relying on
	// the root Flux container to chown the volume instead would tie branding to
	// init-container order and would break a branding-only deployment.
	podSC, ok := values["podSecurityContext"].(map[string]any)
	if !ok || podSC["fsGroup"] != 101 {
		t.Errorf("podSecurityContext.fsGroup = %#v, want 101 (gid of the headlamp container); without it the branding copy fails with Permission denied unless the Flux container runs first", values["podSecurityContext"])
	}
	brandingSC, ok := initContainerNamed(t, values, headlampBrandingPluginName)["securityContext"].(map[string]any)
	if !ok || brandingSC["runAsNonRoot"] != true {
		t.Errorf("branding init container must stay non-root: %#v", initContainerNamed(t, values, headlampBrandingPluginName)["securityContext"])
	}
	if brandingSC["runAsUser"] == 0 || brandingSC["runAsGroup"] == 0 {
		t.Errorf("branding init container must not run as uid/gid 0: %#v", brandingSC)
	}
}

func TestRenderHeadlampBrandingImageIsDigestPinnedAndOverridable(t *testing.T) {
	cfg := newDefault("headlamp-branding-image")
	values := renderHeadlampValues(t, cfg)

	image, _ := initContainerNamed(t, values, headlampBrandingPluginName)["image"].(string)
	if image != configservices.DefaultBrandingImage {
		t.Errorf("branding image = %q, want the built-in pin %q", image, configservices.DefaultBrandingImage)
	}
	if !strings.Contains(image, "@sha256:") {
		t.Errorf("branding image %q must be pinned by digest; a tag breaks rollback and Helm drift detection", image)
	}
	if strings.HasSuffix(image, ":latest") {
		t.Errorf("branding image %q must not track :latest", image)
	}

	const mirrored = "harbor.example.internal/mirror/opencenter-headlamp-plugin-branding@sha256:0000000000000000000000000000000000000000000000000000000000000000"
	hc, ok := cfg.OpenCenter.Services["headlamp"].(*configservices.HeadlampConfig)
	if !ok {
		t.Fatalf("default headlamp service config = %T", cfg.OpenCenter.Services["headlamp"])
	}
	hc.BrandingImage = mirrored
	cfg.OpenCenter.Services["headlamp"] = hc

	overridden := initContainerNamed(t, renderHeadlampValues(t, cfg), headlampBrandingPluginName)
	if got, _ := overridden["image"].(string); got != mirrored {
		t.Errorf("services.headlamp.branding_image override = %q, want %q", got, mirrored)
	}
	// The override must not disturb the Flux UI plugin.
	if got, _ := initContainerNamed(t, renderHeadlampValues(t, cfg), "headlamp-plugins")["image"].(string); !strings.HasPrefix(got, headlampFluxPluginImage) {
		t.Errorf("branding_image override changed the Flux UI plugin image to %q", got)
	}
}

// TestRenderHeadlampPluginsDirMatchesChartDefault guards against re-introducing a
// platform-specific plugins path: the base chart already defaults to
// /headlamp/plugins, and both plugin images lay out /plugins/<name> for that
// destination.
func TestRenderHeadlampPluginsDirMatchesChartDefault(t *testing.T) {
	out, err := templateRenderer(headlampTemplate)(newDefault("headlamp-plugins-dir"))
	if err != nil {
		t.Fatalf("render Headlamp values: %v", err)
	}
	if strings.Contains(out, "/build/plugins") {
		t.Error("Headlamp override values must not override pluginsDir away from the chart default /headlamp/plugins")
	}
}

// TestRenderHeadlampForcesBrandedTheme pins that the branding theme is enforced
// rather than merely offered. Headlamp renders /settings/general to an anonymous
// visitor and gates nothing in it on authentication, so the theme a visitor picks
// is a localStorage preference that no amount of page-hiding constrains.
// -force-theme is the only mechanism that actually binds the branding.
func TestRenderHeadlampForcesBrandedTheme(t *testing.T) {
	values := renderHeadlampValues(t, newDefault("headlamp-force-theme"))

	cfg, ok := values["config"].(map[string]any)
	if !ok {
		t.Fatalf("config block missing: %#v", values["config"])
	}
	raw, ok := cfg["extraArgs"].([]any)
	if !ok {
		t.Fatalf("config.extraArgs must carry -force-theme; without it any visitor can pick a non-branded theme: %#v", cfg["extraArgs"])
	}

	var forced []string
	for _, item := range raw {
		arg, _ := item.(string)
		if strings.HasPrefix(arg, "-force-theme") {
			forced = append(forced, arg)
		}
	}
	if len(forced) != 1 {
		t.Fatalf("expected exactly one -force-theme arg, got %v in %#v", forced, raw)
	}
	// The name must match a theme the branding plugin registers; "Light" shadows
	// the built-in of the same name, so a typo here silently falls back to it.
	if forced[0] != "-force-theme=Light" {
		t.Errorf("-force-theme = %q, want %q", forced[0], "-force-theme=Light")
	}

	// The backend logs default-light-theme/default-dark-theme as redundant when
	// force-theme is set, and they would be ignored anyway.
	for _, item := range raw {
		arg, _ := item.(string)
		if strings.Contains(arg, "-default-light-theme") || strings.Contains(arg, "-default-dark-theme") {
			t.Errorf("%q is ignored while -force-theme is set; drop it", arg)
		}
	}
}
