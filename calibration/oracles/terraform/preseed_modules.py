#!/usr/bin/env python3
"""Pin registry modules, then verify local destinations from OpenTofu init.

The scanner proposes expected module keys and paths. Only OpenTofu's installer
may supply local Dir entries consumed by the oracle. No subject source is edited.
"""

import argparse
import functools
import hashlib
import json
import pathlib
import posixpath
import re
import shutil
import subprocess
import os


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
                installed_child = pathlib.Path(os.path.abspath(os.path.normpath(str(installed_dir / source))))
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


def lexical_dir(record, working_dir):
    path = pathlib.Path(record["Dir"])
    return pathlib.Path(os.path.abspath(os.path.normpath(
        str(path if path.is_absolute() else working_dir / path))))


def canonical_registry_source(source):
    # OpenTofu expands the implicit default host in three-part module addresses.
    package = source.split("//", 1)[0]
    if len(package.split("/")) == 3 and REGISTRY.fullmatch(source):
        return "registry.opentofu.org/" + source
    return source


def sources_agree(native_source, proposed_source):
    if native_source == proposed_source:
        return True
    if native_source.startswith(("./", "../")) and proposed_source.startswith(("./", "../")):
        return posixpath.normpath(native_source) == posixpath.normpath(proposed_source)
    return canonical_registry_source(native_source) == canonical_registry_source(proposed_source)


def has_registry_ancestor(key, expected):
    parts = key.split(".")
    for length in range(1, len(parts)):
        ancestor = expected.get(".".join(parts[:length]))
        if ancestor and not ancestor["Source"].startswith(("./", "../")):
            return True
    return False


def registry_alias_agrees(proposed, actual, working_dir):
    try:
        return lexical_dir(proposed, working_dir).resolve(strict=True) == \
               lexical_dir(actual, working_dir).resolve(strict=True)
    except OSError:
        return False


def check_native_manifest(proposal, native, working_dir, complete=True):
    expected = {record["Key"]: record for record in proposal["Modules"]}
    actual = native.get("Modules")
    if not isinstance(actual, list) or len(actual) > len(expected) or (complete and len(actual) != len(expected)):
        raise ValueError("native module population differs from proposal")
    observed = {}
    local_count = 0
    for record in actual:
        if not isinstance(record, dict) or not isinstance(record.get("Key"), str):
            raise ValueError("malformed native module record")
        key = record["Key"]
        if key in observed or key not in expected:
            raise ValueError("duplicate or unexpected native module key: " + key)
        observed[key] = record
        proposed = expected[key]
        if (not sources_agree(record.get("Source", ""), proposed["Source"])
                or record.get("Version", "") != proposed.get("Version", "")):
            raise ValueError("native module source/version differs from pin: " + key)
        if lexical_dir(record, working_dir) != lexical_dir(proposed, working_dir):
            if not (key and proposed["Source"].startswith(("./", "../"))
                    and has_registry_ancestor(key, expected)
                    and registry_alias_agrees(proposed, record, working_dir)):
                raise ValueError("native module destination differs from proposal: " + key)
        if key and proposed["Source"].startswith(("./", "../")):
            local_count += 1
    if complete and set(observed) != set(expected):
        raise ValueError("native module population differs from proposal")
    return local_count


def count_registry_aliases(proposal, native, working_dir):
    expected = {record["Key"]: record for record in proposal["Modules"]}
    return sum(1 for record in native["Modules"]
               if record["Key"] and expected[record["Key"]]["Source"].startswith(("./", "../"))
               and has_registry_ancestor(record["Key"], expected)
               and lexical_dir(record, working_dir) != lexical_dir(expected[record["Key"]], working_dir))


def pinned_registry_record(record):
    return {**record, "Source": canonical_registry_source(record["Source"])}


def restore_missing_registry(proposal, native):
    observed = {record["Key"] for record in native["Modules"]}
    missing = [pinned_registry_record(record) for record in proposal["Modules"]
               if record["Key"] and not record["Source"].startswith(("./", "../"))
               and record["Key"] not in observed]
    native["Modules"].extend(missing)
    return len(missing)


def registry_link_counts(proposal):
    registry = [pathlib.Path(record["Dir"]) for record in proposal["Modules"]
                if record["Key"] and not record["Source"].startswith(("./", "../"))]
    return (sum(path.is_symlink() and path.exists() for path in registry),
            sum(not path.is_symlink() and not path.exists() for path in registry),
            sum((path.is_symlink() and not path.exists()) or
                (path.exists() and not path.is_symlink()) for path in registry))


def pinned_registry_links(cache, proposal, modules_dir, restore):
    restored = 0
    for record in proposal["Modules"]:
        if not record["Key"] or record["Source"].startswith(("./", "../")):
            continue
        link = pathlib.Path(record["Dir"])
        if link.parent != modules_dir:
            raise ValueError("registry module link leaves root cache: " + record["Key"])
        target, version = pinned_module(cache, record["Source"], record["Version"])
        if version != record["Version"]:
            raise ValueError("registry module version differs from pin: " + record["Key"])
        if link.is_symlink():
            if link.resolve() != target:
                raise ValueError("registry module link differs from exact pin: " + record["Key"])
        elif link.exists():
            raise ValueError("registry module cache path is not a pinned link: " + record["Key"])
        elif restore:
            link.symlink_to(target, target_is_directory=True)
            restored += 1
        else:
            raise ValueError("registry module link missing after native init: " + record["Key"])
    return restored


def installer_env(scratch, data_dir, cli_config):
    dead_proxy = "http://localhost:9"
    return {"HOME": str(scratch / "home"), "TMPDIR": str(scratch / "tmp"),
            "XDG_CONFIG_HOME": str(scratch / "home" / ".config"),
            "XDG_CACHE_HOME": str(scratch / "home" / ".cache"),
            "XDG_DATA_HOME": str(scratch / "home" / ".local" / "share"),
            "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "LANG": "C", "LC_ALL": "C",
            "HTTP_PROXY": dead_proxy, "HTTPS_PROXY": dead_proxy, "ALL_PROXY": dead_proxy,
            "http_proxy": dead_proxy, "https_proxy": dead_proxy, "all_proxy": dead_proxy,
            "NO_PROXY": "", "no_proxy": "", "AWS_EC2_METADATA_DISABLED": "true",
            "TF_CLI_CONFIG_FILE": str(cli_config), "TF_DATA_DIR": str(data_dir),
            "TF_IN_AUTOMATION": "1", "TF_INPUT": "0", "CHECKPOINT_DISABLE": "1"}


def tf_contents(tree):
    return {path.relative_to(tree).as_posix(): path.read_bytes()
            for path in tree.rglob("*.tf")
            if ".git" not in path.relative_to(tree).parts}


def preseed(cache, scanner, subject, scratch, tofu, mirror):
    scratch.mkdir(parents=True, exist_ok=True)
    source_copy = scratch / "src"
    if not source_copy.exists():
        shutil.copytree(subject, source_copy, ignore=shutil.ignore_patterns(".git"))
    if tf_contents(source_copy) != tf_contents(subject):
        raise ValueError("scratch .tf path set or bytes differ from pinned subject")
    for directory in ("home", "tmp", "plugin-cache"):
        (scratch / directory).mkdir(exist_ok=True)
    cli_config = scratch / "tofurc"
    cli_config.write_text('provider_installation {\n  filesystem_mirror {\n    path    = "' +
                          str(mirror) + '"\n    include = ["*/*/*"]\n  }\n}\nplugin_cache_dir = "' +
                          str(scratch / "plugin-cache") + '"\n')
    summary = []
    for number, root in enumerate(roots(subject)):
        manifest = module_manifest(cache, scanner, subject, scratch, root, number)
        path = scratch / "data" / f"{number:03d}" / "modules" / "modules.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        working_dir = source_copy / root
        registry_only = {"Modules": [pinned_registry_record(record) for record in manifest["Modules"]
                                     if record["Key"] == "" or not record["Source"].startswith(("./", "../"))]}
        # An existing local Dir is not evidence: plain init retains it unchanged.
        # Drop every local record, including one left by an earlier oracle run.
        path.write_text(json.dumps(registry_only, separators=(",", ":")) + "\n")
        prior_local_count = -1
        for attempt in range(17):  # 16 ancestor limit plus one final complete pass
            result = subprocess.run((str(tofu), "init", "-backend=false", "-input=false", "-no-color"),
                                    cwd=working_dir, env=installer_env(scratch, path.parent.parent, cli_config),
                                    capture_output=True, text=True, timeout=600, check=False)
            native = json.loads(path.read_text())
            local_count = check_native_manifest(manifest, native, working_dir, complete=False)
            if result.returncode == 0:
                check_native_manifest(manifest, native, working_dir)
                pinned_registry_links(cache, manifest, path.parent, restore=False)
                break
            restored = restore_missing_registry(manifest, native)
            links_restored = pinned_registry_links(cache, manifest, path.parent, restore=True)
            if (local_count <= prior_local_count and not links_restored) or (not restored and not links_restored):
                linked, missing, wrong = registry_link_counts(manifest)
                raise RuntimeError("native module install made no bounded progress for root " + root +
                                   f" (attempt={attempt + 1}, native_locals={local_count}, " +
                                   f"prior_locals={prior_local_count}, restored_registry={restored}, " +
                                   f"restored_links={links_restored}, " +
                                   f"registry_links={linked}, missing_links={missing}, wrong_links={wrong}): " +
                                   result.stderr[-1200:])
            prior_local_count = local_count
            path.write_text(json.dumps(native, separators=(",", ":")) + "\n")
        else:
            raise RuntimeError("native module install exceeded depth bound for root " + root)
        native_bytes = path.read_bytes()
        alias_count = count_registry_aliases(manifest, json.loads(native_bytes), working_dir)
        summary.append({"root_index": number, "modules": len(manifest["Modules"]) - 1,
                        "native_local_agreements": local_count,
                        "native_registry_alias_agreements": alias_count,
                        "native_manifest_sha256": hashlib.sha256(native_bytes).hexdigest()})
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--scanner-bin", type=pathlib.Path, required=True)
    parser.add_argument("--subject", type=pathlib.Path, required=True)
    parser.add_argument("--scratch", type=pathlib.Path, required=True)
    parser.add_argument("--tofu", type=pathlib.Path, required=True)
    parser.add_argument("--mirror", type=pathlib.Path, required=True)
    args = parser.parse_args()
    summary = preseed(args.cache.resolve(), args.scanner_bin.resolve(),
                      args.subject.resolve(), args.scratch.resolve(),
                      args.tofu.resolve(), args.mirror.resolve())
    print(json.dumps({"roots": len(summary), "module_instances": sum(x["modules"] for x in summary),
                      "native_local_agreements": sum(x["native_local_agreements"] for x in summary),
                      "native_registry_alias_agreements": sum(x["native_registry_alias_agreements"] for x in summary),
                      "manifest_receipts": summary}, sort_keys=True))


if __name__ == "__main__":
    main()
