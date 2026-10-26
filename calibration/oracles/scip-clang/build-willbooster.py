#!/usr/bin/env python3
"""Build one private, source-pinned WillBooster C++ WASM audit module.

This never edits the embedded grammar lock or product parser. Outputs are
confined to fresh ignored workspace directories and contain counts/hashes only.
"""

import argparse
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tarfile
import urllib.request

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
spec = importlib.util.spec_from_file_location("cpp_audit", HERE / "cpp-audit.py")
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)

SOURCES = (
    ("willbooster-tree-sitter-cpp", "6.0.2", "8c892195d9e44d816b4cc5ffa9f8cf53d2d1c00b3030feb0d35e612ae5f34bcc", "2bb45b424e2a497412ab9eb903554eebb9847e4f70bc81eecd2609817301679f", 1365727),
    ("willbooster-tree-sitter", "7.0.5", "166cf42202927500081763e65e63d3c0bc9273e05ba998496756fff7569de9c5", "f1e1e62c6e1119d27f5ba38a0cbdae0788fc00f1d0bdf71cb7710ca5d2421d1c", 246632),
)
EXPORTS = (
    "malloc free calloc realloc ts_parser_new ts_parser_delete ts_parser_set_language "
    "ts_parser_parse_string ts_parser_reset fathom_parse_bounded "
    "fathom_query_cursor_exec_bounded fathom_take_cancelled fathom_node_end_byte "
    "ts_tree_root_node ts_tree_delete ts_node_child_count ts_node_named_child_count "
    "ts_node_child ts_node_named_child ts_node_type ts_node_symbol "
    "ts_node_start_byte ts_node_end_byte ts_node_start_point ts_node_end_point "
    "ts_node_is_named ts_node_is_null ts_node_field_name_for_child "
    "ts_language_symbol_name ts_language_abi_version ts_query_new ts_query_delete "
    "ts_query_cursor_new ts_query_cursor_delete ts_query_cursor_exec "
    "ts_query_cursor_next_match ts_query_capture_name_for_id tree_sitter_cpp"
).split()
MODULE_SHA256 = "e39bd1cf09c15b52b9495b8e19e6b0627f41312f1e60453f4427695baf29e684"
MODULE_BYTES = 3852285
ZIG_SHA256 = "e6cd688d25664983833aae272f501d4bceeae304875b8f1741209d15fd13a4ec"
WASM_OPT_SHA256 = "2bf3eadf0b156ab57982f580b9603b5034a22878e08a9516310c14932bd533d4"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def fetch(crate, version, expected, size, dest):
    url = f"https://static.crates.io/crates/{crate}/{crate}-{version}.crate"
    request = urllib.request.Request(url, headers={"User-Agent": "fathom-cpp-audit/1"})
    with urllib.request.urlopen(request, timeout=60) as response:
        data = response.read(size + 1)
        if len(data) != size or response.read(1):
            raise audit.Refusal("crate archive size mismatch")
    if sha(data) != expected:
        raise audit.Refusal("crate archive SHA mismatch")
    dest.write_bytes(data)


def extract(archive, dest, prefix):
    files = entries = total = 0
    with tarfile.open(archive, "r:gz") as tar:
        for member in tar:
            entries += 1
            if entries > 1000:
                raise audit.Refusal("crate entry cap")
            parts = Path(member.name).parts
            if not parts or parts[0] != prefix or len(parts) > 17 or any(part in ("", ".", "..") for part in parts):
                raise audit.Refusal("crate path refused")
            if member.issym() or member.islnk() or not (member.isfile() or member.isdir()):
                raise audit.Refusal("crate entry type refused")
            target = dest.joinpath(*parts[1:])
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            files += 1
            total += member.size
            if member.size > 64 << 20 or total > 128 << 20:
                raise audit.Refusal("crate extracted byte cap")
            target.parent.mkdir(parents=True, exist_ok=True)
            with tar.extractfile(member) as source, target.open("xb") as output:
                copied = shutil.copyfileobj(source, output, 1 << 20)
            if target.stat().st_size != member.size:
                raise audit.Refusal("crate extracted size mismatch")
    return entries, files, total


def tool(name, version, expected_sha):
    path = shutil.which(name)
    if path is None:
        raise audit.Refusal("required build tool missing")
    resolved = Path(path).resolve(strict=True)
    if not stat.S_ISREG(resolved.stat().st_mode):
        raise audit.Refusal("build tool type refused")
    result = subprocess.run([path, "version" if name == "zig" else "--version"], capture_output=True, text=True, timeout=15)
    if result.returncode or version not in result.stdout:
        raise audit.Refusal("build tool version mismatch")
    digest = audit.digest(resolved)
    if digest != expected_sha:
        raise audit.Refusal("build tool hash mismatch")
    return path, digest


def guarded(path):
    path = path.absolute()
    if path.resolve() != path:
        raise audit.Refusal("output path contains symlink")
    if path.exists() or path.is_symlink():
        raise audit.Refusal("output already exists")
    roots = tuple(root.resolve(strict=True) for root in audit.workspace_roots())
    if not any(audit.under(path, root) and path != root for root in roots):
        raise audit.Refusal("output outside ignored workspace")
    return path


def build(args):
    work, out = guarded(args.work_dir), guarded(args.out_dir)
    if audit.under(work, out) or audit.under(out, work):
        raise audit.Refusal("work and output overlap")
    zig, zig_sha = tool("zig", "0.16.0", ZIG_SHA256)
    wasm_opt, opt_sha = tool("wasm-opt", "129", WASM_OPT_SHA256)
    work.mkdir(parents=True)
    out.mkdir(parents=True)
    src = work / "source"
    src.mkdir()
    provenance = []
    for crate, version, archive_sha, license_sha, size in SOURCES:
        archive = work / f"{crate}-{version}.crate"
        fetch(crate, version, archive_sha, size, archive)
        directory = src / f"{crate}-{version}"
        directory.mkdir()
        entries, files, total = extract(archive, directory, f"{crate}-{version}")
        if audit.digest(directory / "LICENSE") != license_sha:
            raise audit.Refusal("crate MIT licence mismatch")
        provenance.append({"crate": crate, "version": version, "archive_sha256": archive_sha,
                           "licence": "MIT", "licence_sha256": license_sha,
                           "archive_bytes": size, "entries": entries, "files": files, "extracted_bytes": total})
    grammar, runtime = (src / f"{crate}-{version}" for crate, version, *_ in SOURCES)
    budget = ROOT / "internal/extractor/tsengine/grammars/budget.c"
    assertion = work / "abi_assert.c"
    assertion.write_text("""#include <tree_sitter/api.h>\n_Static_assert(sizeof(TSNode) == 24, \"TSNode\");\n_Static_assert(sizeof(TSQueryMatch) == 12, \"TSQueryMatch\");\n_Static_assert(sizeof(TSQueryCapture) == 28, \"TSQueryCapture\");\n#ifdef TREE_SITTER_LANGUAGE_VERSION\n_Static_assert(TREE_SITTER_LANGUAGE_VERSION == 0x57420006, \"fork ABI\");\n#endif\n""")
    raw = work / "cpp.wasm"
    flags = [f"-Wl,--export={name}" for name in EXPORTS]
    argv = [zig, "cc", "--target=wasm32-wasi-musl", "-mexec-model=reactor",
            "-I", str(runtime / "include"), "-I", str(runtime / "src"),
            "-I", str(grammar / "src"), str(runtime / "src/lib.c"), str(budget),
            str(grammar / "src/parser.c"), str(grammar / "src/scanner.c"), str(assertion),
            "-o", str(raw), "-Oz", "-Werror=incompatible-pointer-types",
            "-Wl,--no-entry", "-Wl,--strip-debug", *flags]
    audit.run(argv, ROOT, work / "zig.stdout", work / "zig.stderr", 900, 1 << 20)
    optimized = work / "cpp.opt.wasm"
    audit.run([wasm_opt, "-Oz", str(raw), "-o", str(optimized), "--enable-bulk-memory"],
              ROOT, work / "opt.stdout", work / "opt.stderr", 600, 1 << 20)
    data = optimized.read_bytes()
    if len(data) != MODULE_BYTES or sha(data) != MODULE_SHA256:
        raise audit.Refusal("measured WASM module identity mismatch")
    (out / "cpp.wasm").write_bytes(data)
    if args.gzip:
        with (out / "cpp.wasm.gz").open("xb") as output:
            with gzip.GzipFile(filename="", mode="wb", fileobj=output, mtime=0, compresslevel=9) as zipped:
                zipped.write(data)
    lock = {"schema": "fathom-cpp-external-audit-v1",
            "module": {"blob": "cpp", "entrypoint": "tree_sitter_cpp", "sha256": sha(data),
                       "bytes": len(data), "abi": 0x57420006},
            "grammar": provenance[0], "runtime": provenance[1],
            "builder_sha256": audit.digest(Path(__file__)), "budget_sha256": audit.digest(budget),
            "toolchain": {"zig_version": "0.16.0", "zig_sha256": zig_sha,
                          "wasm_opt_version": "129", "wasm_opt_sha256": opt_sha}}
    (out / "grammars.lock.json").write_text(json.dumps(lock, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--work-dir", required=True, type=Path)
    parser.add_argument("--out-dir", required=True, type=Path)
    parser.add_argument("--gzip", action="store_true")
    try:
        build(parser.parse_args())
    except (audit.Refusal, OSError, tarfile.TarError) as error:
        parser.exit(1, f"build-willbooster: {type(error).__name__}: {error}\n")
