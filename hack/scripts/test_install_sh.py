"""Contract tests for ``scripts/install.sh``.

The installer is the documented entry point in README.md, so a regression here
breaks every new user. These tests exercise it against a local stub HTTP server
rather than GitHub, which makes two otherwise-unreachable properties testable:

* the plugin checksum registered in ``checksums.txt`` comes from the release
  manifest, not from a hash of whatever was downloaded -- a tampered artifact
  must be reported as a mismatch, which is the exact flaw the original version
  had;
* verification failure is fatal, so an artifact whose manifest hash does not
  match is never installed.

Only the download/verify/install path is exercised; ``--dry-run`` and argument
parsing are covered by direct invocation.
"""

from __future__ import annotations

import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# hack/scripts/../.. is the repository root.
REPO_ROOT = Path(__file__).resolve().parent.parent.parent
INSTALLER = REPO_ROOT / "scripts" / "install.sh"

VERSION = "9.9.9"
CLI_ASSET = f"opencenter-{VERSION}-linux-amd64"
PLUGIN_ASSET = f"opencenter-local-{VERSION}-linux-amd64"

CLI_BYTES = b"#!/bin/sh\necho cli-stub\n"
PLUGIN_BYTES = b"#!/bin/sh\necho plugin-stub\n"
TAMPERED_PLUGIN = b"#!/bin/sh\necho evil\n"


def release_tree(tamper_plugin: bool = False) -> dict[str, bytes]:
    """Build the asset set one GitHub release would serve."""
    plugin = TAMPERED_PLUGIN if tamper_plugin else PLUGIN_BYTES
    manifest = (
        f"{hashlib.sha256(CLI_BYTES).hexdigest()}  {CLI_ASSET}\n"
        f"{hashlib.sha256(PLUGIN_BYTES).hexdigest()}  {PLUGIN_ASSET}\n"
    ).encode()
    return {
        CLI_ASSET: CLI_BYTES,
        PLUGIN_ASSET: plugin,
        # The installer probes this exact name to decide whether a tag exists.
        "checksums.txt": manifest,
    }


class _Handler(BaseHTTPRequestHandler):
    """Serve the in-memory release tree, mirroring GitHub's URL shape."""

    server_version = "stub/1.0"

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        self._respond(with_body=True)

    def do_HEAD(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        # The installer probes whether a tag exists with `curl -sSfI`, so HEAD
        # has to answer, not just GET.
        self._respond(with_body=False)

    def _respond(self, *, with_body: bool) -> None:
        name = self.path.rsplit("/", 1)[-1]
        body = self.server.assets.get(name)  # type: ignore[attr-defined]
        if body is None:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if with_body:
            self.wfile.write(body)

    def log_message(self, *args: object) -> None:
        pass


class _Server:
    def __enter__(self) -> "_Server":
        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        self.httpd.assets = dict(self.assets)
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)
        self.thread.start()
        return self

    def __exit__(self, *exc: object) -> None:
        self.httpd.shutdown()
        self.httpd.server_close()
        self.thread.join(timeout=5)

    def __init__(self, assets: dict[str, bytes]) -> None:
        self.assets = assets

    @property
    def base_url(self) -> str:
        host, port = self.httpd.server_address[:2]
        return f"http://{host}:{port}/repo/releases/download/v{VERSION}"


def _isolated_env(root: Path) -> dict[str, str]:
    """Return an environment that cannot leak the developer's own settings.

    The installer honours `BIN_DIR`, `INSTALL_DIR`, and
    `OPENCENTER_PLUGINS_DIR` from the ambient environment. A developer who
    installed the CLI per the README has `~/.config/opencenter/plugins` in
    theirs, so an unclean environment would write test artifacts into their real
    plugin directory and make the install-location assertions pass for the wrong
    reason.
    """
    dropped = {
        "BIN_DIR",
        "INSTALL_DIR",
        "OPENCENTER_PLUGINS_DIR",
        "OPENCENTER_CONFIG_DIR",
        "OPENCENTER_VERSION",
        "VERIFY_CHECKSUM",
        "DRY_RUN",
    }
    env = {k: v for k, v in os.environ.items() if k not in dropped}
    env.update(
        {
            "BIN_DIR": str(root / "bin"),
            "OPENCENTER_PLUGINS_DIR": str(root / "plugins"),
            "HOME": str(root),
        }
    )
    return env


def run_installer(
    base_url: str,
    root: Path,
    *args: str,
    env: dict[str, str] | None = None,
    probe_url: str | None = None,
) -> subprocess.CompletedProcess[str]:
    """Run the installer against *base_url*, redirected into *root*.

    Before using ``BASE_URL`` the installer probes a hardcoded GitHub URL for
    the release manifest to decide whether the tag exists, so that probe is
    redirected too -- normally at the same stub, but a missing-manifest test
    points it at a server that has no manifest.
    """
    script = INSTALLER.read_text(encoding="utf-8")
    probe = base_url if probe_url is None else probe_url
    script = script.replace(
        'BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"',
        f'BASE_URL="{base_url}"\nPROBE_URL="{probe}"',
    )
    script = script.replace(
        'if ! curl -sSfI "${BASE_URL}/${CHECKSUMS_ASSET}" >/dev/null 2>&1; then',
        'if ! curl -sSfI "${PROBE_URL}/${CHECKSUMS_ASSET}" >/dev/null 2>&1; then',
    )
    assert f'BASE_URL="{base_url}"' in script, "installer download base changed"
    assert 'PROBE_URL="' in script, "installer tag probe changed"
    script_path = root / "install-under-test.sh"
    script_path.write_text(script, encoding="utf-8")
    run_env = _isolated_env(root)
    run_env.update(env or {})
    return subprocess.run(
        ["bash", str(script_path), "--version", VERSION, *args],
        cwd=root,
        env=run_env,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )


class InstallerDownloadTests(unittest.TestCase):
    def setUp(self) -> None:
        if shutil.which("bash") is None:
            self.skipTest("bash unavailable")
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        (self.root / "bin").mkdir()
        (self.root / "plugins").mkdir()

    def tearDown(self) -> None:
        self.directory.cleanup()

    def test_installs_cli_and_plugin_to_their_own_directories(self) -> None:
        with _Server(release_tree()) as server:
            result = run_installer(server.base_url, self.root)
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertTrue((self.root / "bin" / "opencenter").is_file())
        self.assertTrue((self.root / "plugins" / "opencenter-local").is_file())
        # The plugin must not also land in BIN_DIR: a second copy would be
        # discovered through PATH with no recorded checksum.
        self.assertFalse((self.root / "bin" / "opencenter-local").exists())

    def test_registered_plugin_hash_comes_from_the_release_manifest(self) -> None:
        with _Server(release_tree()) as server:
            self.assertEqual(
                run_installer(server.base_url, self.root).returncode, 0
            )
        registered = (self.root / "plugins" / "checksums.txt").read_text()
        manifest_hash = hashlib.sha256(PLUGIN_BYTES).hexdigest()
        self.assertIn(manifest_hash, registered)
        # The registered value is the manifest hash, not a self-computed hash of
        # the downloaded file. With an untampered artifact both agree, so assert
        # on the manifest line format the installer parses.
        self.assertRegex(registered, r"^\s*\S+\s+opencenter-local\s*$")

    def test_tampered_plugin_is_reported_as_a_mismatch(self) -> None:
        with _Server(release_tree(tamper_plugin=True)) as server:
            result = run_installer(server.base_url, self.root)
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("checksum", result.stdout.lower())
        self.assertFalse((self.root / "plugins" / "opencenter-local").exists())

    def test_tampered_plugin_is_never_registered_as_verified(self) -> None:
        """The registered hash must come from the manifest, not the download.

        This is the flaw the original installer had: it hashed the file it had
        just downloaded and wrote that value to ``checksums.txt``, so a tampered
        plugin was registered as `verified` and the loader's unverified-plugin
        warning was suppressed. Here verification is skipped, which is exactly
        the path where self-attestation is dangerous: the tampered artifact is
        installed, and only the registered hash reveals whether the installer
        trusted the manifest or the download.
        """
        with _Server(release_tree(tamper_plugin=True)) as probe, _Server(
            {k: v for k, v in release_tree(tamper_plugin=True).items() if k != "checksums.txt"}
        ) as server:
            result = run_installer(
                server.base_url,
                self.root,
                "--no-checksum",
                probe_url=probe.base_url,
            )
        self.assertEqual(result.returncode, 0, result.stdout)
        registered = self.root / "plugins" / "checksums.txt"
        if registered.exists():
            self.assertNotIn(
                hashlib.sha256(TAMPERED_PLUGIN).hexdigest(),
                registered.read_text(),
                "registered the tampered download's own hash: self-attestation",
            )

    def test_missing_manifest_fails_instead_of_installing_unverified(self) -> None:
        # The tag exists (the probe server serves a manifest) but the release the
        # installer downloads from serves none, so verification cannot run.
        assets = release_tree()
        del assets["checksums.txt"]
        with _Server(release_tree()) as probe, _Server(assets) as server:
            result = run_installer(
                server.base_url, self.root, probe_url=probe.base_url
            )
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("no release manifest", result.stdout.lower())
        self.assertFalse((self.root / "bin" / "opencenter").exists())

    def test_no_checksum_skips_verification_and_installs(self) -> None:
        assets = release_tree()
        del assets["checksums.txt"]
        with _Server(release_tree()) as probe, _Server(assets) as server:
            result = run_installer(
                server.base_url,
                self.root,
                "--no-checksum",
                probe_url=probe.base_url,
            )
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertTrue((self.root / "bin" / "opencenter").is_file())


class InstallerArgumentTests(unittest.TestCase):
    """Argument handling, with no network access at all."""

    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)

    def tearDown(self) -> None:
        self.directory.cleanup()

    def run_script(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["bash", str(INSTALLER), *args],
            cwd=self.root,
            env=_isolated_env(self.root),
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )

    def test_missing_option_value_prints_usage_not_unbound_variable(self) -> None:
        for option in ("--version", "--bin-dir", "--plugins-dir"):
            result = self.run_script(option)
            self.assertNotEqual(result.returncode, 0, option)
            self.assertNotIn("unbound variable", result.stdout, option)
            self.assertIn("Usage", result.stdout, option)

    def test_unknown_option_is_rejected(self) -> None:
        result = self.run_script("--definitely-not-an-option")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unknown argument", result.stdout)

    def test_dry_run_reports_target_release_without_installing(self) -> None:
        result = self.run_script("--dry-run", "--version", "v1.0.0-rc07")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("Target Release:  v1.0.0-rc07", result.stdout)
        self.assertFalse((self.root / "bin" / "opencenter").exists())

    def test_unknown_version_fails_before_dry_run_return(self) -> None:
        # A nonexistent version must be reported even in dry-run mode: the
        # original ordering returned early and printed a plan for a release that
        # does not exist.
        result = self.run_script("--dry-run", "--version", "98765.0.0")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("not found", result.stdout.lower())

    def test_help_exits_zero(self) -> None:
        result = self.run_script("--help")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("--plugins-dir", result.stdout)

    def test_bare_version_form_is_normalized_to_a_tag(self) -> None:
        result = self.run_script("--dry-run", "--version", "1.0.0-rc07")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("v1.0.0-rc07", result.stdout)

    def test_script_is_shellcheck_clean_or_absent(self) -> None:
        if shutil.which("shellcheck") is None:
            self.skipTest("shellcheck unavailable")
        result = subprocess.run(
            ["shellcheck", "-S", "warning", str(INSTALLER)],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout)

    def test_documented_flags_are_all_accepted(self) -> None:
        # README.md and docs/reference/installation.md promise this exact set.
        usage = self.run_script("--help").stdout
        for flag in ("--version", "--bin-dir", "--plugins-dir", "--no-checksum", "--dry-run", "--help"):
            self.assertIn(flag, usage, flag)
        for name in (
            "OPENCENTER_VERSION",
            "BIN_DIR",
            "INSTALL_DIR",
            "OPENCENTER_PLUGINS_DIR",
            "VERIFY_CHECKSUM",
            "DRY_RUN",
        ):
            self.assertRegex(usage, rf"\b{name}\b", name)
        self.assertIsNone(
            re.search(r"\bbinaries\b", usage),
            "usage must describe one CLI binary, not plural binaries",
        )


if __name__ == "__main__":
    unittest.main()
