#!/usr/bin/env python3
"""Fetch SHA-pinned Terraform calibration inputs into an operator-owned cache.

Run each step only in the serialized preparation slot. This program is not
called by fathom or the language harness. It never runs a provider or a plan.
"""

import argparse
import hashlib
import json
import pathlib
import re
import subprocess
import zipfile


PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())


def command(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        while block := stream.read(1024 * 1024):
            h.update(block)
    return h.hexdigest()


def fetch_git(cache, repository, sha, group):
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("Git pin is not a full SHA")
    target = cache / group / (repository.replace("/", "--") + "--" + sha[:12])
    target.mkdir(parents=True, exist_ok=True)
    if not (target / ".git").is_dir():
        command("git", "init", "--quiet", str(target))
    head = subprocess.run(("git", "rev-parse", "--verify", "HEAD"), cwd=target,
                          text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    current = head.stdout.strip() if head.returncode == 0 else ""
    if current != sha:
        command("git", "fetch", "--depth=1", "https://github.com/" + repository + ".git", sha, cwd=target)
        command("git", "checkout", "--quiet", "--detach", "FETCH_HEAD", cwd=target)
    actual = command("git", "rev-parse", "HEAD", cwd=target)
    if actual != sha or command("git", "status", "--porcelain", cwd=target):
        raise RuntimeError("Git checkout does not match its pin or is dirty: " + repository)
    tracked = subprocess.check_output(("git", "ls-files", "-z"), cwd=target)
    return {"repository": repository, "sha": actual, "tracked_entries": len([p for p in tracked.split(b"\0") if p])}


def fetch_provider(cache, name, pin):
    filename = "terraform-provider-{}_{}_{}.zip".format(name, pin["version"], pin["platform"])
    target = cache / "mirror" / "registry.opentofu.org" / "hashicorp" / name / filename
    target.parent.mkdir(parents=True, exist_ok=True)
    if not target.exists() or digest(target) != pin["sha256"]:
        partial = target.with_name(target.name + ".partial")
        command("curl", "--fail", "--location", "--silent", "--show-error", "--retry", "3", "--output", str(partial), pin["url"])
        if digest(partial) != pin["sha256"]:
            partial.unlink(missing_ok=True)
            raise RuntimeError("provider SHA-256 mismatch: " + name)
        partial.replace(target)
    return {"name": name, "version": pin["version"], "platform": pin["platform"], "sha256": digest(target), "bytes": target.stat().st_size}


def check_tool(cache):
    pin = PINS["opentofu"]
    archive = cache / "tools" / "tofu_{}_{}.zip".format(pin["version"], pin["platform"])
    binary = cache / "tools" / ("tofu-" + pin["version"])
    if digest(archive) != pin["archive_sha256"]:
        raise RuntimeError("OpenTofu release archive SHA-256 mismatch")
    with zipfile.ZipFile(archive) as bundle:
        expected = hashlib.sha256(bundle.read("tofu")).hexdigest()
    if digest(binary) != expected:
        raise RuntimeError("OpenTofu binary differs from pinned archive")
    return {"version": pin["version"], "platform": pin["platform"],
            "archive_sha256": pin["archive_sha256"], "binary_sha256": expected}


def audit_modules(cache):
    data = cache / "native"
    pins = PINS["registry_modules"]
    seen = set()
    count = 0
    manifests = sorted(data.glob("*/data/*/modules/modules.json"))
    for manifest in manifests:
        for module in json.loads(manifest.read_text()).get("Modules", []):
            source = module.get("Source", "")
            match = re.search(r"(?:^|/)terraform-aws-modules/([^/]+)/aws(?:$|/)", source)
            if not match:
                continue
            name = match.group(1)
            key = name + "@" + str(module.get("Version"))
            if key not in pins:
                raise RuntimeError("unmatched registry module version: " + key)
            seen.add(key)
            count += 1
    if not manifests or not seen:
        raise RuntimeError("no native registry module instances found")
    return {"registry_module_instances": count, "distinct_registry_modules": sorted(seen), "manifests": len(manifests)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--step", choices=("tool", "subjects", "modules", "providers", "audit-modules"), action="append", required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    receipt = {}
    for step in args.step:
        if step == "tool":
            receipt[step] = check_tool(cache)
        elif step == "subjects":
            receipt[step] = [fetch_git(cache, x["repository"], x["sha"], "git-subjects") for x in PINS["subjects"]]
        elif step == "modules":
            receipt[step] = []
            for name, pin in sorted(PINS["registry_modules"].items()):
                match = re.fullmatch(r"git::https://github.com/([^?]+)\?ref=([0-9a-f]{40})", pin["source"])
                if not match:
                    raise RuntimeError("module source does not have a pinned Git SHA: " + name)
                receipt[step].append(fetch_git(cache, *match.groups(), "git-modules"))
        elif step == "providers":
            receipt[step] = [fetch_provider(cache, name, pin) for name, pin in sorted(PINS["providers"].items())]
        elif step == "audit-modules":
            receipt[step] = audit_modules(cache)
        print(json.dumps({step: receipt[step]}, sort_keys=True), flush=True)
    output = cache / "terraform-native-preparation.json"
    previous = json.loads(output.read_text()) if output.exists() else {}
    previous.update(receipt)
    output.write_text(json.dumps(previous, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
