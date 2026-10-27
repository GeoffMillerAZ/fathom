#!/usr/bin/env python3
"""Offline, static TypeScript Pulumi triage. Never evaluates index.ts."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

PROFILE = "(version 1) (allow default) (deny network*)"
SANDBOX = Path("/usr/bin/sandbox-exec")
FILES = {
    "index.ts": "5dcbe7491847e62c2142c33377fe8a7ccd48057a401336d296297d82f7fac9fc",
    "tsconfig.json": "55c9d21d0236f323cf26768e38e70f89b06d43a48718cffd6599c957be2e13d2",
    "package.json": "9f669b599182f593061569e6937d3f9b2e94aa7ed2f67f8251a0d9d1a221ea41",
    "package-lock.json": "7fbb4fa42cff9f079eefec8183a5cf1a5606302b997c646587841d2b6b7b6ac3",
    "pins-before-install.json": "b7aaf6bb88e844434f5f181f73a62cdb600b4c9a9a1df9bcd97eebf4d95465cb",
    "node_modules/typescript/bin/tsc": "8d5fa5bd883fec0979fc2004f1fe1d99aef40570155d550eadc0b03b55513bf0",
    "node_modules/@sourcegraph/scip-typescript/dist/src/main.js": "22104bf4323e7667ff0d6f38606937c467af2d6e1fda171203c2065d9b757979",
}
VERSIONS = {
    "@pulumi/aws": "7.0.0",
    "@pulumi/azure-native": "3.0.0",
    "@pulumi/gcp": "9.0.0",
    "@pulumi/pulumi": "3.180.0",
    "typescript": "5.9.3",
    "@sourcegraph/scip-typescript": "0.4.0",
}


def sha256(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def tree_digest(root):
    """Digest every cached entry without following symlinks or emitting names."""
    if root.is_symlink() or not root.is_dir():
        raise ValueError("node_modules must be a real directory")
    digest = hashlib.sha256()
    counts = {"directories": 0, "files": 0, "symlinks": 0}

    def visit(directory):
        for entry in sorted(os.scandir(str(directory)), key=lambda item: item.name):
            path = Path(entry.path)
            relative = path.relative_to(root).as_posix().encode()
            if entry.is_symlink():
                target = os.readlink(str(path))
                if Path(target).is_absolute():
                    raise ValueError("absolute SDK symlink target cannot enter public digest")
                kind, value = "symlinks", target.encode()
            elif entry.is_dir(follow_symlinks=False):
                kind, value = "directories", b""
            elif entry.is_file(follow_symlinks=False):
                kind, value = "files", sha256(path).encode()
            else:
                raise ValueError("unsupported node_modules entry type")
            counts[kind] += 1
            digest.update(kind.encode() + b"\0" + relative + b"\0" + value + b"\n")
            if kind == "directories":
                visit(path)

    visit(root)
    return {"sha256": digest.hexdigest(), "counts": counts}


def validate_cache(root):
    root = Path(root).resolve(strict=True)
    if root.name != "pulumi-probe" or not root.is_dir():
        raise ValueError("cache root must be the existing named pulumi-probe directory")
    for name, expected in FILES.items():
        path = root / name
        if not path.is_file() or sha256(path) != expected:
            raise ValueError("frozen cache input differs: " + name)
    package = json.loads((root / "package.json").read_text())
    lock = json.loads((root / "package-lock.json").read_text())
    pins = json.loads((root / "pins-before-install.json").read_text())
    if package.get("dependencies") != VERSIONS:
        raise ValueError("package dependency pins differ")
    if {p.get("name"): p.get("version") for p in pins} != VERSIONS:
        raise ValueError("registry pin versions differ")
    for pin in pins:
        name = pin["name"]
        installed = json.loads((root / "node_modules" / name / "package.json").read_text())
        locked = lock["packages"].get("node_modules/" + name, {})
        if (installed.get("version") != VERSIONS[name]
                or locked.get("version") != VERSIONS[name]
                or locked.get("integrity") != pin["dist"]["integrity"]):
            raise ValueError("installed/lock/registry pin differs: " + name)
    return root


def installed_tree(root):
    """Resolve staged node_modules so the write-deny covers its physical target."""
    path = root / "node_modules"
    physical = path.resolve(strict=True)
    if not physical.is_dir() or physical == root or root in physical.parents:
        raise ValueError("installed SDK tree must be a separate directory")
    return physical


def sandbox_profile(cache, sdk_tree):
    """Private concrete policy for both the staged cache and physical SDKs."""
    return PROFILE + " (deny file-write* (subpath " + json.dumps(str(cache)) + "))" + \
        " (deny file-write* (subpath " + json.dumps(str(sdk_tree)) + "))"


def clean_environment(scratch, node):
    home = scratch / "home"
    tmp = scratch / "tmp"
    home.mkdir()
    tmp.mkdir()
    return {
        "HOME": str(home), "TMPDIR": str(tmp), "PATH": str(node.parent) + ":/usr/bin:/bin",
        "LANG": "C", "LC_ALL": "C", "CI": "1", "NO_UPDATE_NOTIFIER": "1",
        "NPM_CONFIG_OFFLINE": "true", "NPM_CONFIG_UPDATE_NOTIFIER": "false",
        "XDG_CONFIG_HOME": str(home), "XDG_CACHE_HOME": str(tmp),
    }


def wait_load(seconds=120):
    deadline = time.monotonic() + seconds
    while True:
        load = os.getloadavg()[0]
        if load <= 9:
            return load
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise RuntimeError("load remained above 9")
        time.sleep(min(5, remaining))


def bounded(argv, cwd, env, timeout=120, output_cap=1024 * 1024, profile=PROFILE):
    load = wait_load()
    command = [str(SANDBOX), "-p", profile] + [str(x) for x in argv]
    started = time.monotonic()
    priority_read, priority_write = os.pipe()

    def child_limits():
        resource.setrlimit(resource.RLIMIT_FSIZE, (output_cap, output_cap))
        priority = max(10, os.getpriority(os.PRIO_PROCESS, 0))
        os.setpriority(os.PRIO_PROCESS, 0, priority)
        actual = os.getpriority(os.PRIO_PROCESS, 0)
        if actual < 10:
            raise RuntimeError("child priority below nice 10")
        os.write(priority_write, str(actual).encode() + b"\n")

    with tempfile.TemporaryFile(dir=str(cwd)) as stdout_file, tempfile.TemporaryFile(dir=str(cwd)) as stderr_file:
        try:
            proc = subprocess.Popen(command, cwd=str(cwd), env=env, stdout=stdout_file,
                                    stderr=stderr_file, start_new_session=True,
                                    preexec_fn=child_limits, pass_fds=(priority_write,))
        except BaseException:
            os.close(priority_read)
            raise
        finally:
            os.close(priority_write)
        try:
            priority_message = os.read(priority_read, 32)
        finally:
            os.close(priority_read)
        try:
            actual_priority = int(priority_message.strip())
        except ValueError:
            os.killpg(proc.pid, signal.SIGKILL)
            proc.wait()
            raise RuntimeError("child priority handshake missing")
        if actual_priority < 10:
            os.killpg(proc.pid, signal.SIGKILL)
            proc.wait()
            raise RuntimeError("child priority below nice 10")
        while proc.poll() is None:
            if time.monotonic() - started >= timeout:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
                raise RuntimeError("bounded command timed out")
            if os.fstat(stdout_file.fileno()).st_size + os.fstat(stderr_file.fileno()).st_size > output_cap:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
                raise RuntimeError("bounded command exceeded output cap")
            time.sleep(0.05)
        stdout_file.seek(0)
        stderr_file.seek(0)
        stdout, stderr = stdout_file.read(), stderr_file.read()
    if len(stdout) + len(stderr) > output_cap:
        raise RuntimeError("bounded command exceeded output cap")
    return public_launch_record(proc.returncode, round(time.monotonic() - started, 3),
                                load, actual_priority, stdout, stderr)


def public_launch_record(exit_code, seconds, load, nice, stdout, stderr):
    """Operational counts only: output can echo private paths and timings."""
    return {"exit": exit_code, "seconds": seconds,
            "load_at_launch": load, "actual_nice_at_launch": nice,
            "sandbox_policy": "deny network*; deny writes to staged cache and resolved installed SDK tree"}


def prove_network_denial(cwd, env, profile):
    # First prove a parent-owned listener is reachable, then require EPERM for
    # precisely that live endpoint under the same sandbox profile.
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as listener:
        listener.bind(("127.0.0.1", 0))
        listener.listen(2)
        listener.settimeout(3)
        port = listener.getsockname()[1]
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as client:
            client.settimeout(3)
            client.connect(("127.0.0.1", port))
            accepted, _ = listener.accept()
            accepted.close()
        code = ("import errno,socket,sys\n"
                "try:\n"
                " s=socket.socket()\n"
                " s.connect(('127.0.0.1',int(sys.argv[1])))\n"
                "except OSError as e: sys.exit(0 if e.errno == errno.EPERM else 3)\n"
                "sys.exit(4)\n")
        result = bounded([sys.executable, "-c", code, str(port)], cwd, env,
                         timeout=10, output_cap=4096, profile=profile)
        if result["exit"] != 0:
            raise RuntimeError("reachable loopback TCP was not denied with EPERM")
    return {"unsandboxed_connect": True, "sandbox_denial": "EPERM", "launch": result}


def run(cache_root, scratch, out):
    cache = validate_cache(cache_root)  # Every frozen pin checked before any native launch.
    sdk_tree = installed_tree(cache)
    scratch = Path(scratch).resolve()
    out = Path(out).resolve()
    if scratch.exists() or out.exists() or scratch == out or scratch in out.parents or out in scratch.parents:
        raise ValueError("scratch must be fresh and output separate/nonexistent")
    if cache == scratch or cache in scratch.parents or scratch in cache.parents:
        raise ValueError("scratch must be separate from the read-only cache")
    if out == cache or cache in out.parents or out in cache.parents:
        raise ValueError("output must be separate from the read-only cache")
    if scratch == sdk_tree or sdk_tree in scratch.parents or scratch in sdk_tree.parents:
        raise ValueError("scratch must be separate from the installed SDK tree")
    if out == sdk_tree or sdk_tree in out.parents or out in sdk_tree.parents:
        raise ValueError("output must be separate from the installed SDK tree")
    node_name = shutil.which("node")
    if not node_name or not SANDBOX.is_file():
        raise RuntimeError("node and sandbox-exec are required")
    node = Path(node_name).resolve(strict=True)
    scratch.mkdir(parents=True)
    for name in ("index.ts", "tsconfig.json", "package.json", "package-lock.json"):
        shutil.copyfile(str(cache / name), str(scratch / name))
    (scratch / "node_modules").symlink_to(cache / "node_modules", target_is_directory=True)
    env = clean_environment(scratch, node)
    # The staged path and resolved physical SDK tree can differ through a symlink.
    # Deny writes to both, including transitive Node code, without publishing either path.
    profile = sandbox_profile(cache, sdk_tree)
    tree_before = tree_digest(sdk_tree)
    receipt = {"scope": "TypeScript static SDK imports only", "library_import_index_only": True,
               "full_ingest_measured": False, "pulumi_program_executed": False,
               "python_sdk": "untried/not inventoried; no ceiling",
               "go_sdk": "untried/not inventoried; no ceiling",
               "sdk_pin_evidence": "installed package versions plus lock/registry integrity metadata; tree hash checks local bytes stayed unchanged, not registry tarball identity",
               "node_modules_before": tree_before,
               "inputs_sha256": FILES, "node_sha256": sha256(node), "pins": VERSIONS}
    tsc = cache / "node_modules/typescript/bin/tsc"
    scip = cache / "node_modules/@sourcegraph/scip-typescript/dist/src/main.js"
    try:
        receipt["network_control"] = prove_network_denial(scratch, env, profile)
        receipt["tsc"] = bounded([node, tsc, "--noEmit", "--project", "tsconfig.json"], scratch, env,
                                 profile=profile)
        if receipt["tsc"]["exit"] != 0:
            raise RuntimeError("tsc failed")
        receipt["scip_typescript"] = bounded([node, scip, "index", "--output", "index.scip"], scratch, env,
                                             profile=profile)
        if receipt["scip_typescript"]["exit"] != 0 or not (scratch / "index.scip").is_file():
            raise RuntimeError("scip-typescript failed or emitted no index")
    finally:
        tree_after = tree_digest(sdk_tree)
        if tree_after != tree_before:
            raise RuntimeError("cached node_modules changed during static tools")
        receipt["node_modules_after"] = tree_after
    # Raw SCIP embeds the absolute scratch path in Metadata.project_root.
    # The Go helper hashes a root-normalized, deterministically encoded copy.
    receipt["index_created"] = True
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache-root", required=True)
    parser.add_argument("--scratch", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    run(args.cache_root, args.scratch, args.out)


if __name__ == "__main__":
    main()
