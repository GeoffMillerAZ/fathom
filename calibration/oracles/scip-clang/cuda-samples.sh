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
# clang's --cuda-path reads <cuda-root>/include: a conda install keeps the toolkit's headers under targets/ and may
# already hold a include/ of its own (cccl's), so each missing entry is linked in.
if [ ! -e "$cuda/include" ]; then ln -s targets/x86_64-linux/include "$cuda/include"; fi
for h in "$cuda/targets/x86_64-linux/include/"*; do
  [ -e "$cuda/include/$(basename "$h")" ] || ln -s "$h" "$cuda/include/$(basename "$h")"
done
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
# 2a (round 3 B): each CMake executable's sources, from the build's own link lines (an object the linker reads is the
# compile database entry that writes it; a library the build links, its own objects), at preproc.units.json, the
# link-unit companion of the preprocessor oracle the reading reads.
python3 - "$subj" "$subj/build-nvcc" "$work/preproc.units.json" <<'EOF'
import json, os, re, sys
subj, build, out = sys.argv[1:4]
src_of = {}
for e in json.load(open(os.path.join(build, "compile_commands.json"))):
    o = e.get("output", "")
    src_of[os.path.normpath(os.path.join(e["directory"], o)) if o else ""] = os.path.relpath(e["file"], subj)
text = open(os.path.join(build, "build.ninja")).read().replace("$\n", " ")
links, libs = {}, {}
for line in text.splitlines():
    m = re.match(r"build (\S+): (\w*(?:EXECUTABLE|STATIC_LIBRARY|SHARED_LIBRARY|MODULE_LIBRARY)_LINKER__\S+) (.*)", line)
    if not m or "DEVICE_LINKER" in m.group(2):
        continue
    target = os.path.normpath(os.path.join(build, m.group(1)))
    explicit, _, implicit = m.group(3).partition(" | ")
    ins = [os.path.normpath(os.path.join(build, x)) for x in explicit.split() + implicit.split("||")[0].split()]
    (links if "EXECUTABLE" in m.group(2) else libs)[target] = ins
def sources(ins, seen):
    out = set()
    for i in ins:
        if i in src_of:
            out.add(src_of[i])
        elif i in libs and i not in seen:
            out |= sources(libs[i], seen | {i})
    return out
units = {os.path.relpath(t, build): sorted(sources(ins, set())) for t, ins in sorted(links.items())}
json.dump({"link_units": {u: s for u, s in units.items() if s}}, open(out, "w"), indent=1)
print("link units:", sum(1 for s in units.values() if s), "executables,", len({x for s in units.values() for x in s}), "sources", file=sys.stderr)
EOF

# 2b (round 3 A2): the samples' own build with clang as its CUDA compiler. An index is tier A only when the subject's
# own build wrote its database untranslated: when 90% of the .cu units it configures compile as written, that
# database is the index's (tier A); otherwise the translation below is, at tier B. The counts are in clang-native.json.
clang_args=(-S "$subj" -B "$subj/build-clang" -G Ninja -DCMAKE_EXPORT_COMPILE_COMMANDS=ON -DCMAKE_CUDA_COMPILER=clang++
  -DCMAKE_CUDA_ARCHITECTURES=52 -DCUDAToolkit_ROOT="$cuda" -DCMAKE_CUDA_FLAGS="--cuda-path=$cuda -Wno-unknown-cuda-version"
  -DNVJITLINK_LIB="$lib/libcudart.so")
PATH="$cuda/bin:$PATH" cmake "${clang_args[@]}" > "$work/cmake-clang.log" 2>&1 || true
PATH="$cuda/bin:$PATH" python3 - "$subj/build-clang/compile_commands.json" "$work/clang-native.json" <<'EOF2'
import collections, json, os, shlex, subprocess, sys
from concurrent.futures import ThreadPoolExecutor
db = [e for e in json.load(open(sys.argv[1])) if e["file"].endswith(".cu")] if os.path.exists(sys.argv[1]) else []
def run(e):
    a = shlex.split(e["command"]); i = a.index("-o"); a = a[:i] + ["-o", "/dev/null"] + a[i + 2:]
    r = subprocess.run(a, cwd=e["directory"], capture_output=True, text=True)
    return r.returncode == 0, sorted({l.strip() for l in r.stderr.splitlines() if "error:" in l})
with ThreadPoolExecutor(4) as ex:
    res = list(ex.map(run, db))
ok = sum(1 for k, _ in res if k)
errs = collections.Counter(l for k, ls in res if not k for l in ls)
out = {"configured": len(db), "compiled": ok, "tier": "A" if db and ok * 10 >= len(db) * 9 else "B",
       "errors": [{"count": n, "error": l} for l, n in sorted(errs.items(), key=lambda x: (-x[1], x[0]))[:8]]}
json.dump(out, open(sys.argv[2], "w"), indent=1)
print("clang-native:", out["configured"], "configure,", ok, "compile -> tier", out["tier"], file=sys.stderr)
EOF2
scip_tier=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["tier"])' "$work/clang-native.json")

# 3 and 4: the derived databases.
python3 - "$work" "$subj" "$cuda" <<'EOF'
import json, os, re, shlex, subprocess, sys
work, subj, cuda = sys.argv[1:4]
nv = [e for e in json.load(open(os.path.join(work, "nvcc_compile_commands.json"))) if e["file"].endswith(".cu")]
nv.sort(key=lambda e: e["file"])
dev, host, clang, clang_dev = [], [], [], []
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
    # The device pass of the same unit (round 3 C, E2 for the calls): the active-line oracle is nvcc's host-plus-device
    # union, so the call sites are clang's host pass unioned with its device-only pass at the target's lowest arch.
    # -nocudalib keeps clang from reading the toolkit's version, so the SDK version the host pass takes from it is given
    # (clang 18 knows CUDA up to 12.3; without it a launch needs the pre-9.2 cudaConfigureCall). Each of the target's
    # CMake architectures is tried lowest first: a sample whose device code needs a later one (cuda::barrier, sm_70)
    # compiles at the first it builds for.
    dcmd = [a for a in cmd if a != "--cuda-host-only"]
    x = dcmd.index("-x") + 2
    dcmd[x:x] = ["--cuda-device-only", "-Xclang", "-target-sdk-version=12.3", "--cuda-gpu-arch=sm_%d"]
    clang_dev.append({"directory": subj, "file": e["file"], "command": shlex.join(dcmd), "archs": archs})
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
    def device(c):
        for a in c["archs"]:
            d = dict(c, command=c["command"].replace("--cuda-gpu-arch=sm_%d", "--cuda-gpu-arch=sm_%d" % a))
            if compiles(d):
                c["command"] = d["command"]
                return True
        return False
    ok_dev = list(ex.map(device, clang_dev))
kept = [c for c, k in zip(clang, ok) if k]
json.dump([{k: v for k, v in c.items() if k != "archs"} for c, k, h in zip(clang_dev, ok_dev, ok) if k and h], open(os.path.join(work, "clang-device.json"), "w"), indent=1)
json.dump(sorted(os.path.relpath(c["file"], subj) for c, k, h in zip(clang_dev, ok_dev, ok) if h and not k),
          open(os.path.join(work, "clang-device-skipped.json"), "w"), indent=1)
json.dump(sorted(os.path.relpath(c["file"], subj) for c, k in zip(clang, ok) if not k), open(os.path.join(work, "clang-skipped.json"), "w"), indent=1)
os.makedirs(os.path.join(subj, "build"), exist_ok=True)
json.dump([{k: v for k, v in c.items() if k != "archs"} for c in kept], open(os.path.join(subj, "build", "compile_commands.json"), "w"), indent=1)
print(len(nv), "cu entries;", len(host), "host passes;", len(kept), "clang compiles; lowest archs", sorted({c["archs"][0] for c in clang}), file=sys.stderr)
EOF

if [ "$scip_tier" = A ]; then # the samples' own clang build compiles: its database, untranslated, is the index's
  cp "$subj/build-clang/compile_commands.json" "$subj/build/compile_commands.json"
fi
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
bash "$here/run.sh" "$tools/bin/scip-clang" "$subj" "$subj/build" "$work/index.scip" "$scip_tier" 4
( cd "$repo/calibration/oracles/clang-calls" && go run . -subject "$subj" -compdb "$subj/build/compile_commands.json" -jobs 4 -tier "$scip_tier" > "$work/clang-calls-host.json" )
( cd "$repo/calibration/oracles/clang-calls" && go run . -subject "$subj" -compdb "$work/clang-device.json" -jobs 4 -tier "$scip_tier" > "$work/clang-calls-device.json" )
python3 - "$work" <<'EOF'
import json, os, sys
w = sys.argv[1]
h, d = (json.load(open(os.path.join(w, f))) for f in ("clang-calls-host.json", "clang-calls-device.json"))
key = lambda c: (c["path"], c["line"], c["col"], c["name"], c.get("class", ""))
sites = {key(c): c for src in (h, d) for c in src.get("call_sites", [])}
out = dict(h)
out["call_sites"] = [sites[k] for k in sorted(sites)]
out["version"] = h.get("version", "") + " (clang CUDA host and device-only passes, unioned)"
json.dump(out, open(os.path.join(w, "clang-calls.json"), "w"), indent=1)
print("clang-calls:", len(h.get("call_sites", [])), "host,", len(d.get("call_sites", [])), "device,", len(out["call_sites"]), "unioned", file=sys.stderr)
EOF
( cd "$repo/calibration/oracles/clang-ast-defs" && go run . -subject "$subj" -compdb "$subj/build/compile_commands.json" -jobs 2 -tier "$scip_tier" > "$work/preproc.ranges.json" )

# The driver's own record of how the subject was measured (round 3 A4): never a hand edit of the reading.
note=$(python3 - "$work" "$scip_tier" <<'EOF'
import json, os, sys
w, tier = sys.argv[1], sys.argv[2]
nat = json.load(open(os.path.join(w, "clang-native.json")))
nv = json.load(open(os.path.join(w, "nvcc_compile_commands.json")))
skipped = json.load(open(os.path.join(w, "clang-skipped.json")))
dskip = json.load(open(os.path.join(w, "clang-device-skipped.json")))
units = sum(1 for e in nv if e["file"].endswith(".cu"))
errs = "; ".join("%d: %s" % (e["count"], e["error"].replace("clang++: error: ", "")) for e in nat["errors"][:4])
idx = ("the samples' own clang build's database (tier A)" if tier == "A" else
       "a clang CUDA host-only translation of the nvcc database (tier B: a translated database is never the build's own)")
names = lambda fs: ", ".join(os.path.basename(f) for f in fs) or "none"
print("measured as ONE subject from the samples' own CMake build with nvcc (owner ruling C-R4): %d .cu translation units "
      "configure (the OpenMP, GL, Vulkan, EGL and Tegra samples' CMake skips itself); the preprocessor oracle is nvcc's own, "
      "the device pass at each target's lowest architecture unioned with the host pass (tier A, ruling C-R6); configured "
      "with clang++ as its CUDA compiler the build configures %d and compiles %d as written (%s), so the index, clang-calls "
      "and clang-ast-defs read %s, %d units (%d clang cannot compile are dropped: %s), and clang-calls unions that host pass "
      "with a device-only pass at the target's lowest architecture (%d units; %d more dropped there: %s)%s"
      % (units, nat["configured"], nat["compiled"], errs, idx, units - len(skipped), len(skipped), names(skipped),
         units - len(skipped) - len(dskip), len(dskip), names(dskip),
         "; a tier-B oracle caps L2 and above at good (tierCeiling)" if tier != "A" else ""))
EOF
)
cd "$repo"
for i in $(seq 1 "$runs"); do
  go run ./cmd/indexbench lang-oracle -dialect cuda -subject "$subj" -subject-name NVIDIA/cuda-samples -subject-sha "$sha" -scip-tier "$scip_tier" \
    -ctags "$tools/ctags" -scip "$work/index.scip" -tool-oracle "$work/preproc.json" -tool-oracle "$work/clang-calls.json" \
    -note "$note" -unmatched "$work/unmatched-$i.json" -out "$work/reading-$i.json" > "$work/lang-oracle-$i.log" 2>&1
done
# The throughput record (ruling C-R5): the median run is the reading, written with the record to <work-dir>/reading.json.
python3 - "$work" "$runs" <<'EOF'
import json, os, shutil, sys
w, n = sys.argv[1], int(sys.argv[2])
rs = [json.load(open(os.path.join(w, "reading-%d.json" % i))) for i in range(1, n + 1)]
cpu = [r["tenancy"]["throughput_mb_s"] for r in rs]
wall = [r["tenancy"].get("wall_throughput_mb_s", 0) for r in rs]
med = sorted(range(n), key=lambda i: cpu[i])[n // 2]
r = rs[med]
f = lambda xs: (", ".join("%.2f" % x for x in xs[:-1]) + " and " if len(xs) > 1 else "") + "%.2f" % xs[-1]
r["notes"] = sorted(r["notes"] + ["%s: throughput read %d times at fathom %s in one VM, one subject (owner ruling C-R5): %s MB/s per core (wall %s); the median run, %.2f, is this reading"
                                  % (r["subjects"][0]["name"], n, r["fathom_commit"], f(cpu), f(wall), cpu[med])])
with open(os.path.join(w, "reading.json"), "w") as fh: # byte for byte what langbench.MarshalReading writes
    fh.write(json.dumps(r, indent=2, ensure_ascii=False).replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026") + "\n")
shutil.copy(os.path.join(w, "unmatched-%d.json" % (med + 1)), os.path.join(w, "unmatched.json"))
print("throughput", cpu, "median run", med + 1, file=sys.stderr)
EOF
