#!/usr/bin/env python3
"""Install exact pinned module trees into OpenTofu's per-root module cache.

The scanner only finds static module source/version attributes. OpenTofu still
supplies all native configuration and graph truth. No subject source is edited.
"""

import argparse
import functools
import hashlib
import json
import pathlib
import re
import subprocess


PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())
SKIP_DIRS = {"vendor", "testdata", "node_modules"}
REGISTRY = re.compile(r"^(?:registry\.opentofu\.org/)?terraform-aws-modules/([^/]+)/aws(?://(.+))?$")


def within(path, root):
    return path == root or root in path.parents


def roots(subject):
    found = set()
    for path in subject.rglob("*.tf"):
        relative = path.relative_to(subject)
        if any(part.startswith(".") or part in SKIP_DIRS for part in relative.parts[:-1]):
            continue
        found.add(path.parent.relative_to(subject).as_posix())
    return sorted(found)


def version_tuple(value):
    if not re.fullmatch(r"\d+\.\d+\.\d+", value):
        raise ValueError("non-semver module version: " + value)
    return tuple(map(int, value.split(".")))


def satisfies(version, constraint):
    value = version_tuple(version)
    if re.fullmatch(r"\d+\.\d+\.\d+", constraint):
        return value == version_tuple(constraint)
    match = re.fullmatch(r"~>\s*(\d+)\.(\d+)(?:\.(\d+))?", constraint)
    if match:
        major, minor, patch = match.groups()
        lower = (int(major), int(minor), int(patch or 0))
        upper = (int(major) + 1, 0, 0) if patch is None else (int(major), int(minor) + 1, 0)
        return lower <= value < upper
    match = re.fullmatch(r">=\s*(\d+)\.(\d+)(?:\.(\d+))?", constraint)
    if match:
        major, minor, patch = match.groups()
        return value >= (int(major), int(minor), int(patch or 0))
    raise ValueError("unsupported module version constraint: " + constraint)


def pinned_module(cache, source, constraint):
    match = REGISTRY.fullmatch(source)
    if not match or not constraint:
        raise ValueError("unsupported or unversioned registry module source: " + source)
    name, subdir = match.groups()
    options = [(pin["version"], pin) for key, pin in PINS["registry_modules"].items()
               if key.startswith(name + "@") and satisfies(pin["version"], constraint)]
    if not options:
        raise ValueError("no exact module pin for " + source + " " + constraint)
    version, pin = max(options, key=lambda option: version_tuple(option[0]))
    match = re.fullmatch(r"git::https://github.com/([^?]+)\?ref=([0-9a-f]{40})", pin["source"])
    if not match:
        raise ValueError("module pin lacks exact Git source: " + name)
    repo, sha = match.groups()
    checkout = cache / "git-modules" / (repo.replace("/", "--") + "--" + sha[:12])
    if subprocess.check_output(("git", "-C", str(checkout), "rev-parse", "HEAD"), text=True).strip() != sha:
        raise ValueError("module checkout differs from pin: " + name)
    if subprocess.check_output(("git", "-C", str(checkout), "status", "--porcelain")):
        raise ValueError("module checkout is dirty: " + name)
    target = (checkout / (subdir or ".")).resolve()
    if not within(target, checkout.resolve()) or not target.is_dir():
        raise ValueError("module subdirectory escapes its pin: " + name)
    return target, version


@functools.lru_cache(maxsize=4096)
def scan(scanner, directory):
    raw = subprocess.check_output((str(scanner), "-dir", str(directory)))
    return json.loads(raw)


def module_manifest(cache, scanner, subject, scratch, root, root_index):
    caller = (subject / root).resolve()
    target_root = (scratch / "src" / root).resolve()
    records = [{"Key": "", "Source": "", "Dir": "."}]
    def visit(source_dir, installed_dir, key, ancestors):
        if len(records) > 512 or len(ancestors) > 16:
            raise ValueError("module fan-out/depth cap exceeded")
        for call in scan(scanner, source_dir):
            child_key = (key + "." if key else "") + call["name"]
            source, constraint = call["source"], call.get("version", "")
            if source.startswith(("./", "../")):
                if constraint:
                    raise ValueError("local module has registry version: " + child_key)
                source_child = (source_dir / source).resolve()
                installed_child = (installed_dir / source).resolve()
                if within(source_dir, subject.resolve()):
                    allowed = subject.resolve()
                else:
                    module_cache = (cache / "git-modules").resolve()
                    allowed = next((parent for parent in (source_dir, *source_dir.parents)
                                    if parent.parent == module_cache), None)
                    if allowed is None:
                        raise ValueError("local module source has no pinned repository: " + child_key)
                if not within(source_child, allowed) or not source_child.is_dir():
                    raise ValueError("local module leaves pinned source: " + child_key)
                version = ""
            else:
                source_child, version = pinned_module(cache, source, constraint)
                installed_child = scratch / "data" / f"{root_index:03d}" / "modules" / child_key
                installed_child.parent.mkdir(parents=True, exist_ok=True)
                if installed_child.is_symlink():
                    if installed_child.resolve() != source_child:
                        raise ValueError("module cache link differs from exact pin: " + child_key)
                elif installed_child.exists():
                    raise ValueError("module cache path is not a pinned link: " + child_key)
                else:
                    installed_child.symlink_to(source_child, target_is_directory=True)
            if source_child in ancestors:
                raise ValueError("cyclic local module: " + child_key)
            record = {"Key": child_key, "Source": source, "Dir": str(installed_child)}
            if version:
                record["Version"] = version
            records.append(record)
            visit(source_child, installed_child, child_key, ancestors | {source_child})
    visit(caller, target_root, "", {caller})
    records.sort(key=lambda item: item["Key"])
    return {"Modules": records}


def preseed(cache, scanner, subject, scratch):
    summary = []
    for number, root in enumerate(roots(subject)):
        manifest = module_manifest(cache, scanner, subject, scratch, root, number)
        path = scratch / "data" / f"{number:03d}" / "modules" / "modules.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists():
            if json.loads(path.read_text()) != manifest:
                raise ValueError("existing native module manifest differs from exact pins")
        else:
            path.write_text(json.dumps(manifest, separators=(",", ":")) + "\n")
        summary.append({"root_index": number, "modules": len(manifest["Modules"]) - 1,
                        "manifest_sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--scanner-bin", type=pathlib.Path, required=True)
    parser.add_argument("--subject", type=pathlib.Path, required=True)
    parser.add_argument("--scratch", type=pathlib.Path, required=True)
    args = parser.parse_args()
    summary = preseed(args.cache.resolve(), args.scanner_bin.resolve(),
                      args.subject.resolve(), args.scratch.resolve())
    print(json.dumps({"roots": len(summary), "module_instances": sum(x["modules"] for x in summary),
                      "manifest_receipts": summary}, sort_keys=True))


if __name__ == "__main__":
    main()
