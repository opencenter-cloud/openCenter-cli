#!/usr/bin/env python3
"""
Run the Godog suite in cucumber (JSON) format, detect failing scenarios,
and tag them with @wip in their respective .feature files.

Usage:
  - Default: runs `go test` with cucumber output and updates features.
  - With --input FILE: parse an existing cucumber JSON file instead.

Requirements: go toolchain for running tests, Python 3 to execute this script.
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from typing import Dict, List, Set, Tuple


def ensure_go_available():
    if shutil.which("go") is None:
        print("error: 'go' not found in PATH; install Go to run BDD suite.", file=sys.stderr)
        sys.exit(2)


def run_godog_cucumber(paths: str = "tests/features", tags: str = "") -> str:
    # Execute go test with cucumber format; capture stdout
    cmd = [
        "go", "test", "./...", "-run", "TestFeatures", "-v",
        "args",
        f"--godog.paths={paths}",
        "--godog.format=cucumber",
    ]
    if tags:
        cmd.append(f"--godog.tags={tags}")

    print("Running:", " ".join(cmd))
    p = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    output = p.stdout
    # Attempt to extract the JSON array substring
    start = output.find("[")
    end = output.rfind("]")
    if start == -1 or end == -1 or end < start:
        print("error: could not locate cucumber JSON in output.\n\nOutput:\n" + output, file=sys.stderr)
        sys.exit(3)
    return output[start:end+1]


def parse_failures(cucumber_json: str) -> Dict[str, Set[str]]:
    """
    Returns a mapping: feature_uri -> set of failing scenario names
    """
    data = json.loads(cucumber_json)
    failures: Dict[str, Set[str]] = {}
    for feature in data:
        uri = feature.get("uri") or feature.get("id") or ""
        if not uri:
            continue
        elements = feature.get("elements", [])
        for el in elements:
            if el.get("type") not in ("scenario", "scenario_outline"):  # ignore backgrounds, etc.
                continue
            name = el.get("name", "").strip()
            # Determine failure: any step has result.status == failed
            steps = el.get("steps", [])
            failed = any((s.get("result") or {}).get("status") == "failed" for s in steps)
            if failed and name:
                failures.setdefault(uri, set()).add(name)
    return failures


SCENARIO_RE = re.compile(r"^(?P<indent>\s*)(Scenario(?: Outline)?:)\s*(?P<name>.*)\s*$")
TAGS_RE = re.compile(r"^(?P<indent>\s*)@(?P<tags>.+?)\s*$")


def tag_feature_file(path: str, failing_names: Set[str]) -> int:
    """
    Edit the .feature file in-place, adding @wip to tag line above any failing scenario.
    Returns the number of scenarios tagged in this file.
    """
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except FileNotFoundError:
        return 0

    changed = False
    tagged_count = 0

    i = 0
    while i < len(lines):
        m = SCENARIO_RE.match(lines[i])
        if not m:
            i += 1
            continue
        scen_name = m.group("name").strip()
        if scen_name not in failing_names:
            i += 1
            continue

        # Look back for a tag line immediately above (skip blank comment lines)
        tag_line_idx = None
        j = i - 1
        while j >= 0 and lines[j].strip() == "":
            j -= 1
        if j >= 0 and lines[j].lstrip().startswith("@"):
            tag_line_idx = j

        if tag_line_idx is not None:
            # Append @wip if missing
            line = lines[tag_line_idx].rstrip("\n")
            if "@wip" not in line:
                # Preserve indent and add @wip at end
                if line.strip().endswith(" "):
                    newline = line + "@wip\n"
                else:
                    newline = line + " @wip\n"
                lines[tag_line_idx] = newline
                changed = True
                tagged_count += 1
        else:
            # Insert new tag line above with same indent as scenario
            indent = m.group("indent") or ""
            lines.insert(i, f"{indent}@wip\n")
            changed = True
            tagged_count += 1
            i += 1  # Advance to account for inserted line

        i += 1

    if changed:
        with open(path, "w", encoding="utf-8") as f:
            f.writelines(lines)

    return tagged_count


def main():
    ap = argparse.ArgumentParser(description="Tag failing Godog scenarios with @wip")
    ap.add_argument("--input", help="Path to cucumber JSON. If omitted, runs go test to generate it.")
    ap.add_argument("--paths", default="tests/features", help="Feature paths for test run (default: tests/features)")
    ap.add_argument("--tags", default="", help="Godog tags to include/exclude during failure detection")
    args = ap.parse_args()

    if args.input:
        with open(args.input, "r", encoding="utf-8") as f:
            cucumber_json = f.read()
    else:
        ensure_go_available()
        cucumber_json = run_godog_cucumber(paths=args.paths, tags=args.tags)

    failures = parse_failures(cucumber_json)
    if not failures:
        print("No failing scenarios detected. Nothing to tag.")
        return

    total_tagged = 0
    for uri, names in failures.items():
        # Normalize paths that might be relative to tests dir
        path = uri
        if not os.path.isabs(path) and not os.path.exists(path):
            alt = os.path.join("tests", path)
            if os.path.exists(alt):
                path = alt
        count = tag_feature_file(path, names)
        if count:
            print(f"Tagged {count} scenario(s) in {path}")
        total_tagged += count

    print(f"Done. Tagged {total_tagged} failing scenario(s) with @wip.")


if __name__ == "__main__":
    main()

