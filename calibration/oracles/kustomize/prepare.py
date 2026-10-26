#!/usr/bin/env python3
"""Fetch explicit Kustomize pins into an operator-owned calibration cache."""

import argparse
import hashlib
import io
import json
import pathlib
import subprocess
import tarfile


PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())


def run(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def checkout(cache, pin):
    directory = cache / "git-subjects" / (pin["repository"].replace("/", "--") + "--" + pin["sha"][:12])
    if not (directory / ".git").exists():
        directory.mkdir(parents=True, exist_ok=True)
        run("git", "init", "-q", str(directory))
        run("git", "-C", str(directory), "remote", "add", "origin",
            "https://github.com/" + pin["repository"] + ".git")
    if run("git", "-C", str(directory), "status", "--porcelain"):
        raise RuntimeError("dirty subject checkout: " + pin["repository"])
    run("git", "-C", str(directory), "-c", "core.hooksPath=/dev/null",
        "fetch", "--depth=1", "origin", pin["sha"])
    run("git", "-C", str(directory), "-c", "core.hooksPath=/dev/null",
        "checkout", "--detach", pin["sha"])
    if run("git", "-C", str(directory), "rev-parse", "HEAD") != pin["sha"]:
        raise RuntimeError("subject commit differs: " + pin["repository"])
    return {"repository": pin["repository"], "sha": pin["sha"],
            "directory": str(directory), "subpath": pin.get("subpath", ".")}


def tool(cache):
    pin = PINS["native"]
    archive = cache / "tools" / "kustomize-v5.8.1-darwin-arm64.tar.gz"
    archive.parent.mkdir(parents=True, exist_ok=True)
    if not archive.exists():
        subprocess.run(("curl", "-fL", "--retry", "2", pin["url"], "-o", str(archive)), check=True)
    data = archive.read_bytes()
    if hashlib.sha256(data).hexdigest() != pin["archive_sha256"]:
        raise RuntimeError("native Kustomize archive checksum differs")
    binary = cache / "tools" / "kustomize-5.8.1"
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as source:
        members = [member for member in source.getmembers() if member.name == "kustomize" and member.isfile()]
        if len(members) != 1:
            raise RuntimeError("native archive lacks one regular kustomize binary")
        binary.write_bytes(source.extractfile(members[0]).read())
    binary.chmod(0o755)
    version = run(str(binary), "version")
    if version != pin["version"]:
        raise RuntimeError("native Kustomize version differs: " + version)
    return {"version": version, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "archive_sha256": pin["archive_sha256"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", required=True, type=pathlib.Path)
    parser.add_argument("--step", choices=("tool", "subjects"), action="append", required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    result = {}
    if "tool" in args.step:
        result["native"] = tool(cache)
    if "subjects" in args.step:
        result["subjects"] = [checkout(cache, pin) for pin in PINS["subjects"]]
    directory = cache / "kustomize-r1"
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "preparation.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"steps": args.step, "subject_pins": len(PINS["subjects"])}, sort_keys=True))


if __name__ == "__main__":
    main()
