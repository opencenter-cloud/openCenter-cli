---
doc_type: how-to
---

# Release Process


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Versioning Strategy](#versioning-strategy)
- [Release Types](#release-types)
- [Release Checklist](#release-checklist)
- [Building Releases](#building-releases)
- [Changelog Management](#changelog-management)
- [Testing Releases](#testing-releases)
- [Distribution](#distribution)
- [Hotfix Process](#hotfix-process)
- [Version Support](#version-support)
- [Release Schedule](#release-schedule)
- [Rollback Procedure](#rollback-procedure)
- [Security Releases](#security-releases)
- [Documentation Updates](#documentation-updates)
- [Metrics and Monitoring](#metrics-and-monitoring)
- [Communication](#communication)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)
This document describes the release process for openCenter, including versioning, building, testing, and distribution.

## Who this is for

Maintainers responsible for creating and publishing openCenter releases.

## Versioning Strategy

openCenter follows [Semantic Versioning 2.0.0](https://semver.org/):

**Format**: `MAJOR.MINOR.PATCH`

- **MAJOR**: Incompatible API changes
- **MINOR**: New functionality, backward compatible
- **PATCH**: Bug fixes, backward compatible

**Pre-release versions**:
- `1.0.0-alpha.1`: Alpha releases (unstable)
- `1.0.0-beta.1`: Beta releases (feature complete, testing)
- `1.0.0-rc.1`: Release candidates (production ready, final testing)

**Version metadata**:
- `1.0.0+build.123`: Build metadata (not part of version precedence)

## Release Types

### Patch Release (1.0.x)

**When**: Bug fixes, security patches, minor improvements

**Changes allowed**:
- Bug fixes
- Security patches
- Documentation updates
- Performance improvements (no API changes)

**Not allowed**:
- New features
- Breaking changes
- API modifications

### Minor Release (1.x.0)

**When**: New features, backward compatible changes

**Changes allowed**:
- New commands
- New providers
- New configuration options (with defaults)
- Deprecations (with warnings)
- All patch release changes

**Not allowed**:
- Breaking changes
- Removing features
- Changing existing behavior

### Major Release (x.0.0)

**When**: Breaking changes, major refactoring

**Changes allowed**:
- Breaking API changes
- Removing deprecated features
- Changing configuration schema
- Major architectural changes
- All minor release changes

**Requirements**:
- Migration guide
- Deprecation warnings in previous minor release
- Clear upgrade path

## Release Checklist

### Pre-Release (1-2 weeks before)

- [ ] **Review milestone**: All issues and PRs closed or moved
- [ ] **Update dependencies**: Run `mise run upgrade-deps`
- [ ] **Run full test suite**:
  ```bash
  mise run test
  mise run godog
  mise run test-security
  mise run test-integration
  ```
- [ ] **Update documentation**:
  - [ ] Update version references
  - [ ] Update compatibility matrix
  - [ ] Review and update all docs
- [ ] **Test on all platforms**:
  - [ ] Linux (AMD64, ARM64)
  - [ ] macOS (Intel, Apple Silicon)
  - [ ] Windows (AMD64)
- [ ] **Create release branch**: `release/1.0.0`

### Release Candidate (1 week before)

- [ ] **Tag release candidate**: `1.0.0-rc.1`
- [ ] **Build binaries**:
  ```bash
  mise run build-all
  ```
- [ ] **Test release candidate**:
  - [ ] Install from binary
  - [ ] Run smoke tests
  - [ ] Test upgrade path
- [ ] **Gather feedback**: Share with early adopters
- [ ] **Fix critical issues**: Create RC2, RC3 if needed

### Release Day

- [ ] **Final testing**:
  ```bash
  mise run test
  mise run godog
  mise run schema-verify
  ```
- [ ] **Update CHANGELOG.md**: Add release notes
- [ ] **Update version**: Ensure version is correct
- [ ] **Create git tag**:
  ```bash
  git tag -a 1.0.0 -m "Release 1.0.0"
  git push origin 1.0.0
  ```
- [ ] **Build release binaries**:
  ```bash
  mise run build-all
  ```
- [ ] **Create GitHub release**:
  - [ ] Upload binaries
  - [ ] Add release notes
  - [ ] Mark as latest release
- [ ] **Update documentation site**: Deploy updated docs
- [ ] **Announce release**:
  - [ ] GitHub discussions
  - [ ] Mailing list
  - [ ] Social media

### Post-Release

- [ ] **Monitor for issues**: Watch GitHub issues
- [ ] **Update roadmap**: Plan next release
- [ ] **Merge release branch**: Back to main
- [ ] **Close milestone**: Mark milestone as complete

## Building Releases

### Local Build

Build for current platform:
```bash
mise run build
```

Build for all platforms:
```bash
mise run build-all
```

This creates binaries in `bin/`:
- `openCenter-linux-amd64`
- `openCenter-linux-arm64`
- `openCenter-darwin-amd64`
- `openCenter-darwin-arm64`
- `openCenter-windows-amd64.exe`

### Version Injection

Version information is injected at build time via ldflags:

```bash
GIT_COMMIT=$(git rev-parse HEAD)
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || echo "")
BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
VERSION=${GIT_TAG:-"0.0.1"}

go build -ldflags "\
  -X main.version=${VERSION} \
  -X main.gitCommit=${GIT_COMMIT} \
  -X main.gitBranch=${GIT_BRANCH} \
  -X main.gitTag=${GIT_TAG} \
  -X main.buildDate=${BUILD_DATE}" \
  -o bin/openCenter
```

Verify version:
```bash
./bin/openCenter version
```

### CI/CD Build

Automated builds run on:
- Every commit to `main`
- Every pull request
- Every tag push

CI builds:
1. Run tests
2. Build binaries for all platforms
3. Run smoke tests
4. Upload artifacts (for tags)

## Changelog Management

### Format

Follow [Keep a Changelog](https://keepachangelog.com/) format:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New features

### Changed
- Changes to existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Removed features

### Fixed
- Bug fixes

### Security
- Security fixes

## [1.0.0] - 2024-01-15

### Added
- Initial release
- Cluster initialization
- GitOps repository generation
- SOPS integration
```

### Generating Changelog

1. **Review commits** since last release:
   ```bash
   git log v0.9.0..HEAD --oneline
   ```

2. **Categorize changes**:
   - Added: New features
   - Changed: Changes to existing functionality
   - Deprecated: Soon-to-be removed features
   - Removed: Removed features
   - Fixed: Bug fixes
   - Security: Security fixes

3. **Write user-facing descriptions**:
   - Focus on user impact, not implementation
   - Include examples where helpful
   - Link to issues and PRs

4. **Update CHANGELOG.md**:
   - Move Unreleased changes to new version section
   - Add release date
   - Add comparison links at bottom

## Testing Releases

### Smoke Tests

Basic functionality tests:

```bash
# Version check
./bin/openCenter version

# Help output
./bin/openCenter --help

# Initialize cluster
./bin/openCenter cluster init test-cluster

# Validate configuration
./bin/openCenter cluster validate test-cluster

# Generate schema
./bin/openCenter cluster schema --pretty
```

### Integration Tests

Full workflow tests:

```bash
# Run all BDD tests
mise run godog

# Run priority tests
mise run test-priority1
mise run test-priority2
```

### Platform Tests

Test on each platform:

**Linux**:
```bash
./openCenter-linux-amd64 version
./openCenter-linux-amd64 cluster init test-linux
```

**macOS**:
```bash
./openCenter-darwin-amd64 version
./openCenter-darwin-amd64 cluster init test-macos
```

**Windows**:
```powershell
.\openCenter-windows-amd64.exe version
.\openCenter-windows-amd64.exe cluster init test-windows
```

### Upgrade Tests

Test upgrade from previous version:

1. **Install previous version**
2. **Create cluster configuration**
3. **Upgrade to new version**
4. **Verify configuration still works**
5. **Test new features**

## Distribution

### GitHub Releases

1. **Create release** on GitHub
2. **Upload binaries**:
   - `openCenter-linux-amd64`
   - `openCenter-linux-arm64`
   - `openCenter-darwin-amd64`
   - `openCenter-darwin-arm64`
   - `openCenter-windows-amd64.exe`
3. **Add checksums**:
   ```bash
   sha256sum openCenter-* > checksums.txt
   ```
4. **Add release notes** from CHANGELOG.md
5. **Mark as latest release**

### Package Managers

Future distribution channels:

- **Homebrew**: `brew install openCenter`
- **APT**: `apt install openCenter`
- **RPM**: `yum install openCenter`
- **Chocolatey**: `choco install openCenter`
- **Snap**: `snap install openCenter`

## Hotfix Process

For critical bugs in production:

1. **Create hotfix branch** from release tag:
   ```bash
   git checkout -b hotfix/v1.0.1 1.0.0
   ```

2. **Fix the bug** with minimal changes

3. **Test thoroughly**:
   ```bash
   mise run test
   mise run godog
   ```

4. **Update CHANGELOG.md**:
   ```markdown
   ## [1.0.1] - 2024-01-16
   
   ### Fixed
   - Critical bug in cluster initialization
   ```

5. **Tag and release**:
   ```bash
   git tag -a v1.0.1 -m "Hotfix v1.0.1"
   git push origin v1.0.1
   ```

6. **Build and distribute** following normal release process

7. **Merge back** to main:
   ```bash
   git checkout main
   git merge hotfix/v1.0.1
   git push origin main
   ```

## Version Support

### Support Policy

- **Latest major version**: Full support
- **Previous major version**: Security fixes for 6 months
- **Older versions**: No support

### End of Life

When a version reaches end of life:

1. **Announce EOL** 3 months in advance
2. **Update documentation** with EOL date
3. **Provide migration guide** to newer version
4. **Stop releasing updates** after EOL date

## Release Schedule

### Regular Releases

- **Minor releases**: Every 2-3 months
- **Patch releases**: As needed for bugs
- **Major releases**: Annually or as needed

### Release Windows

Avoid releasing during:
- Major holidays
- End of year
- Known high-traffic periods

Prefer releasing:
- Tuesday-Thursday
- Mid-month
- After thorough testing period

## Rollback Procedure

If a release has critical issues:

1. **Assess severity**: Is rollback necessary?

2. **Communicate**: Notify users immediately

3. **Remove release**: Unmark as latest on GitHub

4. **Revert tag** (if necessary):
   ```bash
   git tag -d 1.0.0
   git push origin :refs/tags/1.0.0
   ```

5. **Create hotfix**: Follow hotfix process

6. **Post-mortem**: Document what went wrong

## Security Releases

For security vulnerabilities:

1. **Assess severity**: Use CVSS scoring

2. **Develop fix** in private

3. **Coordinate disclosure**:
   - Notify affected users privately
   - Prepare security advisory
   - Set disclosure date

4. **Release fix**:
   - Create patch for all supported versions
   - Publish security advisory
   - Update CHANGELOG with security section

5. **Announce**: After fix is available

## Documentation Updates

### Version-Specific Docs

Maintain docs for:
- Latest version (main branch)
- Previous major version (branch)

### Update Process

1. **Update version references** throughout docs
2. **Add migration guides** for breaking changes
3. **Update compatibility matrix**
4. **Review all examples** for accuracy
5. **Deploy updated docs** to documentation site

## Metrics and Monitoring

Track release metrics:
- Download counts
- Installation success rate
- Bug reports per release
- Time to fix critical bugs
- User feedback

Use metrics to improve:
- Testing coverage
- Release quality
- Release frequency
- Documentation quality

## Communication

### Release Announcement Template

```markdown
# openCenter 1.0.0 Released

We're excited to announce the release of openCenter 1.0.0!

## Highlights

- New AWS provider support
- Improved validation with better error messages
- Performance improvements in GitOps generation

## Breaking Changes

- Configuration schema updated (migration guide available)
- Deprecated flags removed

## Installation

Download from GitHub releases:
https://github.com/rackerlabs/openCenter-cli/releases/tag/1.0.0

Or use Homebrew:
```bash
brew install openCenter
```

## Upgrade Guide

See the migration guide for upgrading from v0.9.x:
https://docs.opencenter.io/migration/v0.9-to-v1.0

## Full Changelog

See CHANGELOG.md for complete list of changes.

## Thank You

Thanks to all contributors who made this release possible!
```

### Channels

Announce releases on:
- GitHub Releases
- GitHub Discussions
- Project mailing list
- Social media (Twitter, LinkedIn)
- Internal communication channels

## Troubleshooting

### Build Failures

**Issue**: Build fails on specific platform

**Solution**:
1. Check Go version compatibility
2. Verify platform-specific dependencies
3. Test in clean environment
4. Check for platform-specific code issues

### Test Failures

**Issue**: Tests fail in CI but pass locally

**Solution**:
1. Check for environment-specific issues
2. Verify test isolation
3. Check for timing issues
4. Review CI logs carefully

### Version Mismatch

**Issue**: Binary reports wrong version

**Solution**:
1. Verify git tag is correct
2. Check ldflags in build command
3. Ensure clean build (remove old binaries)
4. Verify version injection code

## See Also

- [Developer Guide](./README.md) - Development setup and workflows
- [Contributing Guidelines](./contributing.md) - How to contribute
- [Architecture Documentation](./architecture.md) - Codebase architecture
- [Testing Guide](./testing/README.md) - Testing strategies
