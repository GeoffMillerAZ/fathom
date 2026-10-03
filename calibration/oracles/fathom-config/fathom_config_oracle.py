"""fathom_config_oracle: the language-breadth harness's oracle for the
fathom-config doc-engine dialect (design section 1.3 and 18.3; LB-Q-fathom-config).
Tier C, and it never reads fathom's own config loader (internal/config).

    python3 fathom_config_oracle.py -subject <checkout> -out <json> \
        [-cue <cue binary> -schema schema/config/schema.cue -work <scratch dir>]

The decode is Python's tomllib (TOML 1.0's reference-quality parser, not
fathom's go-toml/v2 and not internal/config). What it reads of each
`.fathom/config.toml` is the file's own words, not the schema's meaning, so
the oracle is tier C: it caps every metric at good (section 1.3).

  - definitions: a `section` per top-level table other than `profiles`, a
    `key` per top-level scalar or array (default_profile) and per non-table
    key inside a section ("daemon.state_dir"), a `key` per [profiles.<name>]
    table, and a `rule` per [[surface.pattern]] item, named by its `kind`.
  - references: a key that names a profile - the root `default_profile`,
    `[daemon].default_profile` and `[decide].profile` - to the profile when
    the file declares it ([profiles.<name>]). A name no table declares is a
    dangling name and no reference.
  - lines: tomllib keeps no source positions, so each entity's line and end
    line come from this program's own line scanner (table headers and keys,
    multi-line strings and arrays tracked), never a TOML library.

-cue runs `cue vet -d '#Config' <schema> <decoded.json>` on every file (the
structure check, tier A for shape): the count that fails is printed on stderr;
a file that fails is still listed, because the dialect reads whatever the file
holds.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never
a product dependency.
"""

import argparse
import json
import os
import re
import subprocess
import sys
import tomllib

TOOL = "python tomllib (TOML decode) + a line scanner of its own"
SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}

_part = re.compile(r'"((?:[^"\\]|\\.)*)"|\'([^\']*)\'|([A-Za-z0-9_-]+)')


def _parts(dotted):
    return tuple(a or b or c for a, b, c in _part.findall(dotted))


def scan(src):
    """Positions from the text alone: header lines, key lines and the last
    line of each key's value. Returns (headers, keys) where headers is a list
    of (path tuple, is_array, line) and keys a list of (table path tuple,
    key path tuple, first line, last line)."""
    headers, keys = [], []
    table = ()
    lines = src.split("\n")
    i = 0
    while i < len(lines):
        raw = lines[i]
        s = raw.strip()
        if s == "" or s.startswith("#"):
            i += 1
            continue
        if s.startswith("["):
            arr = s.startswith("[[")
            inner = s[2:] if arr else s[1:]
            close = inner.find("]]" if arr else "]")
            if close >= 0:
                table = _parts(inner[:close])
                headers.append((table, arr, i + 1))
            i += 1
            continue
        m = re.match(r'^((?:"(?:[^"\\]|\\.)*"|\'[^\']*\'|[A-Za-z0-9_-]+)(?:\s*\.\s*(?:"(?:[^"\\]|\\.)*"|\'[^\']*\'|[A-Za-z0-9_-]+))*)\s*=\s*(.*)$', s)
        if not m:
            i += 1
            continue
        key = _parts(m.group(1))
        rest = m.group(2)
        first = i + 1
        depth, in_ml = 0, None
        j = i
        while True:
            depth, in_ml = _advance(rest, depth, in_ml)
            if depth <= 0 and in_ml is None:
                break
            j += 1
            if j >= len(lines):
                break
            rest = lines[j]
        keys.append((table, key, first, j + 1))
        i = j + 1
    return headers, keys


def _advance(text, depth, in_ml):
    """Bracket depth and open multi-line string after text."""
    i = 0
    while i < len(text):
        if in_ml:
            end = text.find(in_ml, i)
            if end < 0:
                return depth, in_ml
            i = end + 3
            in_ml = None
            continue
        c = text[i]
        if text.startswith('"""', i) or text.startswith("'''", i):
            in_ml = text[i:i + 3]
            i += 3
            continue
        if c == '"':
            j = i + 1
            while j < len(text) and text[j] != '"':
                j += 2 if text[j] == "\\" else 1
            i = j + 1
            continue
        if c == "'":
            j = text.find("'", i + 1)
            i = (j if j >= 0 else len(text)) + 1
            continue
        if c == "#":
            break
        if c in "[{":
            depth += 1
        elif c in "]}":
            depth -= 1
        i += 1
    return depth, in_ml


class Lines:
    """line and end line of a table path or a dotted key path."""

    def __init__(self, src):
        self.headers, self.keys = scan(src)

    def table_span(self, path):
        """First and last line written under (or at) a table path."""
        lo = hi = 0
        for p, _arr, line in self.headers:
            if p[:len(path)] == path:
                lo = line if lo == 0 else min(lo, line)
                hi = max(hi, line)
        for t, k, first, last in self.keys:
            if (t + k)[:len(path)] == path and len(t + k) >= len(path):
                lo = first if lo == 0 else min(lo, first)
                hi = max(hi, last)
        return lo, hi

    def key_span(self, path):
        for t, k, first, last in self.keys:
            if t + k == path:
                return first, last
        return self.table_span(path)

    def array_header(self, path, n):
        k = 0
        for p, arr, line in self.headers:
            if arr and p == path:
                if k == n:
                    return line
                k += 1
        return 0

    def array_span(self, path, n):
        """An [[a.b]] item: its header to the line before the next header."""
        start = self.array_header(path, n)
        if not start:
            return 0, 0
        end = start
        nxt = [line for _p, _a, line in self.headers if line > start]
        stop = min(nxt) if nxt else 10 ** 9
        for _t, _k, first, last in self.keys:
            if start < first < stop:
                end = max(end, last)
        return start, end


def entities(path, doc, src):
    """(definitions, references) of one decoded document."""
    ln = Lines(src)
    defs, refs = [], []

    def add(name, kind, lo, hi):
        defs.append({"path": path, "name": name, "line": lo, "end": hi, "kind": kind})

    profiles = doc.get("profiles")
    declared = set(profiles) if isinstance(profiles, dict) else set()
    for key, val in doc.items():
        if isinstance(val, dict):
            if key == "profiles":
                for pname, pval in val.items():
                    if isinstance(pval, dict):
                        lo, hi = ln.table_span(("profiles", pname))
                        add(pname, "key", lo, hi)
                continue
            lo, hi = ln.table_span((key,))
            add(key, "section", lo, hi)
            for sk, sv in val.items():
                if isinstance(sv, dict):
                    continue
                klo, khi = ln.key_span((key, sk))
                if key == "surface" and sk == "pattern" and isinstance(sv, list):
                    klo, khi = ln.table_span(("surface", "pattern"))
                add(key + "." + sk, "key", klo, khi)
            if key == "surface" and isinstance(val.get("pattern"), list):
                for n, item in enumerate(val["pattern"]):
                    if isinstance(item, dict) and isinstance(item.get("kind"), str) and item["kind"]:
                        slo, shi = ln.array_span(("surface", "pattern"), n)
                        add(item["kind"], "rule", slo, shi)
            named = val.get("default_profile") if key == "daemon" else val.get("profile") if key == "decide" else None
            if isinstance(named, str) and named in declared:
                refs.append({"path": path, "from": key, "to": named})
        elif isinstance(val, list):
            continue
        else:
            lo, hi = ln.key_span((key,))
            add(key, "key", lo, hi)
            if key == "default_profile" and isinstance(val, str) and val in declared:
                refs.append({"path": path, "from": "default_profile", "to": val})
    return defs, refs


def walk(subject):
    for root, dirs, files in os.walk(subject):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        if os.path.basename(root) == ".fathom" and "config.toml" in files:
            yield os.path.join(root, "config.toml")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-cue", default="")
    ap.add_argument("-schema", default="schema/config/schema.cue")
    ap.add_argument("-work", default="")
    a = ap.parse_args()
    subject = os.path.abspath(a.subject)
    defs, refs = [], []
    n_files = n_bad = vet_ok = vet_bad = 0
    for full in sorted(walk(subject)):
        rel = os.path.relpath(full, subject).replace(os.sep, "/")
        src = open(full, encoding="utf-8").read()
        try:
            doc = tomllib.loads(src)
        except tomllib.TOMLDecodeError as e:
            n_bad += 1
            print("not TOML: %s: %s" % (rel, e), file=sys.stderr)
            continue
        n_files += 1
        d, r = entities(rel, doc, src)
        defs += d
        refs += r
        if a.cue:
            tmp = os.path.join(a.work or os.path.dirname(os.path.abspath(a.out)), "vet-%d.json" % n_files)
            json.dump(doc, open(tmp, "w"))
            p = subprocess.run([a.cue, "vet", "-d", "#Config", a.schema, tmp], capture_output=True, text=True)
            if p.returncode == 0:
                vet_ok += 1
            else:
                vet_bad += 1
                print("cue vet #Config failed: %s: %s" % (rel, p.stderr.strip().splitlines()[0] if p.stderr.strip() else ""), file=sys.stderr)
            os.remove(tmp)
    out = {"tool": TOOL, "version": "python " + sys.version.split()[0] + " tomllib", "tier": "C", "definitions": defs, "references": refs}
    json.dump(out, open(a.out, "w"), indent=1)
    print("fathom-config oracle: %d files (%d not TOML), %d definitions, %d references%s" % (
        n_files, n_bad, len(defs), len(refs),
        (", cue vet #Config: %d valid, %d failed" % (vet_ok, vet_bad)) if a.cue else ""), file=sys.stderr)


if __name__ == "__main__":
    main()
