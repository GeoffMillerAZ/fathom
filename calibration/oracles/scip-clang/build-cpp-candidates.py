#!/usr/bin/env python3
"""Build two exact C++ candidates outside the shipped grammar lock."""
import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import urllib.request

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("willbooster_builder", HERE / "build-willbooster.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)
audit = builder.audit
CORE = ("tree-sitter/tree-sitter", "v0.26.11", "64402de2857cc197ecc4ca3bc144ea91fda7e72e", "1bab01ed21464f3272665b9c60e39ee79f68da1333e80b23f2c9356569d06971", "c5cfb43042b6b72045f4ba997834d0a7786d2793d91680868b5815b39f14fc78", 920828, "tree-sitter-0.26.11")
CANDIDATES = {
    "official-head": ("tree-sitter/tree-sitter-cpp", "c009222808634c1014f82438d4883753516a2c24", "6c6efaf024f5c02fee0403e4e2373346eeadc40828587bcfad9351df2f0a7986", "2e0110e07abef7c2548b26ec9d6969775617ca539a0dc8dbeeb14d6452c711d1", 1740811, "fde7035227a488422e95cc1a408fecbbc5b3440dceebf109e42d483d31270ed2", 2790800),
    "unreal": ("taku25/tree-sitter-unreal-cpp", "920e9810cc64a47fa05cfcf30c7caecb85b5f2c3", "285ad7c2969a03254ed7a77068e0effca26cb8f014bc5b9d300bd4415ec0b17b", "ef02f1077281d63262946e50ec461ba72efda41e51084c887379b59397b96a6d", 2628614, "613af35868891df920a1e5c12fb67b2c2e83eb6a7c9cab6d3a1247acd20d22df", 3975018),
}


def fetch(url, sha, size, archive):
    with urllib.request.urlopen(url, timeout=60) as response:
        data = response.read(size + 1)
    if len(data) != size or hashlib.sha256(data).hexdigest() != sha:
        raise audit.Refusal("pinned source archive identity mismatch")
    archive.write_bytes(data)


def source(work, label, repo, version, archive_sha, licence_sha, size, prefix, url):
    archive = work / (label + ".tar.gz")
    fetch(url, archive_sha, size, archive)
    dest = work / label
    dest.mkdir()
    entries, files, total = builder.extract(archive, dest, prefix)
    if audit.digest(dest / "LICENSE") != licence_sha:
        raise audit.Refusal("pinned MIT licence mismatch")
    return dest, {"crate": repo, "version": version, "archive_sha256": archive_sha,
                  "archive_bytes": size, "licence": "MIT", "licence_sha256": licence_sha,
                  "entries": entries, "files": files, "extracted_bytes": total}


def build(args):
    work, out = builder.guarded(args.work_dir), builder.guarded(args.out_dir)
    if audit.under(work, out) or audit.under(out, work):
        raise audit.Refusal("work/output overlap")
    zig, zig_sha = builder.tool("zig", "0.16.0", builder.ZIG_SHA256)
    opt, opt_sha = builder.tool("wasm-opt", "129", builder.WASM_OPT_SHA256)
    work.mkdir(parents=True)
    out.mkdir(parents=True)
    repo, version, pin, archive_sha, licence_sha, size, prefix = CORE
    core, runtime = source(work, "runtime", repo, version[1:], archive_sha, licence_sha,
                           size, prefix, "https://codeload.github.com/" + repo + "/tar.gz/refs/tags/" + version)
    repo, pin, archive_sha, licence_sha, size, module_sha, module_bytes = CANDIDATES[args.candidate]
    prefix = repo.split("/")[1] + "-" + pin
    grammar, provenance = source(work, "grammar", repo, pin, archive_sha, licence_sha, size,
                                 prefix, "https://codeload.github.com/" + repo + "/tar.gz/" + pin)
    budget = builder.ROOT / "internal/extractor/tsengine/grammars/budget.c"
    env = os.environ.copy()
    env["ZIG_GLOBAL_CACHE_DIR"] = str(work / "zig-global-cache")
    env["ZIG_LOCAL_CACHE_DIR"] = str(work / "zig-local-cache")
    raw, optimized = work / "cpp.wasm", work / "cpp.opt.wasm"
    argv = [zig, "cc", "--target=wasm32-wasi-musl", "-mexec-model=reactor",
            "-I", str(core / "lib/include"), "-I", str(core / "lib/src"),
            str(core / "lib/src/lib.c"), str(budget), "-I", str(grammar / "src"),
            str(grammar / "src/parser.c"), str(grammar / "src/scanner.c"),
            "-Wl,--export=tree_sitter_cpp", "-o", str(raw), "-Oz", "-Wl,--no-entry", "-Wl,--strip-debug",
            *["-Wl,--export=" + name for name in builder.EXPORTS if name != "tree_sitter_cpp"]]
    audit.run(argv, work, work / "zig.stdout", work / "zig.stderr", 900, 1 << 20, env=env)
    audit.run([opt, "-Oz", str(raw), "-o", str(optimized), "--enable-bulk-memory"],
              work, work / "opt.stdout", work / "opt.stderr", 600, 1 << 20, env=env)
    with optimized.open("rb") as stream:
        data = stream.read((32 << 20) + 1)
    if len(data) != module_bytes or hashlib.sha256(data).hexdigest() != module_sha:
        raise audit.Refusal("measured candidate module identity mismatch")
    (out / "cpp.wasm").write_bytes(data)
    if args.gzip:
        with (out / "cpp.wasm.gz").open("xb") as stream:
            with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0, compresslevel=9) as zipped:
                zipped.write(data)
    lock = {"schema": "fathom-cpp-external-audit-v1", "module": {
        "blob": "cpp", "entrypoint": "tree_sitter_cpp", "sha256": module_sha, "bytes": module_bytes, "abi": 15},
        "grammar": provenance, "runtime": runtime,
        "builder_sha256": audit.digest(Path(__file__)), "budget_sha256": audit.digest(budget),
        "toolchain": {"zig_version": "0.16.0", "zig_sha256": zig_sha,
                      "wasm_opt_version": "129", "wasm_opt_sha256": opt_sha}}
    (out / "grammars.lock.json").write_text(json.dumps(lock, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", choices=tuple(CANDIDATES), required=True)
    parser.add_argument("--work-dir", type=Path, required=True)
    parser.add_argument("--out-dir", type=Path, required=True)
    parser.add_argument("--gzip", action="store_true")
    try:
        build(parser.parse_args())
    except (audit.Refusal, OSError) as error:
        parser.exit(1, "build-cpp-candidates: " + type(error).__name__ + "\n")
