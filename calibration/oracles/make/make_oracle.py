#!/usr/bin/env python3
"""make_oracle.py: the L2 (prerequisite edges) oracle for the makefile dialect.

Writes indexbench lang-oracle's neutral oracle JSON to stdout:
  {"tool": "make -pn", "version": ..., "tier": "A",
   "references": [{"path": <repo-relative>, "from": <target as spelled>, "to": <prerequisite as spelled>}]}

`references`, not `calls`: a tool format's L2 is reference recall and precision (design section 1.2).

Tier A: every pair is backed by an edge in GNU make's own rule database (`make -pn`
evaluates variables, functions, conditionals and includes (built-in rules are off, -r), and prints each rule's
prerequisites). make prints them expanded and does not say which file a rule came from
(only a rule with a recipe carries a location), so the pairs are put back in the form
the source spells them, file by file:

  1. Each Makefile-like file (Makefile, makefile, GNUmakefile, *.mk, *.make) that no earlier
     run has read is evaluated as its own entry point (`make -pn -f FILE`, in the file's
     directory, then in the subject root). MAKEFILE_LIST, which make prints, names every
     file that run read, so an included file is measured in its includer's context.
  2. Each file is scanned for rule lines (`targets: prerequisites`, continuation lines joined,
     recipe lines, comments and define blocks skipped; special targets (.PHONY ...), static
     pattern rules and target-specific assignments are not rule lines). A spelled pair
     (target, prerequisite) is kept when the run's database holds the edge:
       - a prerequisite must be spelled literally (no `$`); one written with a variable or a
         function is a value, not a name, and is left out of the truth (as it is out of the
         dialect's references);
       - a target spelled with variables (`$(BUILD)/main.o`) matches every database target of
         that shape (each `$(..)` is `.*`) that lists the prerequisite.
     Only files the run read are matched, so an identical rule line in a file make never read
     is not credited.
  3. A pair on a rule line the database does not confirm (a dead conditional branch, a rule
     make dropped) is not in the truth: the dialect's collection of it counts as a precision
     miss, which is what a static extractor cannot avoid.

The summary on stderr counts what could not be aligned, so a reading can disclose it: database
edges with no spelled literal counterpart (produced by variable expansion, wildcard, pattern
rules), files make could not evaluate, and spelled pairs the database does not confirm.

Run in a disposable clone: `make -pn` executes $(shell ...) and the recipes that remake included
makefiles. The run is confined by sandbox-exec (no network; writes only inside the subject and
the temp dirs) when it is available.

A subject whose makefiles branch on a knob the host cannot satisfy in one run (an architecture,
a platform) is evaluated once per `-variant "VAR=value ..."` as well (extra command-line
assignments for make); the truth is the union of what each evaluation confirmed, so a rule line
live in any evaluated configuration is not counted against a static extractor.

Usage: make_oracle.py -subject <checkout> [-make make] [-timeout 120] [-variant "VAR=value ..."]... > oracle.json
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys

NAMES = {"Makefile", "makefile", "GNUmakefile"}
SUFFIXES = (".mk", ".make")
SPECIAL = re.compile(r"^\.[A-Z_]+$")
DIRECTIVE = re.compile(
    r"^\s*(-?include|sinclude|ifeq|ifneq|ifdef|ifndef|else|endif|export|unexport|override|private|"
    r"vpath|undefine|define|endef|load|-load)\b")
ASSIGN_AFTER_COLON = re.compile(
    r"^\s*(?:(?:override|export|private)\s+)*[^\s=:]+\s*(?:[:+?!]|::|:::)?=")


def is_makefile(name):
    return name in NAMES or name.endswith(SUFFIXES)


def find_files(root):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x != ".git")
        for f in sorted(files):
            p = os.path.join(d, f)
            if is_makefile(f) and not os.path.islink(p):
                out.append(os.path.relpath(p, root).replace(os.sep, "/"))
    return sorted(out, key=lambda p: (p.count("/"), p))


def split_top(s, sep_ws=True):
    """Split s on whitespace outside $(...) / ${...} nesting."""
    toks, cur, depth, i = [], [], 0, 0
    while i < len(s):
        c = s[i]
        if depth == 0 and c == "$" and i + 1 < len(s) and s[i + 1] in "({":
            depth += 1
            cur.append(c)
            cur.append(s[i + 1])
            i += 2
            continue
        if depth > 0 and c in "({":
            depth += 1
        elif depth > 0 and c in ")}":
            depth -= 1
        if depth == 0 and c.isspace():
            if cur:
                toks.append("".join(cur))
                cur = []
        else:
            cur.append(c)
        i += 1
    if cur:
        toks.append("".join(cur))
    return toks


def top_positions(s, ch):
    """Indexes of ch outside $(...) / ${...} nesting."""
    pos, depth, i = [], 0, 0
    while i < len(s):
        c = s[i]
        if c == "$" and i + 1 < len(s) and s[i + 1] in "({":
            depth += 1
            i += 2
            continue
        if depth > 0 and c in "({":
            depth += 1
        elif depth > 0 and c in ")}":
            depth -= 1
        elif depth == 0 and c == ch:
            pos.append(i)
        i += 1
    return pos


def logical_lines(text):
    """Yield (first line number, joined text) of non-recipe, non-comment lines outside define blocks."""
    lines = text.split("\n")
    i, n, defs = 0, len(lines), 0
    while i < n:
        raw, start = lines[i], i + 1
        i += 1
        joined = raw
        while re.search(r"(?<!\\)(?:\\\\)*\\$", joined) and i < n:
            joined = joined[:-1].rstrip() + " " + lines[i].lstrip()
            i += 1
        if raw.startswith("\t"):
            continue
        s = joined.strip()
        if not s or s.startswith("#"):
            continue
        m = re.match(r"^(?:(?:override|export|private)\s+)*(define|endef)\b", s)
        if m:
            defs += 1 if m.group(1) == "define" else -1
            defs = max(defs, 0)
            continue
        if defs:
            continue
        yield start, joined


def parse_rule(joined):
    """Return (targets, prereqs) for a rule line, else None."""
    if DIRECTIVE.match(joined):
        return None
    s = joined
    colons = top_positions(s, ":")
    eqs = top_positions(s, "=")
    rule_colon = None
    for c in colons:
        if s[c:c + 2] == ":=" or s[c:c + 3] == "::=" or s[c:c + 4] == ":::=":
            return None  # a := assignment
        rule_colon = c
        break
    if rule_colon is None:
        return None
    if any(e < rule_colon for e in eqs):
        return None  # VAR = a: b
    tgt, rest = s[:rule_colon], s[rule_colon + 1:]
    if rest.startswith(":"):
        rest = rest[1:]
    tgt = tgt.rstrip("&").rstrip()
    if ASSIGN_AFTER_COLON.match(rest):
        return None  # target-specific variable
    semis = top_positions(rest, ";")
    if semis:
        rest = rest[:semis[0]]
    if top_positions(rest, ":"):
        return None  # static pattern rule
    targets = split_top(tgt)
    if not targets or any(SPECIAL.match(t) for t in targets):
        return None
    prereqs = [t for t in split_top(rest) if t != "|"]
    return targets, prereqs


def shape_regex(tok):
    """Regex for a target spelled with variables: each $(..) / ${..} matches anything."""
    out, i = [], 0
    while i < len(tok):
        if tok[i] == "$" and i + 1 < len(tok) and tok[i + 1] in "({":
            depth, i = 1, i + 2
            while i < len(tok) and depth:
                if tok[i] in "({":
                    depth += 1
                elif tok[i] in ")}":
                    depth -= 1
                i += 1
            out.append(".*")
        else:
            out.append(re.escape(tok[i]))
            i += 1
    return re.compile("".join(out), re.S)


def sandbox_prefix(root):
    if not shutil.which("sandbox-exec"):
        return []
    root = os.path.realpath(root)
    prof = ('(version 1)(allow default)(deny network*)(deny file-write*)'
            '(allow file-write* (subpath "%s") (subpath "/dev") (subpath "/private/tmp") '
            '(subpath "/private/var/folders"))' % root)
    return ["sandbox-exec", "-p", prof]


def run_make(make, root, entry, cwd, timeout, extra=()):
    cmd = sandbox_prefix(root) + [make, "-pnr", "-f", os.path.join(root, entry)] + list(extra)
    env = dict(os.environ, LC_ALL="C")
    for k in ("MAKEFLAGS", "MAKEFILES", "MFLAGS"):
        env.pop(k, None)
    try:
        p = subprocess.run(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                           timeout=timeout, stdin=subprocess.DEVNULL)
    except subprocess.TimeoutExpired:
        return None
    out = p.stdout.decode("utf-8", "replace")
    return out if "\n# Files\n" in out else None


def parse_db(out, cwd):
    """Return (edges {target: set(prereqs)}, files read by the run as absolute paths)."""
    edges, lst = {}, []
    lines = out.split("\n")
    in_rules = False
    prev = ""
    for line in lines:
        m = re.match(r"^MAKEFILE_LIST\s*:?=\s*(.*)$", line)
        if m:
            lst = m.group(1).split()
        if line.startswith("# Implicit Rules"):
            in_rules = True
        if in_rules and line and line[0] not in "#\t " and ":" in line and prev != "# Not a target:":
            t, _, rest = line.partition(":")
            rest = rest.lstrip(":")
            if SPECIAL.match(t.strip()):
                prev = line
                continue
            if "=" in rest.split("|")[0][:2] or t.endswith((":", "=")):
                prev = line
                continue
            edges.setdefault(t.strip(), set()).update(x for x in rest.split() if x != "|")
        prev = line
    files = set()
    for f in lst:
        files.add(os.path.realpath(f if os.path.isabs(f) else os.path.join(cwd, f)))
    return edges, files


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-make", default="make")
    ap.add_argument("-timeout", type=int, default=120)
    ap.add_argument("-variant", action="append", default=[], help="extra make arguments for one more evaluation")
    a = ap.parse_args()
    root = os.path.realpath(a.subject)
    ver = subprocess.run([a.make, "--version"], stdout=subprocess.PIPE, text=True).stdout.split("\n")[0]
    version = ver.replace("GNU Make", "").strip()
    files = find_files(root)
    abs_of = {os.path.realpath(os.path.join(root, f)): f for f in files}
    rules = {}
    for f in files:
        try:
            text = open(os.path.join(root, f), encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        rl = []
        for ln, joined in logical_lines(text):
            r = parse_rule(joined)
            if r:
                rl.append((ln, r[0], r[1]))
        rules[f] = rl
    failed, runs, pairs = [], 0, set()
    st = dict(db_edges=0, aligned=0, spelled_pairs=0, spelled_unconfirmed=0)

    def evaluate(extra, label):
        nonlocal runs
        covered = set()
        for f in files:
            if os.path.realpath(os.path.join(root, f)) in covered:
                continue
            out = None
            for cwd in (os.path.dirname(os.path.join(root, f)), root):
                out = run_make(a.make, root, f, cwd, a.timeout, extra)
                if out is not None:
                    break
            runs += 1
            if out is None:
                failed.append(f + label)
                continue
            edges, read = parse_db(out, cwd)
            in_subject = {abs_of[p] for p in read if p in abs_of}
            covered |= read
            all_x = list(edges)
            st["db_edges"] += sum(len(v) for v in edges.values())
            aligned_db = set()
            for rf in sorted(in_subject):
                for ln, targets, prereqs in rules.get(rf, []):
                    for t in targets:
                        xs = None
                        for p in prereqs:
                            if "$" in p:
                                continue
                            st["spelled_pairs"] += 1
                            if "$" not in t:
                                hit = [t] if p in edges.get(t, ()) else []
                            else:
                                if xs is None:
                                    rx = shape_regex(t)
                                    xs = [x for x in all_x if rx.fullmatch(x)]
                                hit = [x for x in xs if p in edges[x]]
                            if hit:
                                pairs.add((rf, t, p))
                                aligned_db.update((x, p) for x in hit)
                            else:
                                st["spelled_unconfirmed"] += 1
            st["aligned"] += len(aligned_db)

    evaluate([], "")
    for v in a.variant:
        evaluate(v.split(), " [%s]" % v)
    calls = [{"path": p, "from": t, "to": q} for p, t, q in sorted(pairs)]
    json.dump({"tool": "make -pn", "version": version, "tier": "A", "references": calls}, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write(
        "make_oracle: %d files, %d entry runs, %d files make could not evaluate, %d truth pairs; "
        "database edges %d (aligned to a spelled literal %d, expanded-only %d); spelled literal pairs "
        "%d, unconfirmed by the database %d\n"
        % (len(files), runs, len(failed), len(calls), st["db_edges"], st["aligned"],
           st["db_edges"] - st["aligned"], st["spelled_pairs"], st["spelled_unconfirmed"]))
    for f in failed:
        sys.stderr.write("make_oracle: not evaluated: %s\n" % f)


if __name__ == "__main__":
    main()
