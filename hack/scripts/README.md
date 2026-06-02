# hack/scripts

Maintenance utilities for the openCenter CLI repository. Reusable
across the openCenter ecosystem.

## Available scripts

| Script | Purpose |
|---|---|
| `convert_adoc_to_md.py` | Convert an Antora `docs/modules/ROOT/pages/**/*.adoc` tree into Diátaxis-shaped Markdown under `docs/<category>/`. Builds Docusaurus YAML frontmatter from AsciiDoc page attributes (`:description:`, `:page-type:`, `:category:`, `:audience:`, `:tags:`, `:id:`). Idempotent; skip-existing by default. Requires `downdoc` (`npm i -g downdoc`). |
| `audit_doc_frontmatter.py` | Verify every `docs/**/*.md` page has the required Diátaxis frontmatter (`id`, `title`, `sidebar_label`, `description`, `doc_type`, `audience`, `tags`) and that `doc_type` is one of `tutorial`, `how-to`, `reference`, `explanation`. Exits non-zero on failure for CI use. |
| `openstack-reset.sh` | Tear down OpenStack project resources between cluster lifecycle tests. Not documentation-related. |
| `tf2yaml.py` | Lightweight Terraform-to-YAML converter for `locals` and `module` blocks. Used by the GitOps templating helpers. |

## Usage

Run from the repository root:

```bash
# Convert any leftover AsciiDoc pages (default: skip existing .md)
python3 hack/scripts/convert_adoc_to_md.py

# Verify every Markdown page has Diátaxis frontmatter
python3 hack/scripts/audit_doc_frontmatter.py
```

Both documentation scripts assume the docs layout used by openCenter
components:

- Markdown sources under `docs/<category>/<name>.md`
- (Optional, transitional) AsciiDoc sources under `docs/modules/ROOT/pages/`

If a repository does not have one of those trees, the converter
exits with a no-op message and the auditor only checks what it finds.

## When to use these

- Migrating a repository's docs from Antora/AsciiDoc to Markdown.
- Pre-merge validation that every page has Diátaxis frontmatter
  (Documentation steering rule §2).
- Adding fresh pages: copy frontmatter from a sibling page and run
  `audit_doc_frontmatter.py` before commit.
