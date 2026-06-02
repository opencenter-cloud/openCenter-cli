---
id: release-process
title: "Release Process"
sidebar_label: Release Process
description: Cut, sign, and publish a release of openCenter-cli, including version bumps and changelog updates.
doc_type: how-to
audience: "maintainers"
tags: [contributing]
---
# Release Process

**Purpose:** For maintainers, shows how to create and publish releases of openCenter-cli.

## Prerequisites

Before creating a release, you need:

* Maintainer access to the repository
* Git configured with commit signing
* GitHub CLI (`gh`) installed (optional but recommended)
* All tests passing on main branch

## Release Types

### Semantic Versioning

openCenter-cli follows [Semantic Versioning](https://semver.org/):

* **Major** (v2.0.0) - Breaking changes
* **Minor** (v1.1.0) - New features, backward compatible
* **Patch** (v1.0.1) - Bug fixes, backward compatible
* **Pre-release** (v1.0.0-rc1) - Release candidates

### Release Cadence

* **Major releases** - As needed for breaking changes
* **Minor releases** - Monthly or when features are ready
* **Patch releases** - As needed for critical bugs
* **Pre-releases** - Before major/minor releases for testing

## Step 1: Prepare Release

### Update CHANGELOG.md

Move unreleased changes to new version section:

```markdown
## [1.2.0] - 2026-02-17

### Added
- AWS provider support with EC2 provisioning
- Multi-cluster management commands
- Shell integration for active cluster display

### Changed
- Improved validation error messages
- Updated default Kubernetes version to 1.33.5

### Fixed
- VRRP validation logic for OpenStack
- Template rendering for VMware provider

### Security
- Updated dependencies with security patches

## [Unreleased]

### Added
- (empty for next release)
```

### Update Version Documentation

Update version references in:

* `README.md` - Installation instructions
* `docs/tutorials/getting-started.md` - Version examples
* `docs/reference/default-values.md` - Default versions

### Run Full Test Suite

```bash
# Build
mise run build

# Run all tests
mise run test
mise run godog

# Schema verification
mise run schema-verify

# Security tests
mise run test-security

# Integration tests
mise run test-integration
```

All tests must pass before proceeding.

## Step 2: Push The Release Tag

The GitHub Actions release workflow is the source of truth for published artifacts.
Pushing the tag also runs the container image workflow, which publishes the
multi-architecture CLI image to `ghcr.io/opencenter-cloud/opencenter-cli`.

```bash
git tag -a v1.2.0 -m "Release 1.2.0"
git push origin v1.2.0
```

The tag-triggered workflow builds the supported Linux and macOS binaries, generates
`checksums.txt`, signs the release assets with cosign keyless signing, emits an SBOM,
and creates the GitHub release automatically.

## Step 3: Watch The Release Workflow

Use GitHub Actions or the GitHub CLI to monitor the run:

```bash
gh run list --workflow release.yml
gh run watch
```

## Step 4: Verify Release

### Review Generated Assets

Verify the GitHub release contains:

* the four platform binaries
* `checksums.txt`
* cosign signature and certificate files
* `opencenter.spdx.json`

Verify the GitHub Container Registry package contains:

* `ghcr.io/opencenter-cloud/opencenter-cli:1.2.0`
* `ghcr.io/opencenter-cloud/opencenter-cli:1.2`
* the corresponding `sha-*` tag

### Local Dry Run Helper

`mise run release <version>` remains available for local preflight builds and manual
artifact inspection, but it is no longer the publishing mechanism for official releases.

## Step 5: Verify Release

### Download and Test

```bash
# Download binary
curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.2.0/opencenter-1.2.0-linux-amd64 -o opencenter

# Make executable
chmod +x opencenter

# Test
./opencenter version
./opencenter cluster init test --org test-org
```

### Check Release Page

Verify on GitHub:

* Release appears in releases list
* All binaries are attached
* Release notes are correct
* Installation instructions work

## Step 6: Announce Release

### Update Documentation

Update installation instructions:

```markdown
## Installation

Download the latest release:

**Linux (x86_64)**
```bash
curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.2.0/opencenter-1.2.0-linux-amd64 -o opencenter
chmod +x opencenter
sudo mv opencenter /usr/local/bin/
```

**macOS (Apple Silicon)**

```bash
curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.2.0/opencenter-1.2.0-darwin-arm64 -o opencenter
chmod +x opencenter
sudo mv opencenter /usr/local/bin/
```

```

### Announce Release

Announce in:
- GitHub Discussions
- Team Slack/chat
- Project mailing list
- Release notes blog post (if applicable)

## Release Checklist

Before releasing, verify:

- [ ] All tests pass (`mise run test && mise run godog`)
- [ ] CHANGELOG.md updated with release notes
- [ ] Version documentation updated
- [ ] Release binaries built (`mise run release v1.2.0`)
- [ ] Release notes generated (`mise run publish 1.2.0`)
- [ ] Binaries tested on target platforms
- [ ] Git tag created and pushed
- [ ] GitHub release created with binaries
- [ ] GHCR image published for the release tag
- [ ] Release verified by downloading and testing
- [ ] Documentation updated with new version
- [ ] Release announced to team

## Hotfix Releases

For critical bugs in production:

1. Create hotfix branch from release tag:
   ```bash
   git checkout -b hotfix/1.2.1 v1.2.0
```

1. Fix bug and commit:

   ```bash
   git commit -m "fix: critical bug in validation"
   ```
2. Update CHANGELOG.md:

   ```markdown
   ## [1.2.1] - 2026-02-18

   ### Fixed
   - Critical bug in validation logic
   ```
3. Build and release:

   ```bash
   mise run release v1.2.1
   mise run publish 1.2.1
   git tag -a v1.2.1 -m "Hotfix 1.2.1"
   git push origin v1.2.1
   gh release create v1.2.1 --notes-file bin/release/RELEASE_NOTES_1.2.1.md bin/release/opencenter-*
   ```
4. Merge hotfix back to main:

   ```bash
   git checkout main
   git merge hotfix/1.2.1
   git push origin main
   ```

## Pre-Release Testing

Before major/minor releases, create release candidate:

```bash
# Build RC
mise run release v1.2.0-rc1

# Create pre-release
gh release create v1.2.0-rc1 \
  --prerelease \
  --notes "Release candidate for 1.2.0. Please test and report issues." \
  bin/release/opencenter-*
```

Test for 1-2 weeks before final release.

## Rollback

If a release has critical issues:

1. Mark release as pre-release on GitHub
2. Add warning to release notes
3. Create hotfix release
4. Update documentation to point to previous stable version

## Common Issues

### Build fails with version mismatch

**Problem:** `go.mod` version doesn’t match build

**Solution:**

```bash
# Update go.mod
go mod tidy

# Rebuild
mise run build
```

### Tag already exists

**Problem:** Git tag already exists

**Solution:**

```bash
# Delete local tag
git tag -d v1.2.0

# Delete remote tag
git push origin :refs/tags/v1.2.0

# Recreate tag
git tag -a v1.2.0 -m "Release 1.2.0"
git push origin v1.2.0
```

### Release notes missing commits

**Problem:** Some commits not in release notes

**Solution:**

```bash
# Manually generate changelog
git log v1.1.0..v1.2.0 --oneline --no-merges

# Edit release notes
vim bin/release/RELEASE_NOTES_1.2.0.md
```

### Binary doesn’t run on target platform

**Problem:** Binary fails with "exec format error"

**Solution:**

```bash
# Verify GOOS/GOARCH
file bin/release/opencenter-1.2.0-linux-amd64

# Rebuild with correct platform
GOOS=linux GOARCH=amd64 go build -o bin/release/opencenter-1.2.0-linux-amd64
```

---

## Evidence

This documentation is based on the following repository files:

* Release task: `.mise.toml:127-223` (release task)
* Publish task: `.mise.toml:225-326` (publish task)
* Version injection: `.mise.toml:23-47` (build task with ldflags)
* Version management: `.kiro/steering/tech.md:143-149`
* Build system: `.mise.toml:1-961`
* Contributing guide: `CONTRIBUTING.md:1-82`