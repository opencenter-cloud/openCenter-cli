#!/usr/bin/env bash
# openCenter CLI and Plugin Installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/opencenter-cloud/openCenter-cli/main/scripts/install.sh | bash
#
# Options (via flags or environment variables):
#   -v, --version <ver>      Install a specific version (e.g. v1.0.0-rc07)
#   -b, --bin-dir <dir>      Directory to install binaries (default: /usr/local/bin or ~/.local/bin)
#   -p, --plugins-dir <dir>  Directory to install plugins (default: ~/.config/opencenter/plugins)
#   --no-checksum            Skip SHA-256 checksum verification
#   --dry-run                Show actions without downloading or installing
#   -h, --help               Display help message
#
# Environment variables:
#   OPENCENTER_VERSION       Same as --version
#   BIN_DIR / INSTALL_DIR    Same as --bin-dir
#   OPENCENTER_PLUGINS_DIR   Same as --plugins-dir
#   VERIFY_CHECKSUM          Set to "false" to skip checksum verification
#   DRY_RUN                  Set to "true" for dry run

set -euo pipefail

REPO="opencenter-cloud/openCenter-cli"
DEFAULT_FALLBACK_VERSION="v1.0.0-rc07"

VERSION="${OPENCENTER_VERSION:-}"
BIN_DIR="${BIN_DIR:-${INSTALL_DIR:-}}"
PLUGINS_DIR="${OPENCENTER_PLUGINS_DIR:-${OPENCENTER_CONFIG_DIR:-$HOME/.config/opencenter}/plugins}"
VERIFY_CHECKSUM="${VERIFY_CHECKSUM:-true}"
DRY_RUN="${DRY_RUN:-false}"

print_usage() {
  cat << EOF
openCenter CLI & Plugin Installer

Usage:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash [options]
  ./scripts/install.sh [options]

Options:
  -v, --version <version>    Version to install (e.g., v1.0.0-rc07). Default: latest
  -b, --bin-dir <dir>        Directory for the CLI binary (default: /usr/local/bin as root, else ~/.local/bin)
  -p, --plugins-dir <dir>    Directory to install plugins (default: ~/.config/opencenter/plugins)
  --no-checksum              Skip checksum verification
  --dry-run                  Print what would be done without making changes
  -h, --help                 Show this help message

Environment Variables:
  OPENCENTER_VERSION         Target version
  BIN_DIR / INSTALL_DIR      CLI installation path
  OPENCENTER_PLUGINS_DIR     Plugin installation path
  VERIFY_CHECKSUM            Set to "false" to skip verification
  DRY_RUN                    Set to "true" to simulate installation
EOF
}

# require_option_value <option-name> <argc-remaining>
# Guards value-taking options: under `set -u`, "$2" at the end of the argument
# list dies with an "unbound variable" error instead of the usage message.
require_option_value() {
  if [[ "${2}" -lt 2 ]]; then
    echo "Error: ${1} requires a value" >&2
    print_usage >&2
    exit 1
  fi
}

# Parse CLI arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--version)
      require_option_value "$1" "$#"
      VERSION="$2"
      shift 2
      ;;
    -b|--bin-dir)
      require_option_value "$1" "$#"
      BIN_DIR="$2"
      shift 2
      ;;
    -p|--plugins-dir)
      require_option_value "$1" "$#"
      PLUGINS_DIR="$2"
      shift 2
      ;;
    --no-checksum)
      VERIFY_CHECKSUM="false"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

# Detect OS
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *)
    echo "Error: Unsupported operating system: ${OS_RAW}. openCenter currently supports Linux and macOS (Darwin)." >&2
    exit 1
    ;;
esac

# Detect Architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture: ${ARCH_RAW}. openCenter currently supports amd64 (x86_64) and arm64 (aarch64)." >&2
    exit 1
    ;;
esac

# Resolve version
if [[ -z "${VERSION}" ]]; then
  echo "==> Resolving latest release for ${REPO}..."
  # Try latest non-prerelease first
  LATEST_JSON="$(curl -sSfL -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
  RESOLVED_TAG="$(echo "${LATEST_JSON}" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/' || true)"

  # If no latest non-prerelease (e.g. all releases are prereleases or rate-limited), query releases list
  if [[ -z "${RESOLVED_TAG}" ]]; then
    RELEASES_JSON="$(curl -sSfL -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/${REPO}/releases" 2>/dev/null || true)"
    RESOLVED_TAG="$(echo "${RELEASES_JSON}" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/' || true)"
  fi

  if [[ -n "${RESOLVED_TAG}" ]]; then
    VERSION="${RESOLVED_TAG}"
    echo "==> Found release ${VERSION}"
  else
    VERSION="${DEFAULT_FALLBACK_VERSION}"
    echo "==> Could not query GitHub API (rate-limited or offline); using default ${VERSION}"
  fi
fi

# Standardize tag and version number
VERSION_NUM="${VERSION#v}"
TAG="v${VERSION_NUM}"

CLI_ASSET="opencenter-${VERSION_NUM}-${OS}-${ARCH}"
PLUGIN_ASSET="opencenter-local-${VERSION_NUM}-${OS}-${ARCH}"
CHECKSUMS_ASSET="checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

# Resolve the release tag: GitHub releases in this repository have been tagged
# both with and without the leading "v" (v1.0.0-rc07 vs 1.0.0-rc06). Probe the
# checksums manifest, which every release publishes, and fall back to the
# unprefixed tag. A manifest that resolves under neither spelling means the
# version does not exist, so fail here rather than printing a plan -- or
# starting a download -- for assets that are not there.
if ! curl -sSfI "${BASE_URL}/${CHECKSUMS_ASSET}" >/dev/null 2>&1; then
  if curl -sSfI "https://github.com/${REPO}/releases/download/${VERSION_NUM}/${CHECKSUMS_ASSET}" >/dev/null 2>&1; then
    TAG="${VERSION_NUM}"
    BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
  else
    echo "Error: release ${VERSION} not found for ${REPO}." >&2
    echo "       No ${CHECKSUMS_ASSET} at ${BASE_URL}/ or at the unprefixed tag." >&2
    echo "       List published releases: https://github.com/${REPO}/releases" >&2
    exit 1
  fi
fi

# Determine default binary destination directory. Prefer the user-local bin
# directory: /usr/local/bin is group-writable in many container and CI images,
# so a mere writability test would silently install to a system path without
# elevation. Only a real root install targets the system directory.
if [[ -z "${BIN_DIR}" ]]; then
  if [[ "${EUID}" -eq 0 ]]; then
    BIN_DIR="/usr/local/bin"
    echo "==> Running as root; installing to the system directory ${BIN_DIR}."
  else
    BIN_DIR="${HOME}/.local/bin"
    echo "==> Installing to the user directory ${BIN_DIR} (pass --bin-dir to override)."
  fi
fi

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "==> Dry run mode enabled. Planned actions:"
  echo "    Target Release:  ${TAG} (${VERSION_NUM})"
  echo "    Platform:        ${OS}/${ARCH}"
  echo "    CLI Asset:       ${CLI_ASSET}"
  echo "    Plugin Asset:    ${PLUGIN_ASSET}"
  echo "    Install CLI to:  ${BIN_DIR}/opencenter"
  echo "    Install Plugin:  ${PLUGINS_DIR}/opencenter-local"
  echo "    Plugins Dir:     ${PLUGINS_DIR}"
  echo "    Verify SHA-256:  ${VERIFY_CHECKSUM}"
  exit 0
fi

echo "==> Installing openCenter CLI and plugin (${TAG}) for ${OS}/${ARCH}..."
echo "    CLI Destination:     ${BIN_DIR}/opencenter"
echo "    Plugin Destination:  ${PLUGINS_DIR}/opencenter-local"

TMP_DIR="$(mktemp -d -t opencenter-install.XXXXXX)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# Fetch the release checksum manifest BEFORE the binaries. The manifest is the
# only trusted source for the plugin hash that gets registered in
# <plugins-dir>/checksums.txt, and the loader treats a matching entry as
# verified (internal/plugins/loader.go). Hashing the freshly downloaded file
# and writing that value back would be self-attestation: the download would
# certify itself, and a tampered asset would pass.
RELEASE_CHECKSUMS=""
if curl -fsSL "${BASE_URL}/${CHECKSUMS_ASSET}" -o "${TMP_DIR}/${CHECKSUMS_ASSET}" 2>/dev/null; then
  RELEASE_CHECKSUMS="${TMP_DIR}/${CHECKSUMS_ASSET}"
else
  echo "Warning: ${CHECKSUMS_ASSET} not found on release; checksum verification and plugin registration are disabled." >&2
fi

echo "==> Downloading assets from ${BASE_URL}..."
curl -fSL --progress-bar "${BASE_URL}/${CLI_ASSET}" -o "${TMP_DIR}/${CLI_ASSET}"
curl -fSL --progress-bar "${BASE_URL}/${PLUGIN_ASSET}" -o "${TMP_DIR}/${PLUGIN_ASSET}"

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${1}" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${1}" | awk '{print $1}'
  fi
}

if [[ "${VERIFY_CHECKSUM}" == "true" ]]; then
  if [[ -n "${RELEASE_CHECKSUMS}" ]]; then
    echo "==> Verifying SHA-256 checksums against the release manifest..."
    (
      cd "${TMP_DIR}"
      grep -E "(${CLI_ASSET}|${PLUGIN_ASSET})\$" "${CHECKSUMS_ASSET}" > release-checksums.txt
      if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -c release-checksums.txt
      elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 -c release-checksums.txt
      else
        echo "Error: neither sha256sum nor shasum is available to verify the download." >&2
        exit 1
      fi
    )
  else
    echo "Error: checksum verification requested but no release manifest is available." >&2
    echo "       Re-run with --no-checksum to install without verification." >&2
    exit 1
  fi
fi

# Ensure destination directories exist
mkdir -p "${BIN_DIR}"
mkdir -p "${PLUGINS_DIR}"

# Install CLI binary
echo "==> Installing ${BIN_DIR}/opencenter..."
cp "${TMP_DIR}/${CLI_ASSET}" "${BIN_DIR}/opencenter"
chmod 0755 "${BIN_DIR}/opencenter"

# Install the plugin to the plugins directory only.
#
# internal/plugins/loader.go discovers plugins from OPENCENTER_PLUGINS_DIR,
# then <config-dir>/plugins, then every PATH entry, and keys the result map by
# basename. Copying the same basename into both PLUGINS_DIR and BIN_DIR puts
# two candidates in scope and lets PATH ordering plus map iteration decide
# which one runs. PLUGINS_DIR is always in the discovery set, so a second copy
# in BIN_DIR adds nondeterminism without adding reachability.
echo "==> Installing plugin ${PLUGINS_DIR}/opencenter-local..."
cp "${TMP_DIR}/${PLUGIN_ASSET}" "${PLUGINS_DIR}/opencenter-local"
chmod 0755 "${PLUGINS_DIR}/opencenter-local"

# Register the plugin's checksum so `opencenter` reports it as verified. The
# value MUST come from the release manifest, never from hashing the file we
# just downloaded: a self-computed hash certifies whatever arrived, including a
# tampered binary, and suppresses the loader's unverified-plugin warning.
PLUGIN_CHECKSUMS_FILE="${PLUGINS_DIR}/checksums.txt"
PLUGIN_SHA=""
if [[ -n "${RELEASE_CHECKSUMS}" ]]; then
  PLUGIN_SHA="$(awk -v asset="${PLUGIN_ASSET}" '$2 == asset || $2 == "*" asset {print $1; exit}' "${RELEASE_CHECKSUMS}")"
  if [[ -z "${PLUGIN_SHA}" ]]; then
    echo "Warning: ${PLUGIN_ASSET} has no entry in the release manifest; the plugin will be reported unverified." >&2
  fi
else
  echo "Note: no release manifest available; skipping plugin checksum registration." >&2
fi

if [[ -n "${PLUGIN_SHA}" ]]; then
  TMP_CHECKSUMS="$(mktemp "${TMP_DIR}/checksums.XXXXXX")"
  if [[ -f "${PLUGIN_CHECKSUMS_FILE}" ]]; then
    grep -v ' opencenter-local$' "${PLUGIN_CHECKSUMS_FILE}" > "${TMP_CHECKSUMS}" 2>/dev/null || true
  fi
  printf '%s  opencenter-local\n' "${PLUGIN_SHA}" >> "${TMP_CHECKSUMS}"
  mv "${TMP_CHECKSUMS}" "${PLUGIN_CHECKSUMS_FILE}"
  chmod 0644 "${PLUGIN_CHECKSUMS_FILE}"
fi

echo ""
echo "============================================================"
echo " openCenter CLI & Plugin successfully installed!"
echo "============================================================"

# Verify installed binary
if command -v "${BIN_DIR}/opencenter" >/dev/null 2>&1; then
  echo "CLI Version: $("${BIN_DIR}/opencenter" version 2>/dev/null || echo "${VERSION_NUM}")"
fi

# PATH notification
case ":${PATH}:" in
  *:"${BIN_DIR}":*) ;;
  *)
    echo ""
    echo "Notice: ${BIN_DIR} is not in your \$PATH."
    echo "Add it to your profile (e.g. ~/.bashrc or ~/.zshrc):"
    echo ""
    echo "    export PATH=\"${BIN_DIR}:\$PATH\""
    echo ""
    ;;
esac

echo "Run 'opencenter --help' or 'opencenter plugins list' to get started."
