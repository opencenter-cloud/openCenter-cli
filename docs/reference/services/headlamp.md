---
last_updated: 2026-11-03
id: service-headlamp
title: "Headlamp Dashboard"
sidebar_label: Headlamp
description: Headlamp service configuration, OIDC dependency rule, and catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [dashboard, oidc, headlamp, services]
---
# Headlamp Dashboard

**Purpose:** For platform engineers, operators, documents headlamp service configuration, OIDC dependency rule, and catalog ownership.

> **Evidence:** `internal/config/services/headlamp.go`, `internal/config/v2/defaults.go`, `internal/config/v2/config.go`, `internal/config/services/dependency_validator.go`, `internal/gitops/render_catalog.go`, and `internal/gitops/override_values_renderers.go`.

## Configuration

The generated default is enabled in `headlamp`, with hostname `dashboard.<cluster FQDN>`. `HeadlampConfig` adds `hostname`, `oidc_issuer_url`, `oidc_client_id`, and `branding_image` to `BaseConfig`. `HeadlampSecrets` declares `oidc_client_secret`.

```yaml
opencenter:
  services:
    headlamp:
      enabled: true
      namespace: headlamp
      hostname:
      oidc_issuer_url:
      oidc_client_id:
      branding_image:
      adoption_mode: managed
      address_pool:
```

## Runtime and rendering

The config dependency graph records `keycloak` for Headlamp. A separate `ValidateHeadlampOIDC` check also describes the conditional case: when Headlamp is enabled and either OIDC field is set, `keycloak` must be enabled. The repository therefore contains both an unconditional graph entry and a conditional OIDC-specific check. The catalog uses base path `applications/base/services/headlamp` and a dedicated override-values renderer. No explicit service descriptor is present.

### Plugins

The override-values renderer emits two init containers that both populate one
`emptyDir`, which the Headlamp container mounts at `/headlamp/plugins`;
`config.pluginsDir` points at the same path, and Headlamp loads every
`<pluginsDir>/*/main.js` at startup.

| Init container | Image | Runs as | Ships |
| --- | --- | --- | --- |
| `headlamp-plugins` | `ghcr.io/headlamp-k8s/headlamp-plugin-flux:latest` | uid 0 | `/plugins/flux` |
| `headlamp-branding-plugins` | `branding_image`, digest-pinned | uid 100 / gid 101 | `/plugins/branding` |

The branding plugin is **additive**: the Flux UI plugin is not replaced. The two
coexist because each image lays out a distinct subdirectory, so the copies land
side by side rather than overwriting each other.

Five parts are load-bearing and easy to break independently:

- **Both init containers carry their own `volumeMounts`.** The chart templates
  `.Values.initContainers` verbatim and applies top-level `volumeMounts` only to
  the main container. A mount omitted here leaves that init container with no
  plugins directory at all, failing with `No such file or directory` — before any
  permission concern, and silently for whichever plugin loses the mount.
- **`podSecurityContext.fsGroup: 101` is required.** Kubernetes provisions the
  `emptyDir` root-owned `0755`, and the branding container runs as uid 100 /
  gid 101 to match the Headlamp container, so without `fsGroup` its copy fails
  with `cp: can't create directory '/headlamp/plugins/branding': Permission denied`.
  The Flux container would mask this, because it runs as root and chowns the
  volume — but only when it runs first, so relying on it would tie branding to
  init-container order and break a branding-only deployment.
- **Each plugin directory needs a `package.json` next to `main.js`.** Headlamp reads
  `<pluginsDir>/<plugin>/package.json` and disables the plugin unless
  `devDependencies["@kinvolk/headlamp-plugin"]` coerces to a version satisfying
  `>=0.8.0-alpha.3` (`frontend/src/plugin/index.ts` in `kubernetes-sigs/headlamp`). A
  missing manifest coerces to the empty string, which never satisfies the range. The
  failure is quiet: the init container copies `main.js`, Headlamp lists the plugin and
  serves it with HTTP 200, and never executes it — the UI shows the stock theme plus
  `Warning. Incompatible plugins disabled: (<plugin>)`. An image carrying only `dist/`
  is therefore not a deployable plugin image.
- **`branding_image` is pinned by digest**, defaulting to
  `services.DefaultBrandingImage`. A tag would break rollback and Helm drift
  detection. Point it at a mirrored copy when the cluster cannot reach `ghcr.io`,
  and use the mirror's own digest — the upstream digest will not resolve there.
- **`config.pluginsDir` stays at the chart default.** It previously diverged to
  `/build/plugins`; mounting at the chart default while `pluginsDir` points
  elsewhere populates a directory Headlamp never reads.

### Theme enforcement

Headlamp does not gate its Settings UI on authentication. `/settings/general`
renders in full to an anonymous visitor — theme picker, language, rows-per-table —
and nothing in the chart or in Headlamp hides it: `SettingsButton` returns `null`
only when there is no cluster context, and `Settings.tsx` has no auth check. The
chosen theme is a per-visitor `localStorage` preference, so no gateway rule on
`/settings/*` constrains it either.

`config.extraArgs: ["-force-theme=Dark"]` is what actually binds the branding. The
picker still renders but greys out behind "Theme has been forced by your
administrator" and stops responding. Two constraints:

- `-force-theme` supersedes `-default-light-theme` and `-default-dark-theme`, which
  the backend logs as redundant when it is also set. Configure one strategy.
- The value is a theme *name*. The branding plugin registers `Light` and `Dark`,
  which shadow the built-ins of those names, so a misspelling silently resolves to
  the unbranded built-in instead of failing.

Helm merges lists by replacement, and the gitops base ships `config.extraArgs: []`.
That is harmless only because the `HelmRelease` reads `headlamp-values-base` before
`headlamp-values-override`, so the CLI's rendered values win. Reversing that order
would silently drop the flag.

`Running` pods prove the files were copied, not that the theme took effect. The
plugin directory can look complete — `main.js` present, listed by `GET /plugins`,
served over HTTP 200 — while Headlamp refuses to execute it.

Two checks, in a browser:

1. **Unauthenticated:** the login page must not carry
   `Warning. Incompatible plugins disabled: (<plugin>)`, and `/settings/general`
   must show the theme picker greyed out with the forced-theme notice. Both need no
   credentials. `GET /plugins` must list the plugin, and the served `main.js` must
   match the bundle in the image.
2. **Authenticated:** the branding must be applied — logo, colours, and the
   `Light`/`Dark` entries in the theme list.

## Commands

```bash
opencenter cluster service enable headlamp
opencenter cluster service disable headlamp
opencenter cluster service options headlamp
```
