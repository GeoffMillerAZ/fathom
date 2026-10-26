#!/usr/bin/env python3
"""Build both Jinja variants from committed HEAD and compare pinned subjects."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

HERE = Path(__file__).resolve().parents[3]
GRAMMARS = Path("internal/extractor/tsengine/grammars")
LOCK = GRAMMARS / "grammars.lock.json"
BLOB = GRAMMARS / "jinja2.wasm.gz"
DRIVER = "./calibration/oracles/jinja2-meta/parse-gate"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def row(lock):
    return next(item for item in lock["grammars"] if item["blob"] == "jinja2")


def run(cmd, cwd, env, log):
    result = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True)
    log.write_text(result.stdout + result.stderr)
    if result.returncode:
        raise RuntimeError(f"{' '.join(cmd[:2])} failed; see workspace log {log.name}")
    return result.stdout


def blob_numbers(path):
    compressed = path.read_bytes()
    raw = gzip.decompress(compressed)
    return {"blob_sha256": sha(raw), "blob_bytes": len(raw), "gz_bytes": len(compressed)}


def write_lock(path, lock):
    path.write_text(json.dumps(lock, indent=2) + "\n")


def set_digest(items):
    return sha(b"".join(s.encode() + b"\0" for s in sorted(items)))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-kolla", default="workspace/calibration/kolla/kolla/docker")
    parser.add_argument("-kolla-ansible", default="workspace/calibration/kolla-ansible/ansible/roles")
    parser.add_argument("-out", default="workspace/artifacts/jinja-patch-proof.json")
    args = parser.parse_args()
    roots = [Path(args.kolla).resolve(), Path(args.kolla_ansible).resolve()]
    output = Path(args.out).resolve()
    workspace = (HERE / "workspace").resolve()
    if not output.is_relative_to(workspace):
        parser.error("-out must be under workspace/")
    for root in roots:
        if not root.is_dir():
            parser.error("pinned subject root is missing")
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=HERE, text=True).strip()
    for executable in ("git", "go", "zig", "wasm-opt", "npx", "jq", "curl"):
        if not shutil.which(executable):
            parser.error(f"{executable} is required on PATH")
    # The snapshot must include this entrypoint and the lock-declared patch.
    tracked = subprocess.check_output(["git", "ls-tree", "-r", "--name-only", "HEAD"], cwd=HERE, text=True)
    for required in ("calibration/oracles/jinja2-meta/prove_patch.py", "calibration/oracles/jinja2-meta/parse-gate/main.go"):
        if required not in tracked.splitlines():
            parser.error(f"commit {required} before running this proof")

    for directory in (workspace / "tmp", workspace / "logs", output.parent):
        directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="jinja-patch-proof-", dir=workspace / "tmp") as temp:
        scratch = Path(temp)
        logs = workspace / "logs"
        snapshot = scratch / "source"
        snapshot.mkdir()
        archive = scratch / "head.tar"
        with archive.open("wb") as dst:
            subprocess.run(["git", "archive", "--format=tar", "HEAD"], cwd=HERE, stdout=dst, check=True)
        with tarfile.open(archive) as src:
            for member in src:
                target = (snapshot / member.name).resolve()
                if not target.is_relative_to(snapshot):
                    raise RuntimeError("unsafe committed archive member")
                if member.issym():
                    if not (target.parent / member.linkname).resolve().is_relative_to(snapshot):
                        raise RuntimeError("unsafe committed archive link")
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.symlink_to(member.linkname)
                elif member.islnk():
                    raise RuntimeError("unsupported committed archive hard link")
                elif member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                elif member.isfile():
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with src.extractfile(member) as source, target.open("wb") as destination:
                        shutil.copyfileobj(source, destination)
                else:
                    raise RuntimeError("unsupported committed archive member")
        lock_path = snapshot / LOCK
        committed_lock = json.loads(lock_path.read_text())
        patched_row = row(committed_lock)
        if len(patched_row.get("patches", [])) != 1:
            raise RuntimeError("expected one committed Jinja source patch")
        patch_path = snapshot / GRAMMARS / patched_row["patches"][0]["path"]
        if sha(patch_path.read_bytes()) != patched_row["patches"][0]["sha256"]:
            raise RuntimeError("committed source patch digest differs from lock")
        expected = {key: patched_row[key] for key in ("blob_sha256", "blob_bytes", "gz_bytes")}
        env = os.environ.copy()
        for key, folder in {
            "TMPDIR": "tmp", "GOTMPDIR": "tmp", "GOCACHE": "go-cache",
            "GOMODCACHE": "gomodcache", "ZIG_GLOBAL_CACHE_DIR": "zig-cache",
            "ZIG_LOCAL_CACHE_DIR": "zig-local", "npm_config_cache": "npm-cache",
        }.items():
            location = workspace / folder
            location.mkdir(parents=True, exist_ok=True)
            env.setdefault(key, str(location))
        env["GOWORK"] = "off"
        env.setdefault("XDG_CACHE_HOME", str(workspace / "xdg-cache"))
        (workspace / "xdg-cache").mkdir(exist_ok=True)

        def build(label):
            run(["bash", str(snapshot / GRAMMARS / "build.sh"), "jinja2", str(snapshot / GRAMMARS)],
                snapshot, env, logs / f"jinja-{commit[:12]}-{label}-build.log")
            return blob_numbers(snapshot / BLOB)

        def count(label):
            details = snapshot / "workspace" / "tmp" / f"{label}-details.json"
            details.parent.mkdir(parents=True, exist_ok=True)
            command = ["go", "run", DRIVER, "-kolla", str(roots[0]), "-kolla-ansible", str(roots[1]), "-details", str(details)]
            summary = json.loads(run(command, snapshot, env, logs / f"jinja-{commit[:12]}-{label}-count.log"))
            private = {item["name"]: item for item in json.loads(details.read_text())}
            return summary, private

        baseline_lock = json.loads(json.dumps(committed_lock))
        row(baseline_lock)["patches"] = []
        write_lock(lock_path, baseline_lock)
        baseline_blob = build("baseline")
        row(baseline_lock).update(baseline_blob)
        write_lock(lock_path, baseline_lock)  # I55 permits the measured baseline blob.
        baseline, before = count("baseline")

        write_lock(lock_path, committed_lock)
        patched_blob = build("patched")
        if patched_blob != expected:
            raise RuntimeError("patched production build differs from committed Jinja lock")
        patched, after = count("patched")
        comparison = []
        for old, new in zip(baseline, patched):
            if old["name"] != new["name"] or old["files"] != new["files"] or old["bytes"] != new["bytes"] or old["content_sha256"] != new["content_sha256"]:
                raise RuntimeError("subject changed between grammar runs")
            name = old["name"]
            old_errors, new_errors = set(before[name]["errors"]), set(after[name]["errors"])
            old_faults, new_faults = set(before[name]["faults"]), set(after[name]["faults"])
            if new_errors - old_errors or new_faults or old_faults:
                raise RuntimeError("patch introduced an error or a parse/query fault")
            repaired = old_errors - new_errors
            comparison.append({"name": name, "repaired": len(repaired), "repaired_set_sha256": set_digest(repaired), "new_errors": 0})
        proof = {
            "source_commit": commit,
            "command": "python3 calibration/oracles/jinja2-meta/prove_patch.py",
            "builder": "internal/extractor/tsengine/grammars/build.sh jinja2",
            "baseline_blob": baseline_blob, "patched_blob": patched_blob,
            "baseline": baseline, "patched": patched, "comparison": comparison,
        }
        output.write_text(json.dumps(proof, indent=2) + "\n")
        print(json.dumps(proof, indent=2))


if __name__ == "__main__":
    main()
