#!/usr/bin/env python3
"""bazel_oracle.py -- tier-A oracle for Bazel (LB-Q-bazel): `bazel query` read as `indexbench lang-oracle`'s neutral oracle file.

Python 3 standard library only, bench-only, never a product dependency. Bazel's own loading phase is the witness:

    bazel query //... --output=streamed_jsonproto --proto:instantiation_stack --proto:definition_stack --noimplicit_deps

For each rule the query lists, `location` and `instantiation_stack` name the BUILD call that created it (a macro's
rules share that call) and the .bzl frames between the call and `native.<rule>()`; `definition_stack` names the .bzl
statement that called `rule()` for a Starlark rule. The source text is read only to find what Bazel's evaluation cannot
say: where a call ends and which strings the call spells.

definitions      the BUILD call whose literal `name = "x"` is a rule Bazel instantiated, or the generator_name Bazel
                 records on the rules a macro call created: (BUILD file, x, line of the
                 name literal, line of the call's closing parenthesis, kind target)
references       the rule or macro a call names, (BUILD file, x, callee), for each call Bazel instantiated rules from;
                 and a label the call spells as ":y" that is an input of one of those rules: (BUILD file, x, y).
                 Bazel says the label is a real input of the package; the text says how it was spelled
cross_references (BUILD or .bzl file, the target or function making the call) to (the .bzl file defining what it
                 calls), from consecutive frames of the instantiation stack, and from the last frame to the
                 definition stack's first frame (the `rule()` call). File granularity: `to` is empty

Usage:
    bazel_oracle.py -subject <workspace dir> -output-user-root <scratch dir> [-bazel <path>] > oracle.json
    bazel_oracle.py -subject <workspace dir> -scope-globs        # the files `//...` can reach, as lang-oracle -only globs

`//...` does not reach a directory `.bazelignore` names or one holding a MODULE.bazel, WORKSPACE, WORKSPACE.bazel or
REPO.bazel of its own (a nested repository), so the oracle says nothing about the BUILD files there. -scope-globs
prints the subject's files outside those directories as comma-separated globs for `indexbench lang-oracle -only`.
"""
import json
import os
import re
import subprocess
import sys

FRAME = re.compile(r"^(.*?):(\d+):(\d+): (.*)$")


def parse_args(argv):
    a = {"subject": None, "root": None, "bazel": "bazel", "extra": [], "scope": False}
    i = 1
    while i < len(argv):
        k = argv[i]
        if k == "-subject":
            a["subject"] = argv[i + 1]; i += 2
        elif k == "-output-user-root":
            a["root"] = argv[i + 1]; i += 2
        elif k == "-bazel":
            a["bazel"] = argv[i + 1]; i += 2
        elif k == "-scope-globs":
            a["scope"] = True; i += 1
        elif k == "-flag":
            a["extra"].append(argv[i + 1]); i += 2
        else:
            sys.exit("bazel_oracle: unknown argument " + k)
    if not a["subject"] or (not a["root"] and not a["scope"]):
        sys.exit("usage: bazel_oracle.py -subject <workspace dir> -output-user-root <dir> [-bazel path] [-flag f]...")
    return a


def scan_call(src, line, col):
    """The text of the call whose opening parenthesis is at (line, col), 1-based; returns (start, end) offsets of
    the parentheses, or None. Strings (single, double, triple, raw) and comments are skipped."""
    lines = src.split("\n")
    if line < 1 or line > len(lines):
        return None
    off = sum(len(l) + 1 for l in lines[: line - 1])
    start = off + col - 1
    if start >= len(src) or src[start] != "(":
        # Bazel reports the call's callee start or its parenthesis depending on the frame; find the next one.
        p = src.find("(", off + max(col - 1, 0))
        if p < 0:
            return None
        start = p
    depth, i, n = 0, start, len(src)
    while i < n:
        c = src[i]
        if c == "#":
            while i < n and src[i] != "\n":
                i += 1
            continue
        if c in "\"'":
            q = c * 3 if src.startswith(c * 3, i) else c
            raw = i > 0 and src[i - 1] in "rR"
            i += len(q)
            while i < n and not src.startswith(q, i):
                i += 2 if src[i] == "\\" and not raw else 1
            i += len(q)
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
            if depth == 0:
                return start, i
        i += 1
    return None


STR = re.compile(r'"((?:[^"\\\n]|\\.)*)"|\'((?:[^\'\\\n]|\\.)*)\'')


def strings_of(text, base):
    """String literals (single-line, quoted with ' or ") of text outside comments, as (value, offset)."""
    out, i, n = [], 0, len(text)
    while i < n:
        c = text[i]
        if c == "#":
            while i < n and text[i] != "\n":
                i += 1
            continue
        if c in "\"'":
            if text.startswith(c * 3, i):
                j = text.find(c * 3, i + 3)
                i = n if j < 0 else j + 3
                continue
            m = STR.match(text, i)
            if m:
                out.append((m.group(1) if m.group(1) is not None else m.group(2), base + i))
                i = m.end()
                continue
        i += 1
    return out


def name_literal(text):
    """The depth-1 `name = "x"` keyword argument of a call's text (parentheses included): (value, offset) or None."""
    depth, i, n = 0, 0, len(text)
    while i < n:
        c = text[i]
        if c == "#":
            while i < n and text[i] != "\n":
                i += 1
            continue
        if c in "\"'":
            q = c * 3 if text.startswith(c * 3, i) else c
            i += len(q)
            while i < n and not text.startswith(q, i):
                i += 2 if text[i] == "\\" else 1
            i += len(q)
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif depth == 1 and (text[i - 1] in "(, \t\n") and text.startswith("name", i):
            m = re.compile(r'name\s*=\s*(?!=)\s*("([^"\\\n]*)"|\'([^\'\\\n]*)\')').match(text, i)
            if m:
                v = m.group(2) if m.group(2) is not None else m.group(3)
                return v, m.start(1)
        i += 1
    return None


BOUNDARY = ("MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel", "REPO.bazel")


def scope_globs(subject):
    """Globs of the subject's files that `bazel query //...` can reach (see the module doc)."""
    ignored = set()
    try:
        with open(os.path.join(subject, ".bazelignore"), encoding="utf-8") as f:
            for line in f:
                line = line.split("#", 1)[0].strip().strip("/")
                if line:
                    ignored.add(os.path.normpath(line).replace(os.sep, "/"))
    except OSError:
        pass
    excluded = set(ignored)
    for dirpath, dirs, files in os.walk(subject):
        dirs[:] = sorted(d for d in dirs if d != ".git")
        rel = os.path.relpath(dirpath, subject).replace(os.sep, "/")
        if rel != "." and any(b in files for b in BOUNDARY):
            excluded.add(rel)
    globs = []

    def clean(rel):
        return not any(e == rel or e.startswith(rel + "/") for e in excluded)

    def expand(rel):
        base = os.path.join(subject, rel) if rel else subject
        files = False
        for e in sorted(os.listdir(base)):
            p = (rel + "/" + e) if rel else e
            if e == ".git" or p in excluded:
                continue
            if os.path.isdir(os.path.join(base, e)) and not os.path.islink(os.path.join(base, e)):
                if clean(p):
                    globs.append(p + "/**")
                else:
                    expand(p)
            else:
                files = True
        if files:
            globs.append((rel + "/*") if rel else "*")

    expand("")
    return globs, sorted(excluded)


def main():
    a = parse_args(sys.argv)
    subject = os.path.realpath(a["subject"])
    if a["scope"]:
        globs, ex = scope_globs(subject)
        sys.stdout.write(",".join(globs) + "\n")
        sys.stderr.write("bazel_oracle: %d globs; outside //...: %s\n" % (len(globs), ", ".join(ex)))
        return
    cmd = [a["bazel"], "--output_user_root=" + a["root"], "query", "//...", "--output=streamed_jsonproto",
           "--proto:instantiation_stack", "--proto:definition_stack", "--noimplicit_deps", "--keep_going"] + a["extra"]
    p = subprocess.run(cmd, cwd=subject, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    sys.stderr.write("\n".join(p.stderr.strip().split("\n")[-6:]) + "\n")
    ver = subprocess.run([a["bazel"], "--version"], cwd=subject, stdout=subprocess.PIPE, text=True).stdout.strip()
    rules = []
    for l in p.stdout.splitlines():
        if not l.strip():
            continue
        r = json.loads(l)
        if r.get("type") == "RULE":
            rules.append(r["rule"])

    def rel(path):
        path = path.replace("\\", "/")
        if os.path.isabs(path):
            try:
                return os.path.relpath(path, subject).replace(os.sep, "/")
            except ValueError:
                return path
        return path

    def parse_frame(s):
        m = FRAME.match(s)
        return (rel(m.group(1)), int(m.group(2)), int(m.group(3)), m.group(4)) if m else None

    def exists(path):
        if path.startswith("@") or path.startswith("..") or os.path.isabs(path):
            return False
        return os.path.isfile(os.path.join(subject, path))

    sites = {}
    for r in rules:
        st = [parse_frame(f) for f in r.get("instantiationStack", [])]
        st = [f for f in st if f]
        if not st:
            continue
        f0 = st[0]
        if os.path.basename(f0[0]) not in ("BUILD", "BUILD.bazel") or not exists(f0[0]):
            continue
        sites.setdefault((f0[0], f0[1], f0[2]), []).append((r, st))

    srcs = {}
    defs, refs, xrefs = set(), set(), set()
    drop = {"sites": 0, "name_computed": 0, "call_unreadable": 0, "label_not_input": 0, "xref_external": 0, "ref_dup": 0}
    for (path, line, col), rs in sorted(sites.items()):
        drop["sites"] += 1
        src = srcs.setdefault(path, open(os.path.join(subject, path), encoding="utf-8", errors="replace").read())
        span = scan_call(src, line, col)
        if not span:
            drop["call_unreadable"] += 1
            continue
        s, e = span
        text = src[s : e + 1]
        pkg = os.path.dirname(path)
        names = {r["name"].split(":", 1)[1] for r, _ in rs if ":" in r["name"]}
        for r, _ in rs:
            # a macro's own name: Bazel records it on every rule the macro creates, whether or not one is named so
            for at in r.get("attribute", []):
                if at.get("name") == "generator_name" and at.get("stringValue"):
                    names.add(at["stringValue"])
        lit = name_literal(text)
        if not lit or lit[0] not in names:
            drop["name_computed"] += 1
            continue
        name, noff = lit
        nline = src.count("\n", 0, s + noff) + 1
        eline = src.count("\n", 0, e) + 1
        defs.add((path, name, nline, eline, "target"))
        callee = re.search(r"([A-Za-z_][A-Za-z0-9_]*)\s*$", src[:s])
        if callee:
            refs.add((path, name, callee.group(1)))  # the rule or macro the call names: Bazel instantiated it
        inputs = set()
        for r, _ in rs:
            for lab in r.get("ruleInput", []):
                if lab.startswith("//" + pkg + ":") or (pkg == "" and lab.startswith("//:")):
                    inputs.add(lab.split(":", 1)[1])
        for v, _ in strings_of(text, s):
            if v.startswith(":") and len(v) > 1:
                if v[1:] in inputs:
                    refs.add((path, name, v[1:]))
                else:
                    drop["label_not_input"] += 1
        for r, st in rs:
            chain = []
            for k in range(len(st) - 1):
                chain.append((st[k][0], name if k == 0 else st[k][3], st[k + 1][0]))
            ds = [parse_frame(f) for f in r.get("definitionStack", [])]
            ds = [f for f in ds if f]
            if ds:
                chain.append((st[-1][0], name if len(st) == 1 else st[-1][3], ds[0][0]))
            for frm_file, frm, to_file in chain:
                if frm_file == to_file:
                    continue
                if not exists(frm_file) or not exists(to_file):
                    drop["xref_external"] += 1
                    continue
                xrefs.add((frm_file, frm, to_file))

    out = {
        "tool": "bazel query --output=streamed_jsonproto",
        "version": ver,
        "tier": "A",
        "definitions": [{"path": p_, "name": n, "line": l, "end": e_, "kind": k} for p_, n, l, e_, k in sorted(defs)],
        "references": [{"path": p_, "from": f, "to": t} for p_, f, t in sorted(refs)],
        "cross_references": [{"path": p_, "from": f, "to_path": t} for p_, f, t in sorted(xrefs)],
    }
    json.dump(out, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write("bazel_oracle: %d rules in %d BUILD call sites; %d definitions, %d references, %d cross_references; dropped %s\n"
                     % (len(rules), len(sites), len(defs), len(refs), len(xrefs), json.dumps(drop, sort_keys=True)))
    if p.returncode not in (0, 3):
        sys.stderr.write("bazel_oracle: bazel exited %d (3 = some packages did not load, --keep_going)\n" % p.returncode)


if __name__ == "__main__":
    main()
