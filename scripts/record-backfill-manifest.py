#!/usr/bin/env python3
"""Generate lexical title candidates, never permission to backfill plans.

Uses Python's stdlib only. Output must be an explicit, new workspace scratch
file. Native-parser correspondence remains required before any actual backfill.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re


BASE = "468dc705813382d584926887b451cbea145c76ac"
HEADER = (
    "# REC-8 frozen lexical title-candidate manifest\n\n"
    "Contract companion to records-integration.md. This is a concrete immutable write\n"
    "allowlist, not native-parser verification or permission to edit plans now. The\n"
    "implementing lane must prove exact correspondence with existing plan parser before\n"
    "writing; mismatch refuses rather than broadening the list. Only top-level plans,\n"
    "README/archive excluded. Source hashes bind every candidate to the stated base.\n\n"
    "~~~json\n"
)


def unquoted_chars(text):
    """Mirror markdown.go's escapes rule for delimiter scanning."""
    quoted = False
    i = 0
    while i < len(text):
        char = text[i]
        if char == "\\" and i + 1 < len(text) and text[i + 1] in ('"', "\\"):
            i += 2
            continue
        if char == '"':
            quoted = not quoted
        elif not quoted:
            yield i, char
        i += 1
    if quoted:
        raise ValueError("unclosed quote")


def extract_row_id(line):
    """Read the annotation's id field, refusing ambiguous or missing metadata.

    Follow the native parser's trailing balanced brace block and quote-aware
    comma rules. Be stricter about malformed/duplicate fields rather than guess.
    """
    stack = []
    pairs = []
    for i, char in unquoted_chars(line):
        if char == "{":
            stack.append(i)
        elif char == "}":
            if not stack:
                raise ValueError("unmatched closing brace")
            pairs.append((stack.pop(), i))
    if stack or not pairs:
        raise ValueError("missing or unbalanced annotation")
    start, end = pairs[-1]
    block = line[start + 1:end]
    cuts = [-1] + [i for i, c in unquoted_chars(block) if c == ","] + [len(block)]
    fields = {}
    for left, right in zip(cuts, cuts[1:]):
        key, sep, value = block[left + 1:right].partition(":")
        key, value = key.strip(), value.strip()
        if not sep or not key or not value or key in fields:
            raise ValueError("missing or duplicate annotation field")
        fields[key] = value
    value = fields.get("id", "")
    if value.startswith('"') and value.endswith('"'):
        value = value[1:-1]
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", value):
        raise ValueError("missing or invalid annotation id")
    return value


def generate(root, base=BASE):
    entries, files = [], {}
    for path in sorted((root / "docs/plans").glob("*.md")):
        if path.name == "README.md":
            continue
        raw = path.read_bytes()
        for number, line in enumerate(raw.decode().splitlines(), 1):
            match = re.match(r"^[-*] \[[ xX~-]\] (.*)", line)
            if not match:
                continue
            # Preserve the original candidate selection and byte hashes.
            title = match.group(1).split("{", 1)[0]
            gate = title.startswith("(op) ")
            lane = re.search(r"(?<!`)\(lane:\s*([A-Za-z0-9][A-Za-z0-9-]*)\)(?!`)", title)
            if not gate and not lane:
                continue
            rel = path.relative_to(root).as_posix()
            try:
                row_id = extract_row_id(line)
            except ValueError as error:
                raise ValueError(f"{rel}:{number}: {error}") from error
            files[rel] = hashlib.sha256(raw).hexdigest()
            entries.append(dict(path=rel, line=number, row_id=row_id,
                                before_sha256=hashlib.sha256(line.encode()).hexdigest(),
                                gate=gate, lane=lane.group(1) if lane else ""))
    if not entries:
        raise ValueError("no title candidates found")
    return dict(schema="fathom.record-title-backfill/v1", base=base, files=files, rows=entries)


def render(manifest):
    return HEADER + json.dumps(manifest, indent=2) + "\n~~~\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--base", default=BASE)
    parser.add_argument("--output", type=Path, required=True,
                        help="new file under source root's workspace/tmp or workspace/artifacts")
    args = parser.parse_args()
    root, output = args.root.resolve(), args.output.resolve()
    scratch = [root / "workspace" / name for name in ("tmp", "artifacts")]
    if not any(directory in output.parents for directory in scratch):
        parser.error("output must be a new workspace scratch file")
    try:
        manifest = generate(root, args.base)
        with output.open("x", encoding="utf-8", newline="\n") as target:
            target.write(render(manifest))
    except (OSError, ValueError) as error:
        parser.error(str(error))
    print(f"frozen lexical candidates {len(manifest['rows'])} in {len(manifest['files'])} "
          "exact plan files; native-parser parity unrun")


if __name__ == "__main__":
    main()
