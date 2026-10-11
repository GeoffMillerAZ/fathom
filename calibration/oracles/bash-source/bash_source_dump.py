#!/usr/bin/env python3
"""bash_source_dump.py: Bash `source` and `.` statements resolved by bash's lookup rule (tier B).

Bash has no SCIP producer and no static tool that prints the file a `source` opens. This script
writes the neutral `imports` `indexbench lang-oracle -tool-oracle` reads (LB-F40, scored as
import_resolution and, with no SCIP index, as the file-edge truth, LB-F57):

- Statements: shfmt `--to-json` (mvdan.cc/sh/v3, an independent Go parser of Bash, `-ln bash`)
  gives every simple command; one whose command word is `source` or `.` after quote removal
  (`\\.` is `.`), or `builtin`/`command` followed by one of them, is a source statement, and its
  first argument is the file. An argument written only with literal text, single quotes and
  double quotes holding no expansion is literal; anything else (`$VAR`, `${X}`, `$(dirname
  "$0")/x`, a tilde, an arithmetic or process substitution) is dynamic, and an unquoted `*`, `?`
  or `[` makes it a glob. Dynamic and glob statements are counted, never resolved.
- Lookup: bash's own rule (bash(1), `.` builtin). A name with a slash opens relative to the
  current directory; an absolute one is outside the subject. A name with no slash is searched in
  PATH and then, when bash is not in POSIX mode, in the current directory. PATH holds no subject
  directory, so a slash-less name resolves only through the current directory, and only for an
  importer bash runs in bash mode: a script whose `#!` names `sh` runs bash in POSIX mode (or
  dash), which never searches the current directory, so its slash-less names are counted
  unresolved (`source_posix_path_only`).
- Current directory: the operator's choice, as the load path is ruby-require's. A static run
  cannot know where a script is started, so each literal path is looked up from the script's
  own directory, then from the subject root: ShellCheck's `-x -P SCRIPTDIR` order run from the
  root. The first regular file found inside the subject is the statement's file.

Tier B, not A: the parser is not bash's, and the current directory is the operator's (design
section 1.3). With `--shellcheck`, ShellCheck (GPL-3.0, run as a tool, never linked) checks the
lookup: `shellcheck -x -P SCRIPTDIR --extended-analysis=false -f json1` from the subject root reports SC1091 on each
literal statement it could not follow; every statement this script resolves must be followed
and every relative literal one it leaves unresolved must not, and the agreement is disclosed (an
absolute path is outside the subject either way, and a `# shellcheck disable=SC1091` silences it).

    python3 -I bash_source_dump.py --shfmt <bin> [--shellcheck <bin>] --ext .sh --ext .bash \\
        --shebang bash --shebang sh <clone> > out/bash-<subject>-imports.json

Files: the bash manifest's match rule (a listed extension, or no extension and a `#!` line naming
a listed interpreter; env's first non-flag argument, a version suffix allowed), as
calibration/oracles/parser-ranges/shfmt_ranges.py selects them. A file shfmt rejects is counted.
Only statements resolving to a file of the subject are written; stderr and the `disclose` list
count the rest per outcome.
"""

import argparse
import json
import os
import subprocess
import sys

GLOB = set("*?[")


def interpreter(head):
    """The interpreter a `#!` first line names, as connector.shebangInterpreter reads it."""
    if not head.startswith(b"#!"):
        return ""
    line = head[2:].split(b"\n", 1)[0].decode("utf-8", "replace").strip()
    fields = line.split()
    if not fields:
        return ""
    prog = os.path.basename(fields[0])
    if prog == "env":
        for f in fields[1:]:
            if not f.startswith("-") and "=" not in f:
                return os.path.basename(f)
        return ""
    return prog


def interpreter_is(interp, name):
    return interp.startswith(name) and interp[len(name):].strip("0123456789.") == ""


def head_of(path):
    try:
        with open(path, "rb") as fh:
            return fh.read(256)
    except OSError:
        return b""


def wanted(root, rel, exts, shebangs):
    base = os.path.basename(rel)
    i = base.rfind(".")
    ext = base[i:].lower() if i >= 0 else ""  # Go's filepath.Ext: ".bashrc" has one
    if ext:
        return ext in exts
    interp = interpreter(head_of(os.path.join(root, rel)))
    return any(interpreter_is(interp, s) for s in shebangs)


def unescape_unquoted(s):
    """Quote removal of an unquoted literal: a backslash quotes the next character."""
    out, i = [], 0
    while i < len(s):
        if s[i] == "\\" and i + 1 < len(s):
            out.append(s[i + 1])
            i += 2
            continue
        out.append(s[i])
        i += 1
    return "".join(out)


def unescape_dquoted(s):
    """Inside double quotes a backslash quotes only $ ` " \\ and a newline."""
    out, i = [], 0
    while i < len(s):
        if s[i] == "\\" and i + 1 < len(s) and s[i + 1] in '$`"\\\n':
            if s[i + 1] != "\n":
                out.append(s[i + 1])
            i += 2
            continue
        out.append(s[i])
        i += 1
    return "".join(out)


def unquoted_glob(s):
    """Whether an unquoted literal holds a glob character no backslash quotes."""
    i = 0
    while i < len(s):
        if s[i] == "\\":
            i += 2
            continue
        if s[i] in GLOB:
            return True
        i += 1
    return False


def word_literal(word):
    """(text, kind): kind is "literal", "glob" or "dynamic" for one shfmt Word."""
    text, glob = [], False
    for part in word.get("Parts", []):
        t = part.get("Type")
        if t == "Lit":
            v = part.get("Value", "")
            if not text and v.startswith("~"):
                return "", "dynamic"  # tilde expansion: the user's home
            if unquoted_glob(v):
                glob = True
            text.append(unescape_unquoted(v))
        elif t == "SglQuoted":
            if part.get("Dollar"):
                return "", "dynamic"  # $'...' ANSI-C quoting: escapes, not plain text
            text.append(part.get("Value", ""))
        elif t == "DblQuoted":
            if part.get("Dollar"):
                return "", "dynamic"  # $"..." locale translation
            for sub in part.get("Parts", []) or []:
                if sub.get("Type") != "Lit":
                    return "", "dynamic"
                text.append(unescape_dquoted(sub.get("Value", "")))
        else:
            return "", "dynamic"
    return "".join(text), ("glob" if glob else "literal")


def calls(node, out):
    if isinstance(node, dict):
        if node.get("Type") == "CallExpr" and node.get("Args"):
            out.append(node)
        for v in node.values():
            calls(v, out)
    elif isinstance(node, list):
        for v in node:
            calls(v, out)


def source_statements(tree):
    """(line, word or None) for each source statement: the command's line and its file argument."""
    found = []
    cs = []
    calls(tree, cs)
    for c in cs:
        args = c["Args"]
        name, kind = word_literal(args[0])
        i = 1
        if kind == "literal" and name in ("builtin", "command") and len(args) > 1:
            name, kind = word_literal(args[1])
            i = 2
        if kind != "literal" or name not in ("source", "."):
            continue
        line = args[0]["Pos"]["Line"]
        found.append((line, args[i] if len(args) > i else None))
    return found


def inside(root, p):
    real = os.path.realpath(p)
    rroot = os.path.realpath(root)
    return real == rroot or real.startswith(rroot + os.sep)


def lookup(root, rel, value, posix):
    """(outcome, to_path): bash's lookup from the script's directory, then the subject root."""
    if value.startswith("/"):
        return "source_absolute", ""
    if "/" not in value and posix:
        return "source_posix_path_only", ""
    for cwd in (os.path.dirname(rel), ""):
        cand = os.path.normpath(os.path.join(root, cwd, value))
        if not inside(root, cand):
            continue
        if os.path.isfile(cand):
            to = os.path.relpath(os.path.realpath(cand), os.path.realpath(root)).replace(os.sep, "/")
            return "source_in_subject", to
    return "source_unresolved", ""


def shellcheck_unfollowed(sc, root, files):
    """{(path, line)} ShellCheck -x -P SCRIPTDIR reports SC1091 on, run from the subject root."""
    out = set()
    for i in range(0, len(files), 50):
        batch = files[i:i + 50]
        p = subprocess.run([sc, "-x", "-P", "SCRIPTDIR", "--extended-analysis=false", "-f", "json1", "-S", "info", *batch],
                           cwd=root, capture_output=True)
        try:
            doc = json.loads(p.stdout or b"{}")
        except json.JSONDecodeError:
            continue
        for c in doc.get("comments", []):
            if c.get("code") == 1091:
                out.add((c["file"], c["line"]))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--shfmt", required=True)
    ap.add_argument("--shellcheck", default="")
    ap.add_argument("--ext", action="append", default=[])
    ap.add_argument("--shebang", action="append", default=[])
    ap.add_argument("root")
    a = ap.parse_args()
    exts = {e.lower() for e in a.ext}
    version = subprocess.run([a.shfmt, "--version"], capture_output=True, text=True, check=True).stdout.strip()
    files = []
    for dirpath, dirnames, filenames in os.walk(a.root):
        dirnames[:] = sorted(d for d in dirnames if d != ".git")
        for f in sorted(filenames):
            rel = os.path.relpath(os.path.join(dirpath, f), a.root).replace(os.sep, "/")
            if os.path.isfile(os.path.join(a.root, rel)) and wanted(a.root, rel, exts, a.shebang):
                files.append(rel)
    files.sort()
    counts = {k: 0 for k in ("files", "unparsed", "statements", "source_in_subject", "source_unresolved",
                             "source_absolute", "source_posix_path_only", "source_dynamic", "source_glob",
                             "source_no_argument")}
    imports, literal = [], []
    for rel in files:
        with open(os.path.join(a.root, rel), "rb") as fh:
            src = fh.read()
        p = subprocess.run([a.shfmt, "-ln", "bash", "--to-json"], input=src, capture_output=True)
        if p.returncode != 0:
            counts["unparsed"] += 1
            print(f"shfmt: {rel}: {p.stderr.decode('utf-8', 'replace').strip()}", file=sys.stderr)
            continue
        counts["files"] += 1
        posix = interpreter_is(interpreter(src[:256]), "sh")
        for line, word in source_statements(json.loads(p.stdout)):
            counts["statements"] += 1
            if word is None:
                counts["source_no_argument"] += 1
                continue
            value, kind = word_literal(word)
            if kind != "literal":
                counts["source_" + kind] += 1
                continue
            outcome, to = lookup(a.root, rel, value, posix)
            counts[outcome] += 1
            if outcome != "source_absolute":  # outside the subject; a directive may silence ShellCheck on it
                literal.append((rel, line, outcome == "source_in_subject"))
            if to:
                imports.append({"path": rel, "line": line, "to_path": to, "module": value})
    imports.sort(key=lambda i: (i["path"], i["line"], i["to_path"], i["module"]))
    tool_version = f"{version} (mvdan.cc/sh/v3 syntax, -ln bash); bash 5 lookup, cwd the script's directory then the subject root"
    disclose = [
        "bash-source (tier B): {statements} source/. statements in {files} files ({unparsed} shfmt rejected): "
        "{source_in_subject} resolve to a subject file, {source_unresolved} literal paths name no subject file, "
        "{source_absolute} are absolute, {source_posix_path_only} are slash-less names in a sh script (PATH only), "
        "{source_dynamic} are dynamic ($VAR, $(...), ~) and {source_glob} globs, counted and never resolved, "
        "{source_no_argument} have no argument; the current directory is the operator's (the script's directory, "
        "then the subject root)".format(**counts)
    ]
    if a.shellcheck:
        sc_version = subprocess.run([a.shellcheck, "--version"], capture_output=True, text=True).stdout.split()
        scv = sc_version[sc_version.index("version:") + 1] if "version:" in sc_version else "unknown"
        unfollowed = shellcheck_unfollowed(a.shellcheck, a.root, files)
        agree = sum(1 for rel, line, ok in literal if ok != ((rel, line) in unfollowed))
        disagree = [f"{rel}:{line}" for rel, line, ok in literal if ok == ((rel, line) in unfollowed)]
        disclose.append(
            f"ShellCheck {scv} -x -P SCRIPTDIR from the subject root agrees on {agree} of {len(literal)} relative literal "
            f"statements (followed where resolved, SC1091 where not)"
            + (f"; disagrees on {', '.join(disagree[:5])}" if disagree else ""))
        tool_version += f"; checked by ShellCheck {scv}"
    print(" ".join(f"{k} {v}" for k, v in counts.items()), file=sys.stderr)
    json.dump({
        "tool": "bash-source",
        "tier": "B",
        "version": tool_version,
        "disclose": disclose,
        "imports": imports,
    }, sys.stdout, indent=1, sort_keys=True)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
