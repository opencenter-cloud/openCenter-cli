# Security Policy

## Supported Versions

Security reports are accepted for:

- The latest tagged release
- The current `main` branch

Older, unsupported versions may not receive fixes.

## Reporting a Vulnerability

Report vulnerabilities privately through one of these channels:

- GitHub Security Advisories for this repository
- Email: `security@opencenter.cloud`

Include:

- A clear description of the issue
- The affected version or commit
- Reproduction steps or a proof of concept
- Expected impact, if known

Do not open a public GitHub issue for an unpatched vulnerability.

## Response Targets

The maintainers aim to:

- Acknowledge reports within 3 business days
- Provide an initial triage update within 7 business days
- Coordinate a remediation and disclosure timeline based on severity

## Scope

In scope:

- The `opencenter` CLI
- Release artifacts built from this repository
- External plugin trust and execution behavior documented here

Out of scope unless directly caused by this repository:

- Third-party infrastructure misconfiguration
- Vulnerabilities in external services deployed by the CLI
- Issues that require privileged local access without a CLI weakness
