#!/usr/bin/env python3
import re
import sys
from pathlib import Path


def strip_inline_comments(line: str) -> str:
    s = line
    out = []
    i = 0
    in_single = False
    in_double = False
    while i < len(s):
        ch = s[i]
        nxt = s[i + 1] if i + 1 < len(s) else ''
        if ch == '\\':
            # keep escape and next char
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
        if not in_single and not in_double:
            # line comment start with // or #
            if ch == '#' or (ch == '/' and nxt == '/'):
                break
        out.append(ch)
        i += 1
    return ''.join(out).rstrip()


def count_braces_outside_strings(s: str) -> tuple[int, int]:
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


def to_yaml_scalar(val: str) -> str:
    v = val.strip().rstrip(',')
    # Lists in one line
    if v.startswith('[') and v.endswith(']'):
        return v
    # Booleans
    if v in ('true', 'false'):
        return v
    # Integers
    if re.fullmatch(r"[0-9]+", v):
        return v
    # Already quoted strings
    if (v.startswith('"') and v.endswith('"')) or (v.startswith("'") and v.endswith("'")):
        return v
    # Default: quote as string, use single quotes; escape single quotes
    return "'" + v.replace("'", "''") + "'"


def parse_assignments(lines: list[str], i: int) -> tuple[dict, int]:
    """Parse simple key = value assignments until end index.
    Returns (mapping, next_index).
    """
    mapping: dict[str, object] = {}
    n = len(lines)
    while i < n:
        raw = lines[i]
        line = strip_inline_comments(raw).strip()
        if not line:
            i += 1
            continue
        # End of block?
        if line == '}':
            i += 1
            break
        m = re.match(r"^([A-Za-z0-9_]+)\s*=\s*(.+)$", line)
        if not m:
            i += 1
            continue
        key, val = m.group(1), m.group(2).strip()
        # Object value
        if val.startswith('{') and not val.endswith('}'):  # multiline object
            # accumulate until balanced
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
        elif val.startswith('{') and val.endswith('}'):  # single-line object
            mapping[key] = parse_object(val)
        else:
            mapping[key] = to_yaml_scalar(val)
        i += 1
    return mapping, i


def parse_object(s: str) -> dict:
    # remove outer braces
    inner = s.strip()
    if inner.startswith('{'):
        inner = inner[1:]
    if inner.endswith('}'): 
        inner = inner[:-1]
    lines = [ln for ln in inner.splitlines()]
    # Convert object assignments
    i = 0
    mapping, _ = parse_assignments(lines, i)
    return mapping


def parse_locals(src: str) -> dict:
    m = re.search(r"(?m)^\s*locals\s*\{", src)
    if not m:
        return {}
    i = m.end()
    rest = src[i:]
    lines = rest.splitlines()
    # parse until matching closing brace of the locals block
    mapping = {}
    depth = 1
    i_line = 0
    while i_line < len(lines):
        raw = lines[i_line]
        ob, cb = count_braces_outside_strings(raw)
        # If this line would close the block entirely, stop before processing
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
        # collect block lines until matching closing brace
        block_lines = []
        depth = 1
        i += 1
        while i < n and depth > 0:
            lr = lines[i]
            block_lines.append(lr)
            ob, cb = count_braces_outside_strings(lr)
            depth += ob - cb
            i += 1
        # remove trailing closing brace line from block_lines if present
        # (parse_assignments will stop at '}' anyway)
        mod_map, _ = parse_assignments(block_lines, 0)
        modules[name] = mod_map
    return modules


def write_yaml(locals_map: dict, modules_map: dict, out: Path):
    with out.open('w') as f:
        # locals
        f.write('locals:\n')
        for k, v in locals_map.items():
            if isinstance(v, dict):
                f.write(f'  {k}:\n')
                for sk, sv in v.items():
                    f.write(f'    {sk}: {sv}\n')
            else:
                f.write(f'  {k}: {v}\n')
        # modules
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


def main(argv):
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

