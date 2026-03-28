#!/usr/bin/env python3
"""Lightweight Terraform-to-YAML converter.

Extracts ``locals`` and ``module`` blocks from a .tf file and emits a
simplified YAML representation.  This is intentionally *not* a full HCL
parser — it only handles the subset of syntax we actually use in
openCenter infrastructure definitions (scalar assignments, single-line
lists, and one-level-deep object values).  Anything more complex should
be handled by a proper HCL library.

Usage:
    tf2yaml.py <input.tf> <output.yaml>
"""

import re
import sys
from pathlib import Path


# ---------------------------------------------------------------------------
# Low-level text helpers
# ---------------------------------------------------------------------------

def strip_inline_comments(line: str) -> str:
    """Remove ``#`` and ``//`` comments while respecting quoted strings.

    Terraform allows both comment styles.  A naïve split would break on
    comment characters that appear inside string literals, so we walk the
    line character-by-character and track quoting state.
    """
    s = line
    out = []
    i = 0
    in_single = False
    in_double = False
    while i < len(s):
        ch = s[i]
        nxt = s[i + 1] if i + 1 < len(s) else ''

        # Preserve escaped characters verbatim so that an escaped quote
        # (e.g. \") does not toggle the quoting state.
        if ch == '\\':
            if i + 1 < len(s):
                out.append(ch)
                out.append(nxt)
                i += 2
                continue

        if not in_single and ch == '"':
            in_double = not in_double
            out.append(ch)
            i += 1
            continue
        if not in_double and ch == "'":
            in_single = not in_single
            out.append(ch)
            i += 1
            continue

        # Only treat # and // as comment markers when outside any string.
        if not in_single and not in_double:
            if ch == '#' or (ch == '/' and nxt == '/'):
                break

        out.append(ch)
        i += 1
    return ''.join(out).rstrip()


def count_braces_outside_strings(s: str) -> tuple[int, int]:
    """Return (open_count, close_count) of ``{`` / ``}`` ignoring strings.

    Used to track brace depth when collecting multi-line blocks so we
    stop at the correct closing brace rather than one embedded in a
    string literal.
    """
    open_b = close_b = 0
    in_single = in_double = False
    esc = False
    for ch in s:
        if esc:
            esc = False
            continue
        if ch == '\\':
            esc = True
            continue
        if not in_single and ch == '"' and not in_double:
            in_double = True
            continue
        elif in_double and ch == '"':
            in_double = False
            continue
        if not in_double and ch == "'" and not in_single:
            in_single = True
            continue
        elif in_single and ch == "'":
            in_single = False
            continue
        if in_single or in_double:
            continue
        if ch == '{':
            open_b += 1
        elif ch == '}':
            close_b += 1
    return open_b, close_b


# ---------------------------------------------------------------------------
# Value conversion
# ---------------------------------------------------------------------------

def to_yaml_scalar(val: str) -> str:
    """Convert a Terraform scalar value to a YAML-safe representation.

    Handles the value types we encounter in openCenter .tf files:
    inline lists, booleans, integers, and strings.  Unrecognised values
    are single-quoted to avoid YAML interpretation surprises (e.g. a
    bare ``yes`` being parsed as boolean).
    """
    v = val.strip().rstrip(',')

    # Inline lists like ["a", "b"] — pass through as-is.
    if v.startswith('[') and v.endswith(']'):
        return v

    # Terraform booleans map directly to YAML booleans.
    if v in ('true', 'false'):
        return v

    # Bare integers need no quoting.
    if re.fullmatch(r"[0-9]+", v):
        return v

    # Already-quoted strings are kept as-is to preserve the author's
    # intent (double vs. single quotes, interpolation markers, etc.).
    if (v.startswith('"') and v.endswith('"')) or (v.startswith("'") and v.endswith("'")):
        return v

    # Everything else is single-quoted.  Internal single quotes are
    # escaped by doubling them per YAML spec.
    return "'" + v.replace("'", "''") + "'"


# ---------------------------------------------------------------------------
# Block / assignment parsing
# ---------------------------------------------------------------------------

def parse_assignments(lines: list[str], i: int) -> tuple[dict, int]:
    """Walk ``key = value`` lines and return a dict of parsed values.

    Handles three value shapes:
      1. Scalar  — ``key = "value"``
      2. Single-line object — ``key = { a = 1, b = 2 }``
      3. Multi-line object  — opening ``{`` on the assignment line,
         closing ``}`` on a later line.

    Stops when it hits a bare ``}`` (end of enclosing block) or runs
    out of lines.  Returns (mapping, next_line_index).
    """
    mapping: dict[str, object] = {}
    n = len(lines)
    while i < n:
        raw = lines[i]
        line = strip_inline_comments(raw).strip()
        if not line:
            i += 1
            continue

        # A bare closing brace ends the current block.
        if line == '}':
            i += 1
            break

        m = re.match(r"^([A-Za-z0-9_]+)\s*=\s*(.+)$", line)
        if not m:
            i += 1
            continue
        key, val = m.group(1), m.group(2).strip()

        # --- Multi-line object: opening brace without a matching close ---
        if val.startswith('{') and not val.endswith('}'):
            # Accumulate lines until braces balance.  Depth tracking
            # prevents stopping early on nested objects.
            acc = [val]
            depth = 0
            ob, cb = count_braces_outside_strings(val)
            depth += ob - cb
            i += 1
            while i < n and depth > 0:
                seg = strip_inline_comments(lines[i])
                acc.append(seg)
                ob, cb = count_braces_outside_strings(seg)
                depth += ob - cb
                i += 1
            val_full = '\n'.join(acc)
            obj = parse_object(val_full)
            mapping[key] = obj
            continue

        # --- Single-line object: ``{ ... }`` on one line ---
        elif val.startswith('{') and val.endswith('}'):
            mapping[key] = parse_object(val)

        # --- Scalar value ---
        else:
            mapping[key] = to_yaml_scalar(val)

        i += 1
    return mapping, i


def parse_object(s: str) -> dict:
    """Parse a brace-delimited Terraform object into a flat dict.

    Strips the outer ``{ }`` and delegates to ``parse_assignments`` for
    the inner key/value pairs.
    """
    inner = s.strip()
    if inner.startswith('{'):
        inner = inner[1:]
    if inner.endswith('}'):
        inner = inner[:-1]
    lines = [ln for ln in inner.splitlines()]
    mapping, _ = parse_assignments(lines, 0)
    return mapping


# ---------------------------------------------------------------------------
# Top-level block extractors
# ---------------------------------------------------------------------------

def parse_locals(src: str) -> dict:
    """Extract the first ``locals { … }`` block from Terraform source.

    Only top-level scalar assignments are captured — nested blocks and
    expressions are outside the scope of this converter.  Returns an
    empty dict when no locals block is found.
    """
    m = re.search(r"(?m)^\s*locals\s*\{", src)
    if not m:
        return {}
    i = m.end()
    rest = src[i:]
    lines = rest.splitlines()

    # Walk lines while tracking brace depth so we stop at the closing
    # brace of the locals block, not at a nested one.
    mapping = {}
    depth = 1
    i_line = 0
    while i_line < len(lines):
        raw = lines[i_line]
        ob, cb = count_braces_outside_strings(raw)

        # Check *before* processing: if this line's closing braces
        # would drop depth to zero, the locals block is done.
        if depth - cb <= 0:
            break

        line = strip_inline_comments(raw).rstrip()
        m2 = re.match(r"^\s*([A-Za-z0-9_]+)\s*=\s*(.+?)\s*$", line)
        if m2:
            key = m2.group(1)
            val = m2.group(2).rstrip(',').strip()
            mapping[key] = to_yaml_scalar(val)
        depth += ob - cb
        i_line += 1
    return mapping


def parse_modules(src: str) -> dict:
    """Extract all ``module "<name>" { … }`` blocks from Terraform source.

    Each module's body is parsed with ``parse_assignments`` so nested
    object values (e.g. provider configs) are handled correctly.
    Returns a dict keyed by module name.
    """
    modules: dict[str, dict] = {}
    lines = src.splitlines()
    n = len(lines)
    i = 0
    while i < n:
        line_raw = lines[i]
        line = strip_inline_comments(line_raw)
        m = re.match(r'^\s*module\s+"([^"]+)"\s*\{\s*$', line)
        if not m:
            i += 1
            continue
        name = m.group(1)

        # Collect every line inside the module block by tracking brace
        # depth.  We start at depth 1 (the opening brace on the module
        # line) and stop when we return to zero.
        block_lines = []
        depth = 1
        i += 1
        while i < n and depth > 0:
            lr = lines[i]
            block_lines.append(lr)
            ob, cb = count_braces_outside_strings(lr)
            depth += ob - cb
            i += 1

        # parse_assignments will stop at the trailing ``}`` naturally.
        mod_map, _ = parse_assignments(block_lines, 0)
        modules[name] = mod_map
    return modules


# ---------------------------------------------------------------------------
# YAML output
# ---------------------------------------------------------------------------

def write_yaml(locals_map: dict, modules_map: dict, out: Path):
    """Write a two-section YAML file: ``locals`` then ``modules``.

    Supports one level of nesting (dict values) which covers the object
    attributes we use in openCenter Terraform definitions.  Deeper
    nesting would need a recursive writer or a proper YAML library.
    """
    with out.open('w') as f:
        f.write('locals:\n')
        for k, v in locals_map.items():
            if isinstance(v, dict):
                f.write(f'  {k}:\n')
                for sk, sv in v.items():
                    f.write(f'    {sk}: {sv}\n')
            else:
                f.write(f'  {k}: {v}\n')

        f.write('modules:\n')
        for mname, attrs in modules_map.items():
            f.write(f'  {mname}:\n')
            for k, v in attrs.items():
                if isinstance(v, dict):
                    f.write(f'    {k}:\n')
                    for sk, sv in v.items():
                        f.write(f'      {sk}: {sv}\n')
                else:
                    f.write(f'    {k}: {v}\n')


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def main(argv):
    """Read a .tf file, extract locals + modules, and write YAML."""
    if len(argv) < 2:
        print('Usage: tf2yaml.py <input.tf> <output.yaml>', file=sys.stderr)
        return 2
    in_path = Path(argv[0])
    out_path = Path(argv[1]) if len(argv) > 1 else Path('-')
    src = in_path.read_text()
    locals_map = parse_locals(src)
    modules_map = parse_modules(src)
    write_yaml(locals_map, modules_map, out_path)
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))