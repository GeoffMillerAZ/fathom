#!/usr/bin/env bash
# cuda-samples.sh — the LB-Q-cuda measurement driver (F12): NVIDIA/cuda-samples as ONE subject, the way a user
# indexes it, from the samples' own CMake build (owner ruling C-R4).
#
#   cuda-samples.sh <shared-clone> <cuda-root> <tools-dir> <work-dir> [runs]
#
# <shared-clone> is calibration/clone.sh's sparse checkout (never written); <cuda-root> an unpacked CUDA 12.9
# toolkit with the nvcc driver and cicc (conda-forge cuda-nvcc 12.9.86 and the -dev header packages, under
# workspace/tools, never committed); <tools-dir> holds bin/scip-clang (0.4.0) and ctags; <work-dir> is scratch.
# Steps:
#   1. copy the subject to <work-dir>/subject; give the sources the sparse clone leaves out (data, .c, .ll) an empty
#      file and each static library the toolkit lacks an empty archive, so CMake configures (nothing is built);
#   2. cmake with nvcc: <work-dir>/nvcc_compile_commands.json, the build's own database;
#   3. the build's own preprocessor, both passes (tier A, ruling C-R6 E1): nvcc -E is the device pass at the lowest
#      architecture the build names; the host pass is the gcc -E line nvcc --dryrun prints for the entry (no
#      __CUDA_ARCH__); the two runs' active lines and includes are unioned into preproc.json;
#   4. a clang CUDA host-only translation of each .cu entry (cuda-compdb.py's flags over the entry's own -I, -isystem
#      and -D) at <work-dir>/subject/build/compile_commands.json: scip-clang 0.4.0 does not read nvcc entries, so the
#      index, clang-calls and clang-ast-defs read it, and the last two are written at -tier B;
#   5. indexbench lang-oracle over the one subject, <runs> times (default 3) at one commit, readings kept apart
#      (ruling C-R5: the median throughput is the one committed).
set -euo pipefail
clone=$(cd "$1" && pwd) cuda=$(cd "$2" && pwd) tools=$(cd "$3" && pwd) work=$4 runs=${5:-3}
repo=$(cd "$(dirname "$0")/../../.." && pwd)
here="$repo/calibration/oracles/scip-clang"
mkdir -p "$work" && work=$(cd "$work" && pwd)
subj="$work/subject"
sha=$(git -C "$clone" rev-parse HEAD) # the copy below has no .git: the reading names the clone's commit (round 3 A1)
rm -rf "$subj" && cp -a "$clone" "$subj" && rm -rf "$subj/.git"

lib="$cuda/targets/x86_64-linux/lib"
[ -e "$cuda/include" ] || ln -s targets/x86_64-linux/include "$cuda/include"
for n in $(grep -rhoE 'CUDA::[A-Za-z_]+_static' "$subj/Samples" --include=CMakeLists.txt | sort -u | sed 's/CUDA:://'); do
  [ -e "$lib/lib$n.a" ] || ar rc "$lib/lib$n.a"
done
cmake_args=(-S "$subj" -B "$subj/build-nvcc" -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
  -DCMAKE_CUDA_COMPILER="$cuda/bin/nvcc" -DCUDAToolkit_ROOT="$cuda" -DNVJITLINK_LIB="$lib/libcudart.so")
for _ in 1 2 3 4 5 6; do
  if cmake "${cmake_args[@]}" > "$work/cmake.log" 2>&1; then break; fi
  python3 - "$work/cmake.log" "$subj" <<'EOF'
import os, re, sys
log, subj = open(sys.argv[1]).read(), sys.argv[2]
for d, f in re.findall(r'CMake Error at (\S+)/CMakeLists.txt:\d+ \((?:add_executable|add_library)\):\s*Cannot find source file:\s*\n\s*\n\s*(\S+)', log):
    p = os.path.join(subj, d, f) if not os.path.isabs(d) else os.path.join(d, f)
    os.makedirs(os.path.dirname(p), exist_ok=True); open(p, "a").close()
for f in re.findall(r'file COPY cannot find\s*"([^"]+)"', log):
    os.makedirs(os.path.dirname(f), exist_ok=True); open(f, "a").close()
EOF
done
cp "$subj/build-nvcc/compile_commands.json" "$work/nvcc_compile_commands.json"

# 3 and 4: the derived databases.
python3 - "$work" "$subj" "$cuda" <<'EOF'
import json, os, re, shlex, subprocess, sys
work, subj, cuda = sys.argv[1:4]
nv = [e for e in json.load(open(os.path.join(work, "nvcc_compile_commands.json"))) if e["file"].endswith(".cu")]
nv.sort(key=lambda e: e["file"])
dev, host, clang = [], [], []
for e in nv:
    args = shlex.split(e["command"])
    gens = [a for a in args if a.startswith("--generate-code=arch=compute_")]
    archs = sorted({int(re.search(r"compute_(\d+)", g).group(1)) for g in gens}) or [52]
    keep = [a for a in args if not a.startswith("--generate-code")]
    low = "--generate-code=arch=compute_%d,code=[compute_%d,sm_%d]" % ((archs[0],) * 3)
    dev.append({"directory": e["directory"], "file": e["file"], "command": shlex.join(keep[:1] + [low] + keep[1:])})
    dry = subprocess.run(keep[:1] + [low] + keep[1:] + ["--dryrun"], cwd=e["directory"], capture_output=True, text=True).stderr
    for line in dry.splitlines():
        line = line.removeprefix("#$ ").strip()
        if re.match(r"^\S*(gcc|g\+\+|c\+\+)\s", line) and " -E " in line and "__CUDA_ARCH__" not in line and "-D__CUDACC__" in line:
            h = shlex.split(line)
            if "-o" in h:
                i = h.index("-o"); del h[i:i + 2]
            host.append({"directory": e["directory"], "file": e["file"], "command": shlex.join(h)})
            break
    flags, i = [], 1
    while i < len(keep):
        a = keep[i]
        if a in ("-isystem", "-I", "-D") and i + 1 < len(keep):
            flags += [a, keep[i + 1]]; i += 2; continue
        if a.startswith(("-I", "-D", "-isystem")):
            flags.append(a)
        i += 1
    cmd = ["clang++", "-x", "cuda", "--cuda-path=" + cuda, "--cuda-host-only", "-nocudalib", "-Wno-unknown-cuda-version",
           "-std=c++17", "-I" + os.path.join(cuda, "include")] + flags + ["-c", e["file"], "-o", "/dev/null"]
    clang.append({"directory": subj, "file": e["file"], "command": shlex.join(cmd), "archs": archs})
for name, db in (("nvcc-device", dev), ("nvcc-host", host)):
    json.dump(db, open(os.path.join(work, name + ".json"), "w"), indent=1)
# A unit clang cannot compile (an OpenMP, GL or Vulkan header it cannot find, a construct only nvcc accepts) is
# dropped from the clang database and listed: clang-calls refuses a run with a unit that stops.
from concurrent.futures import ThreadPoolExecutor
def compiles(c):
    a = shlex.split(c["command"]); i = a.index("-c")
    r = subprocess.run(a[:i] + ["-fsyntax-only", "-w", c["file"]], cwd=subj, capture_output=True, text=True)
    return r.returncode == 0
with ThreadPoolExecutor(4) as ex:
    ok = list(ex.map(compiles, clang))
kept = [c for c, k in zip(clang, ok) if k]
json.dump(sorted(os.path.relpath(c["file"], subj) for c, k in zip(clang, ok) if not k), open(os.path.join(work, "clang-skipped.json"), "w"), indent=1)
os.makedirs(os.path.join(subj, "build"), exist_ok=True)
json.dump([{k: v for k, v in c.items() if k != "archs"} for c in kept], open(os.path.join(subj, "build", "compile_commands.json"), "w"), indent=1)
print(len(nv), "cu entries;", len(host), "host passes;", len(kept), "clang compiles; lowest archs", sorted({c["archs"][0] for c in clang}), file=sys.stderr)
EOF

export PATH="$cuda/bin:$PATH"
( cd "$here/preproc" && go run . -subject "$subj" -compdb "$work/nvcc-device.json" -tier A > "$work/preproc-device.json" )
( cd "$here/preproc" && go run . -subject "$subj" -compdb "$work/nvcc-host.json" -tier A > "$work/preproc-host.json" )
python3 - "$work" <<'EOF'
import json, os, sys
w = sys.argv[1]
a, b = (json.load(open(os.path.join(w, f))) for f in ("preproc-device.json", "preproc-host.json"))
lines = {}
for src in (a, b):
    for f, rs in src.get("active_lines", {}).items():
        s = lines.setdefault(f, set())
        for r in rs:
            s.update(range(r[0], r[1] + 1))
def ranges(s):
    out, xs = [], sorted(s)
    for x in xs:
        if out and out[-1][1] + 1 == x: out[-1][1] = x
        else: out.append([x, x])
    return out
imps = {json.dumps(i, sort_keys=True) for src in (a, b) for i in src.get("imports", [])}
out = dict(a)
out["active_lines"] = {f: ranges(s) for f, s in sorted(lines.items())}
out["imports"] = [json.loads(i) for i in sorted(imps)]
out["version"] = a.get("version", "") + " (nvcc device and host passes, unioned)"
json.dump(out, open(os.path.join(w, "preproc.json"), "w"), indent=1)
EOF
bash "$here/run.sh" "$tools/bin/scip-clang" "$subj" "$subj/build" "$work/index.scip" 4
( cd "$repo/calibration/oracles/clang-calls" && go run . -subject "$subj" -compdb "$subj/build/compile_commands.json" -jobs 4 -tier B > "$work/clang-calls.json" )
( cd "$repo/calibration/oracles/clang-ast-defs" && go run . -subject "$subj" -compdb "$subj/build/compile_commands.json" -jobs 2 -tier B > "$work/preproc.ranges.json" )

cd "$repo"
for i in $(seq 1 "$runs"); do
  go run ./cmd/indexbench lang-oracle -dialect cuda -subject "$subj" -subject-name NVIDIA/cuda-samples -subject-sha "$sha" \
    -ctags "$tools/ctags" -scip "$work/index.scip" -tool-oracle "$work/preproc.json" -tool-oracle "$work/clang-calls.json" \
    -unmatched "$work/unmatched-$i.json" -out "$work/reading-$i.json" > "$work/lang-oracle-$i.log" 2>&1
done
