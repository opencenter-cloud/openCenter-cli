# openCenter CLI documentation

This folder is the source for the openCenter CLI documentation. Pages are
written in Markdown with Docusaurus-compatible YAML frontmatter and follow
the [Diátaxis](https://diataxis.fr/) framework: every page is one of
`tutorial`, `how-to`, `reference`, or `explanation`.

## Layout

Pages are organised by **lifecycle category**, not by Diátaxis type. The
category a page lives in matches its `doc_type` per these defaults:

| Directory | Default `doc_type` | Purpose |
|---|---|---|
| `getting-started/` | `tutorial` | First-cluster walkthroughs |
| `operations/` | `how-to` | Day-2 task guides |
| `reference/` | `reference` | CLI, schema, flags, lookup |
| `concepts/` | `explanation` | Architecture and rationale |
| `providers/` | `reference` | Per-provider guides |
| `contributing/` | `explanation` | Developer docs |
| `release/` | `reference` | Release notes |

Top-level files (`index.md`, `glossary.md`) live directly under `docs/`.

```
docs/
├── index.md
├── glossary.md
├── getting-started/
├── operations/
├── reference/
│   └── opencenter/        # Auto-generated Cobra command reference
├── concepts/
├── providers/
├── contributing/
└── release/
```

## Editing rules

- Every page must start with YAML frontmatter that includes `id`,
  `title`, `sidebar_label`, `description`, `doc_type`, `audience`,
  and `tags`. Run `python3 hack/scripts/audit_doc_frontmatter.py` to
  verify before committing.
- Pick exactly one `doc_type` per file. Split mixed content into
  separate pages and cross-link.
- Start the body with a `**Purpose:**` line that names the audience
  and what the page covers.
- Place the page in the lifecycle directory that matches the reader's
  task ("get started", "operate something", "look up a flag").
- Refresh the per-command reference under `reference/opencenter/`
  with `go run -tags tools ./cmd/docs` whenever the Cobra command
  tree changes.

## Tooling

Maintenance utilities live in [`../hack/scripts/`](../hack/scripts/):

- `convert_adoc_to_md.py` — convert any leftover Antora `.adoc` pages
  to Markdown with proper Diátaxis frontmatter.
- `audit_doc_frontmatter.py` — verify every Markdown page satisfies
  the frontmatter rules. Suitable for CI use.

## Other folders in this repo

- [`CODEMAPS/`](CODEMAPS/) — generated architecture maps used by the
  development workflow. Not part of the published documentation site.
- [`superpowers/`](superpowers/) — internal planning specs and design
  notes. Not part of the published documentation site.
