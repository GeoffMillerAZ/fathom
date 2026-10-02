#!/usr/bin/env python3
"""cmake_oracle.py: CMake's own evaluation as a tier-A oracle for indexbench lang-oracle (LB-Q-cmake).

Python 3 standard library only; bench-only, never imported by fathom. It configures a subject with
CMake's File API codemodel-v2 query and CMake's own command trace, and writes the neutral oracle file
`indexbench lang-oracle -tool-oracle` reads (calibration/oracles/README.md):

  references        (path, target, referenced name; `::` written `.`, the spelling the dialect gives a
                    namespaced target, CT-1) for every literal argument of an executed
                    target_link_libraries / add_dependencies command whose target is defined in the
                    same file: the trace (--trace-expand --trace-format=json-v1) says the command ran,
                    the source says how each argument was spelled.
  cross_references  (path, target, to_path, dependency) for every dependency the codemodel records
                    between two targets whose definitions sit in different files, on the file the
                    dependency was added in (the dependency's backtrace); file granularity.

An argument or dependency written with a variable, a generator expression, a quote or a path is not a
name a static reader of the file can see; it is counted on stderr and left out of the truth, as the make
oracle leaves out a prerequisite spelled with `$`.

  python3 cmake_oracle.py -subject <checkout> -build <scratch dir> [-cmake cmake] [-D VAR=VALUE]...
"""
import argparse
import glob
import json
import os
import re
import subprocess
import sys

KEYWORDS = {"PRIVATE", "PUBLIC", "INTERFACE", "LINK_PRIVATE", "LINK_PUBLIC", "LINK_INTERFACE_LIBRARIES",
            "debug", "optimized", "general"}
NAME_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_.:+-]*$")
LINK_CMDS = ("target_link_libraries", "add_dependencies")
DEF_CMDS = ("add_executable", "add_library", "add_custom_target")


def command_args(src, line):
    """The raw argument tokens of the command that starts on 1-based line `line` of src:
    [(text, quoted)], comments dropped. Returns None when the line holds no invocation."""
    lines = src.split("\n")
    if line < 1 or line > len(lines):
        return None
    start = sum(len(l) + 1 for l in lines[: line - 1])
    m = re.compile(r"\s*[A-Za-z_][A-Za-z0-9_]*\s*\(").match(src, start)
    if not m:
        return None
    i, n, depth = m.end(), len(src), 1
    toks, cur = [], []

    def flush():
        if cur:
            toks.append(("".join(cur), False))
            cur.clear()

    while i < n and depth > 0:
        c = src[i]
        if c == "#":
            flush()
            b = re.compile(r"#\[(=*)\[").match(src, i)
            if b:
                end = src.find("]" + b.group(1) + "]", b.end())
                i = n if end < 0 else end + len(b.group(1)) + 2
            else:
                j = src.find("\n", i)
                i = n if j < 0 else j
            continue
        if c == '"':
            flush()
            j = i + 1
            while j < n and src[j] != '"':
                j += 2 if src[j] == "\\" else 1
            toks.append((src[i + 1 : j], True))
            i = j + 1
            continue
        if c == "[":
            b = re.compile(r"\[(=*)\[").match(src, i)
            if b:
                flush()
                end = src.find("]" + b.group(1) + "]", b.end())
                j = n if end < 0 else end + len(b.group(1)) + 2
                toks.append((src[b.end() : max(b.end(), j - len(b.group(1)) - 2)], True))
                i = j
                continue
        if c == "(":
            depth += 1
            cur.append(c)
        elif c == ")":
            depth -= 1
            if depth > 0:
                cur.append(c)
        elif c in " \t\r\n":
            flush()
        else:
            cur.append(c)
        i += 1
    flush()
    return toks


def literal_names(src, line, skip_first=True):
    """Unquoted, variable-free, name-shaped argument tokens after the first."""
    toks = command_args(src, line)
    if not toks:
        return None, []
    first = toks[0][0] if not toks[0][1] else None
    out = []
    for text, quoted in toks[1:] if skip_first else toks:
        if quoted or "$" in text or text in KEYWORDS or not NAME_RE.match(text):
            continue
        out.append(text)
    return first, out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-build", required=True, help="scratch build directory (created, never inside the subject)")
    ap.add_argument("-cmake", default="cmake")
    ap.add_argument("-D", action="append", default=[], dest="defs")
    ap.add_argument("-timeout", type=int, default=900)
    a = ap.parse_args()
    subject, build = os.path.realpath(a.subject), os.path.realpath(a.build)
    if build.startswith(subject + os.sep):
        sys.exit("cmake_oracle: -build must not be inside the subject")
    os.makedirs(os.path.join(build, ".cmake/api/v1/query"), exist_ok=True)
    open(os.path.join(build, ".cmake/api/v1/query/codemodel-v2"), "w").close()
    trace = os.path.join(build, "trace.json")
    cmd = [a.cmake, "-S", subject, "-B", build, "-Wno-dev", "--trace-expand", "--trace-format=json-v1",
           "--trace-redirect=" + trace] + ["-D" + d for d in a.defs]
    r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=a.timeout)
    if r.returncode != 0:
        sys.exit("cmake_oracle: configure failed:\n" + r.stdout[-3000:])
    version = subprocess.run([a.cmake, "--version"], stdout=subprocess.PIPE, text=True).stdout.split()[2]

    def rel(p):
        p = os.path.realpath(p) if os.path.isabs(p) else p
        if p.startswith(subject + os.sep) and not p.startswith(build + os.sep):
            return p[len(subject) + 1 :]
        return None

    srcs = {}

    def source(relpath):
        if relpath not in srcs:
            srcs[relpath] = open(os.path.join(subject, relpath), encoding="utf-8", errors="replace").read()
        return srcs[relpath]

    # The trace: which commands ran, and where each target was defined.
    defs_at = {}  # target name -> set of files
    links = []  # (rel path, line, cmd)
    seen = set()
    with open(trace, encoding="utf-8", errors="replace") as fh:
        for raw in fh:
            try:
                ev = json.loads(raw)
            except ValueError:
                continue
            if "cmd" not in ev:
                continue
            p = rel(ev["file"])
            if p is None:
                continue
            c = ev["cmd"].lower()
            args = ev.get("args") or []
            if c in DEF_CMDS and args:
                defs_at.setdefault(args[0], set()).add(p)
            elif c in LINK_CMDS and args and (p, ev["line"]) not in seen:
                seen.add((p, ev["line"]))
                links.append((p, ev["line"], c, args))
    references, stats = set(), {"link_commands": len(links), "args_not_literal": 0, "target_elsewhere": 0}
    for p, line, c, args in links:
        first, names = literal_names(source(p), line)
        if first is None or first != args[0]:
            stats["args_not_literal"] += 1
            continue
        if defs_at.get(first) != {p}:
            stats["target_elsewhere"] += 1
            continue
        for n in names:
            if n in args[1:]:
                references.add((p, first, n.replace("::", ".")))

    # The codemodel: every dependency between two targets.
    reply = sorted(glob.glob(os.path.join(build, ".cmake/api/v1/reply/index-*.json")))
    if not reply:
        sys.exit("cmake_oracle: no File API reply")
    index = json.load(open(reply[-1]))
    cm = next(o for o in index["objects"] if o["kind"] == "codemodel")
    model = json.load(open(os.path.join(build, ".cmake/api/v1/reply", cm["jsonFile"])))
    targets = {}
    for t in model["configurations"][0]["targets"]:
        tj = json.load(open(os.path.join(build, ".cmake/api/v1/reply", t["jsonFile"])))
        targets[tj["id"]] = tj

    def bt_site(tj, bt):
        g = tj["backtraceGraph"]
        node = g["nodes"][bt]
        return rel(os.path.join(subject, g["files"][node["file"]]) if not os.path.isabs(g["files"][node["file"]]) else g["files"][node["file"]]), node.get("line", 0)

    xrefs, xs = set(), {"dependencies": 0, "same_file": 0, "not_literal": 0, "outside_subject": 0}
    for tj in targets.values():
        for d in tj.get("dependencies", []):
            dep = targets.get(d["id"])
            if dep is None or "backtrace" not in tj or "backtrace" not in dep:
                continue
            xs["dependencies"] += 1
            from_path, _ = bt_site(tj, tj["backtrace"])
            to_path, _ = bt_site(dep, dep["backtrace"])
            link_path, link_line = bt_site(tj, d["backtrace"]) if "backtrace" in d else (None, 0)
            if None in (to_path, link_path) or from_path is None:
                xs["outside_subject"] += 1
                continue
            if to_path == link_path:
                xs["same_file"] += 1
                continue
            first, names = literal_names(source(link_path), link_line)
            if first != tj["name"] or dep["name"] not in names:
                xs["not_literal"] += 1
                continue
            xrefs.add((link_path, tj["name"].replace("::", "."), to_path, dep["name"].replace("::", ".")))

    out = {
        "tool": "cmake",
        "version": version + " (File API codemodel-v2, --trace-expand json-v1)",
        "tier": "A",
        "definitions": [],
        "references": [{"path": p, "from": f, "to": t} for p, f, t in sorted(references)],
        "cross_references": [{"path": p, "from": f, "to_path": tp, "to": t} for p, f, tp, t in sorted(xrefs)],
    }
    json.dump(out, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write("cmake_oracle: %d references %s, %d cross_references %s\n" % (len(references), stats, len(xrefs), xs))


if __name__ == "__main__":
    main()
