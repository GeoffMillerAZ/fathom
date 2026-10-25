#!/usr/bin/env python3
"""cuda-compdb.py — a clang compilation database for a CUDA calibration subject (LB-Q-cuda).

    cuda-compdb.py <subject-dir> <cuda-root> <out-compile_commands.json> [-I dir]... [--only substr]... [--max N]

The CUDA build's own compiler is nvcc, which a Linux x86_64 redistributable set without the nvcc driver
cannot run, so each .cu translation unit is rewritten to a clang CUDA host-only compile
(`clang++ -x cuda --cuda-path=<root> --cuda-host-only -nocudalib`): scip-clang then indexes it with the clang
it embeds. That is not the build's own compiler, so a run over this database is tier B for anything that reads
the compiler's own view (clang-calls, clang-ast-defs, the preprocessor), and the reading says so.
Only .cu files are translation units; a .cuh no .cu includes has no truth. Paths in the database are absolute
to <subject-dir>; run scip-clang from <subject-dir>. Output order is sorted.
"""
import json, os, sys

args = sys.argv[1:]
subject, root, out = (os.path.abspath(a) for a in args[:3])
incs, only, mx, i = [], [], 0, 3
while i < len(args):
    if args[i] == "-I":
        incs.append(os.path.join(subject, args[i + 1])); i += 2
    elif args[i] == "--only":
        only.append(args[i + 1]); i += 2
    elif args[i] == "--max":
        mx = int(args[i + 1]); i += 2
    else:
        raise SystemExit("unknown argument " + args[i])
files = []
for d, _, fs in os.walk(subject):
    if os.sep + "build" + os.sep in d + os.sep or ".git" in d.split(os.sep):
        continue
    for f in fs:
        if f.endswith(".cu"):
            p = os.path.join(d, f)
            rel = os.path.relpath(p, subject)
            if only and not any(s in rel for s in only):
                continue
            files.append(p)
files.sort()
if mx and len(files) > mx:
    step = len(files) / mx
    files = [files[int(k * step)] for k in range(mx)]
entries = []
for p in files:
    cmd = ["clang++", "-x", "cuda", "--cuda-path=" + root, "--cuda-host-only", "-nocudalib",
           "-Wno-unknown-cuda-version", "-std=c++17", "-I" + os.path.join(root, "include"), "-I" + os.path.dirname(p)]
    cmd += ["-I" + x for x in incs]
    cmd += ["-c", p, "-o", "/dev/null"]
    entries.append({"directory": subject, "file": p, "command": " ".join(cmd)})
os.makedirs(os.path.dirname(out), exist_ok=True)
json.dump(entries, open(out, "w"), indent=1)
print(len(entries), "entries", file=sys.stderr)
