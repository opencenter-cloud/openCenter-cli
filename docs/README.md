---
id: docs-readme
title: "Documentation Maintenance"
sidebar_label: Documentation Maintenance
description: Repository map and maintenance guide for openCenter CLI Markdown documentation.
doc_type: reference
audience: "contributors, maintainers"
tags: [documentation, maintenance, diataxis]
last_updated: 2026-11-05
---

# Documentation maintenance

**Purpose:** For contributors and maintainers, explains where documentation
source lives, how pages are classified, and how to verify documentation
changes. This is a repository guide, not a claim about a published site.

## Boundaries

- [`docs/index.md`](index.md) is the canonical reader-facing source index.
- `README.md` at the repository root is the concise project entry point.
- `llms.txt` at the repository root is a code-oriented navigation aid; it should not
  duplicate the reader-facing index or generated command reference.
- [`CODEMAPS/INDEX.md`](CODEMAPS/INDEX.md) contains repository-internal code
  maps and is not reader-facing product documentation.
- [`docs/README.md`](README.md) is this maintenance guide.

The repository contains Markdown source. Do not describe a hosted site as
published unless repository configuration or release evidence establishes that
fact.

## Source layout and Diátaxis classification

Directories provide lifecycle navigation; they do not create a second set of
taxonomy folders. Each reader-facing page has one `doc_type` in its YAML
frontmatter:

| Source area | Classification | Use it for |
| --- | --- | --- |
| `getting-started/` | `tutorial` | A guided first outcome |
| `operations/` | `how-to` | A task or operational procedure |
| `reference/` | `reference` | Structured facts, fields, commands, and options |
| `concepts/` | `explanation` | Context, rationale, and system behavior |
| `providers/` | Usually `reference` | Provider-specific facts and procedures |
| `contributing/` | Choose one per page | Developer procedures, facts, or explanations |
| `index.md`, `glossary.md` | `reference` | Navigation and terminology |
| `CODEMAPS/` | Internal explanation | Code maps; excluded from the reader-facing set |

Use the [Diátaxis framework](https://diataxis.fr/) to choose the page type,
but keep existing lifecycle directories. Do not create `tutorial/`,
`how-to/`, `reference/`, or `explanation/` taxonomy directories.

## Required page metadata

Reader-facing pages start with frontmatter containing:

```yaml
id: url-safe-slug
title: "Page title"
sidebar_label: Page title
description: One-sentence scope.
doc_type: tutorial | how-to | reference | explanation
audience: "specific audience"
tags: [at-least-one-tag]
last_updated: YYYY-MM-DD
```

Use exactly one `doc_type`. Begin the body with a `**Purpose:**` line that
states the audience and scope. Advance `last_updated` only for a substantive
change to meaning, instructions, or technical facts; do not advance it for a
link-only or formatting change.

`docs/README.md` and `docs/CODEMAPS/**` are explicitly excluded from the
default frontmatter audit because they are repository navigation material, but
this guide keeps metadata so contributors can classify it consistently.

## Maintenance workflow

1. **Find the source of truth.** Re-check code, configuration, tests, and CI
   before changing a technical claim. Prefer links to the detailed page over
   repeating a value in an index or navigation file.
2. **Choose the page type.** Apply one Diátaxis `doc_type`; keep the page in
   the lifecycle directory that readers already use.
3. **Update the page and its links.** Keep local links relative to the file,
   point to files rather than empty directory paths, and remove links to files
   that do not exist. Add `last_updated` for substantive changes.
4. **Regenerate generated command reference when needed.** If the built-in
   Cobra command tree changes, run `mise run docs-gen`. Do not hand-edit
   `reference/opencenter/`; the generator is `go run cmd/docs/generate.go`.
5. **Review the diff.** Confirm that examples, flags, provider boundaries,
   and links still match the source. Avoid copying generated command details
   into README, the index, or `llms.txt`.

## Checks

The repository-defined frontmatter audit is:

```bash
python3 hack/scripts/audit_doc_frontmatter.py --strict
# or, through mise:
mise run test-docs-frontmatter
```

The pull-request docs workflow audits changed Markdown files, checks that no
page's `last_updated` predates its last body change, runs Vale, and runs
`mise run test-docs`; see `.github/workflows/docs-p0.yml` at the repository root.
For generated command pages, use `mise run test-docs` after regeneration. This
is a repository-only check: it does not publish or deploy a documentation site.
Its Mermaid coverage is structural and fence-only; it does not render diagrams.

`mise run test-docs` also verifies that every task defined in `.mise.toml` has a
row in [Mise tasks](reference/mise-tasks.md), and that every command shown in a
shell code block resolves against the built binary. Run `mise run build` first,
or the command check skips.

Two hooks in `.pre-commit-config.yaml` at the repository root keep
frontmatter honest: `doc-last-updated-refresh` rewrites `last_updated` on every
staged page, and `doc-frontmatter` audits the result. A content edit leaves the
old date untouched and invisible in the diff, which is why the refresh is a hook
while the staleness check also runs in CI -- `git commit --no-verify` skips the
first, not the second.

For the local full-package Go sweep, use `mise run test-go-all`. The task gives
each Go test run a 15-minute timeout and, on failure, writes a transient failure
log under `.tmp/` for diagnosis rather than keeping a repository artifact. This
sweep is separate from `mise run test:all`, the broader aggregate of the
repository's unit, race, vet, BDD, property, and vulnerability checks.

## Useful entry points

- [Reader-facing documentation index](index.md)
- `README.md` at the repository root (outside this documentation tree)
- [CLI command reference](reference/cli-commands.md)
- [Configuration schema](reference/configuration-schema.md)
- [Glossary](glossary.md)
- [cluster deploy — Step Matrix per Provider](contributing/cluster-deploy-steps.md)

## Codebase maps

The repository-internal maps under `CODEMAPS/` are indexed through
[`CODEMAPS/INDEX.md`](CODEMAPS/INDEX.md), which links every map in the family:
[CLI commands](CODEMAPS/cli-commands.md),
[Config system](CODEMAPS/config-system.md),
[DI container](CODEMAPS/di-container.md),
[Cluster lifecycle](CODEMAPS/cluster-lifecycle.md),
[GitOps engine](CODEMAPS/gitops-engine.md),
[Rendering ownership and secret artifacts](CODEMAPS/rendering-ownership-and-secret-artifacts.md),
[Secrets management](CODEMAPS/secrets-management.md),
[Providers](CODEMAPS/providers.md),
[OpenStack provider and storage operations](CODEMAPS/openstack-provider-storage-operations.md),
[Import, operations, and resilience](CODEMAPS/import-operations-and-resilience.md), and
[Runtime extensions and local development](CODEMAPS/runtime-extensions-and-local-development.md).
