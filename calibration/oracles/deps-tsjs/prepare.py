#!/usr/bin/env python3
"""Materialize two clean subject paths per pin, optionally using read-only bare caches."""

import argparse
import json
import os
from pathlib import Path
import subprocess


def git(*args):
    return subprocess.check_output(["git", *map(str, args)]).strip().decode()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path)
    parser.add_argument("--cache", type=Path)
    args = parser.parse_args()
    dest = args.destination.resolve()
    workspace = (Path.cwd() / "workspace").resolve()
    if not dest.is_relative_to(workspace):
        parser.error("destination must be in checkout-local workspace")
    pins = json.loads(Path(__file__).with_name("pins.json").read_text())
    for subject in pins["subjects"]:
        name, url, commit = (subject[key] for key in ("name", "url", "commit"))
        for arm in ("primary", "relocated"):
            checkout = dest / arm / name
            checkout.parent.mkdir(parents=True, exist_ok=True)
            if not checkout.exists():
                if args.cache is None:
                    git("init", "--quiet", checkout)
                    git("-C", checkout, "remote", "add", "origin", url)
                    git("-C", checkout, "fetch", "--quiet", "--depth=1", "origin", commit)
                else:
                    bare = (args.cache / (name + ".git")).resolve(strict=True)
                    if git("--git-dir", bare, "rev-parse", commit + "^{commit}") != commit:
                        raise RuntimeError(f"missing cache pin for {name}")
                    git("clone", "--quiet", "--shared", "--no-checkout", bare, checkout)
                    git("-C", checkout, "remote", "set-url", "origin", url)
                git("-C", checkout, "checkout", "--quiet", "--detach", commit)
            if git("-C", checkout, "rev-parse", "HEAD") != commit:
                raise RuntimeError(f"unexpected {arm} commit for {name}")
            if git("-C", checkout, "remote", "get-url", "origin") != url:
                raise RuntimeError(f"unexpected {arm} URL for {name}")
            changed = git("-C", checkout, "status", "--porcelain", "--untracked-files=all", "--ignored=matching")
            if changed:
                raise RuntimeError(f"dirty {arm} checkout for {name}")
        print(name, commit)


if __name__ == "__main__":
    # Subjects are external inputs. Exclude ambient Git templates and hooks
    # during acquisition and avoid asking interactively for credentials.
    os.environ["GIT_CONFIG_COUNT"] = "2"
    os.environ["GIT_CONFIG_KEY_0"] = "core.hooksPath"
    os.environ["GIT_CONFIG_VALUE_0"] = "/dev/null"
    os.environ["GIT_CONFIG_KEY_1"] = "init.templateDir"
    os.environ["GIT_CONFIG_VALUE_1"] = "/dev/null"
    os.environ["GIT_TERMINAL_PROMPT"] = "0"
    main()
