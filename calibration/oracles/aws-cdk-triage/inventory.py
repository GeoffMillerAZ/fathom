#!/usr/bin/env python3
"""Read-only scoped AWS CDK inventory; aggregate counts only."""
import json
import os
from pathlib import Path
import shutil
import argparse

parser = argparse.ArgumentParser()
parser.add_argument("--cache-root", type=Path, required=True)
parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[3])
args = parser.parse_args()
repo = args.repo_root.resolve()
cache = args.cache_root.resolve()
roots = {"tools": cache / "tools", "git_subjects": cache / "git-subjects",
         "subjects": cache / "subjects", "workspace": repo / "workspace"}
missing_roots = sorted(key for key, root in roots.items() if not root.is_dir())


def walk(root):
    if not root.is_dir():
        return
    def fail(error):
        raise error
    for base, dirs, files in os.walk(root, onerror=fail):
        dirs[:] = sorted(d for d in dirs if d != ".git")
        yield Path(base), dirs, sorted(files)


packages = {name: {} for name in ("aws-cdk", "aws-cdk-lib", "yaml")}
cdk_executables = 0
for base, _, files in walk(roots["tools"]):
    if "cdk" in files and os.access(base / "cdk", os.X_OK):
        cdk_executables += 1
    if base.name in packages and base.parent.name == "node_modules" and "package.json" in files:
        try:
            version = str(json.loads((base / "package.json").read_text()).get("version", "unknown"))
        except (OSError, ValueError):
            version = "unreadable"
        packages[base.name][version] = packages[base.name].get(version, 0) + 1

subject_roots = 0
cdk_json_files = 0
for key in ("git_subjects", "subjects"):
    if key in missing_roots:
        continue
    for subject in sorted(roots[key].iterdir()):
        if not subject.is_dir() or subject.is_symlink():
            continue
        count = sum(files.count("cdk.json") for _, _, files in walk(subject))
        cdk_json_files += count
        subject_roots += int(count > 0)
assemblies = sum(1 for key in ("workspace", "git_subjects", "subjects")
                 for base, dirs, _ in walk(roots[key])
                 if base.name == "cdk.out" and (base / "manifest.json").is_file())

print(json.dumps({
    "schema": "aws-cdk-readonly-inventory-v2",
    "root_roles": sorted(roots),
    "missing_roots": missing_roots,
    "counts": {"on_path": {name: int(shutil.which(name) is not None)
                            for name in ("node", "npm", "cdk")},
               "cdk_executable_candidates_in_tools": cdk_executables,
               "installed_package_versions_in_tools": packages,
               "cdk_json_files_in_subject_roots": cdk_json_files,
               "subject_roots_with_cdk_json": subject_roots,
               "cdk_assemblies": assemblies}
}, sort_keys=True, indent=2))
