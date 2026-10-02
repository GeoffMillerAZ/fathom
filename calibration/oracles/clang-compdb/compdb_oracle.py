#!/usr/bin/env python3
"""compdb_oracle.py: libclang's CompilationDatabase as a tier-A oracle for indexbench lang-oracle
(LB-Q-compile-commands).

Bench-only, never imported by fathom. Python 3 with `pip install libclang` (a venv under
<scratch>/lang-oracles/clang-compdb-venv). It loads a subject's compile_commands.json with
clang's own JSONCompilationDatabase (clang.cindex.CompilationDatabase), which tokenizes every `command`
string with LLVM's shell-quoting rules and prepends the driver mode, and writes the neutral oracle file
`indexbench lang-oracle -tool-oracle` reads (calibration/oracles/README.md):

  definitions  one `step` per database entry, named by the entry's `file` as written, on the lines the
               entry's object spans in the JSON text (clang reports no positions; the k-th object of the
               text is the k-th command clang returns, checked by file).
  references   (path, file, flag) for every include directory (-I, -iquote, -isystem, -idirafter), macro
               (-D) and output (-o) in the entry's arguments, each written the flag and its argument
               joined (`-I/usr/include`, `-DNDEBUG`, `-oa.o`): clang's driver reads each of the first
               four joined to its argument or separate from it (JoinedOrSeparate), -o separate or joined.

The dialect reads the flags as fusion's includeRoots does (-I joined or separate, the other three
separate only, whitespace-split command): where the two readers differ the pair is a miss, counted in the
reading. Entries with the same `file` and different flags are distinct entries and distinct oracle
definitions.

  compdb_oracle.py -subject <checkout holding compile_commands.json>
"""
import argparse
import importlib.metadata
import json
import os
import sys

import clang.cindex as ci

INCLUDE_FLAGS = ("-I", "-iquote", "-isystem", "-idirafter")


def object_lines(text):
    """(start line, end line) of every top-level object of a JSON array, 1-based."""
    spans, depth, in_str, esc, line, start = [], 0, False, False, 1, 0
    for c in text:
        if in_str:
            if esc:
                esc = False
            elif c == "\\":
                esc = True
            elif c == '"':
                in_str = False
        elif c == '"':
            in_str = True
        elif c == "{":
            depth += 1
            if depth == 1:
                start = line
        elif c == "}":
            depth -= 1
            if depth == 0:
                spans.append((start, line))
        if c == "\n":
            line += 1
    return spans


def flags(args):
    """Include directories, macros and output of a clang argument list, flag and argument joined."""
    inc, defs, out = [], [], None
    i = 0
    while i < len(args):
        a = args[i]
        hit = None
        for f in INCLUDE_FLAGS + ("-D",):
            if a == f and i + 1 < len(args):
                hit, i = f + args[i + 1], i + 1
                break
            if a.startswith(f) and len(a) > len(f):
                hit = a
                break
        if hit is not None:
            (defs if hit.startswith("-D") else inc).append(hit)
        elif a == "-o" and i + 1 < len(args):
            out, i = "-o" + args[i + 1], i + 1
        i += 1
    return inc, defs, out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    a = ap.parse_args()
    root = os.path.realpath(a.subject)
    path = os.path.join(root, "compile_commands.json")
    text = open(path, encoding="utf-8").read()
    entries = json.loads(text)
    spans = object_lines(text)
    cmds = ci.CompilationDatabase.fromDirectory(root).getAllCompileCommands()
    if not (len(entries) == len(spans) == len(cmds)):
        sys.exit("compdb_oracle: %d entries in the JSON, %d objects found, %d commands from clang" % (len(entries), len(spans), len(cmds)))
    defs, refs = [], set()
    for e, (s, t), cc in zip(entries, spans, cmds):
        want = os.path.normpath(os.path.join(e.get("directory", ""), e["file"]))
        if os.path.normpath(cc.filename) != want:
            sys.exit("compdb_oracle: entry %r is clang's %r" % (e["file"], cc.filename))
        defs.append({"path": "compile_commands.json", "name": e["file"], "line": s, "end": t, "kind": "step"})
        inc, ds, out = flags(list(cc.arguments))
        for f in inc + ds + ([out] if out else []):
            refs.add(("compile_commands.json", e["file"], f))
    json.dump({
        "tool": "libclang CompilationDatabase",
        "version": "libclang " + importlib.metadata.version("libclang") + " (pip)",
        "tier": "A",
        "definitions": defs,
        "references": [{"path": p, "from": f, "to": t} for p, f, t in sorted(refs)],
    }, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write("compdb_oracle: %d entries, %d reference pairs\n" % (len(defs), len(refs)))


if __name__ == "__main__":
    main()
