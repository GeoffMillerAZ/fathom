#!/usr/bin/env python3
"""Read-only scoped Terragrunt inventory; aggregate counts only."""
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


binary_candidates = 0
for base, _, files in walk(roots["tools"]):
    for name in files:
        path = base / name
        if name.startswith("terragrunt") and path.is_file() and os.access(path, os.X_OK):
            binary_candidates += 1

subject_roots = 0
unit_files = 0
stack_files = 0
for key in ("git_subjects", "subjects"):
    if key in missing_roots:
        continue
    for subject in sorted(roots[key].iterdir()):
        if not subject.is_dir() or subject.is_symlink():
            continue
        has_content = False
        for _, _, files in walk(subject):
            units = files.count("terragrunt.hcl")
            stacks = files.count("terragrunt.stack.hcl")
            unit_files += units
            stack_files += stacks
            has_content |= units + stacks > 0
        subject_roots += int(has_content)
stack_dirs = sum(1 for key in ("workspace", "git_subjects", "subjects")
                 for _, dirs, _ in walk(roots[key]) if ".terragrunt-stack" in dirs)

print(json.dumps({
    "schema": "terragrunt-readonly-inventory-v2",
    "root_roles": sorted(roots),
    "missing_roots": missing_roots,
    "counts": {"terragrunt_on_path": int(shutil.which("terragrunt") is not None),
               "versioned_executable_candidates_in_tools": binary_candidates,
               "terragrunt_hcl_files_in_subject_roots": unit_files,
               "terragrunt_stack_hcl_files_in_subject_roots": stack_files,
               "subject_roots_with_terragrunt_content": subject_roots,
               "generated_stack_directories": stack_dirs}
}, sort_keys=True, indent=2))
