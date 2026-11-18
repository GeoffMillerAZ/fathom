#!/usr/bin/env python3
"""Place the two pinned checkouts and record their tracked-set manifests."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess


PINS = json.loads(Path(__file__).with_name("pins.json").read_text())


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args])


def prepare(corpus, cache):
    for layout in ("first", "relocated"):
        for subject in PINS["subjects"]:
            root = corpus / layout / subject["name"]
            root.mkdir(parents=True, exist_ok=False)
            git(root, "init", "--quiet")
            git(root, "config", "core.autocrlf", "false")
            git(root, "config", "core.hooksPath", os.devnull)
            source = cache / (subject["name"] + ".git") if cache else subject["url"]
            git(root, "fetch", "--quiet", "--no-tags", "--depth=1", str(source), subject["commit"])
            git(root, "checkout", "--quiet", "--detach", "FETCH_HEAD")


def assert_git_bytes(data, oid):
    blob = b"blob " + str(len(data)).encode() + b"\0" + data
    if hashlib.sha1(blob).hexdigest().encode() != oid:
        raise SystemExit("tracked bytes differ from the pinned Git blob")


def snapshot(root, subject, receipt):
    top = os.fsdecode(git(root, "rev-parse", "--show-toplevel")).rstrip("\n")
    rev = git(root, "rev-parse", "HEAD").decode().strip()
    if Path(top).resolve() != root or rev != subject["commit"]:
        raise SystemExit("expected the pinned checkout root")
    lines, size = [], 0
    counts = dict(files=0, symlinks=0, gitlinks=0)
    records = git(root, "ls-files", "-s", "--cached", "-z", "--", ".")
    entries = []
    for record in records.split(b"\0"):
        if not record:
            continue
        meta, name = record.split(b"\t", 1)
        mode, oid, stage = meta.split()
        if stage != b"0":
            raise SystemExit("unmerged index")
        entries.append((name, mode, oid))
    if not entries:
        raise SystemExit("empty tracked set")
    for name, mode, oid in sorted(entries):
        member = root / os.fsdecode(name)
        if mode in (b"100644", b"100755"):
            if not stat.S_ISREG(member.lstat().st_mode):
                raise SystemExit("changed tracked file type")
            data = member.read_bytes()
            assert_git_bytes(data, oid)
            kind, identity = "file", hashlib.sha256(data).hexdigest()
            counts["files"] += 1
            size += len(data)
        elif mode == b"120000":
            data = os.readlink(os.fsencode(member))
            assert_git_bytes(data, oid)
            kind, identity = "symlink", hashlib.sha256(data).hexdigest()
            counts["symlinks"] += 1
        elif mode == b"160000":
            kind, identity = "gitlink", oid.decode("ascii")
            counts["gitlinks"] += 1
        else:
            raise SystemExit("unsupported Git index mode")
        lines.append(json.dumps([os.fsdecode(name), kind, identity], ensure_ascii=True,
                                separators=(",", ":")) + "\n")
    manifest = "".join(lines).encode()
    untracked = git(root, "ls-files", "-z", "--others", "--", ".")
    receipt.with_suffix(".manifest").write_bytes(manifest)
    receipt.with_suffix(".untracked").write_bytes(untracked)
    info = dict(sha256=hashlib.sha256(manifest).hexdigest(), entries=len(entries), bytes=size, **counts)
    receipt.with_suffix(".json").write_text(json.dumps(info, sort_keys=True) + "\n")
    if untracked:
        raise SystemExit("untracked or ignored files in corpus checkout")
    return info


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("corpus", type=Path)
    command = p.add_subparsers(dest="command", required=True)
    prep = command.add_parser("prepare")
    prep.add_argument("--cache", type=Path)
    manifest = command.add_parser("snapshot")
    manifest.add_argument("receipts", type=Path)
    args = p.parse_args()
    corpus = args.corpus.resolve()
    if args.command == "prepare":
        prepare(corpus, args.cache.resolve() if args.cache else None)
        return
    args.receipts.mkdir(parents=True, exist_ok=False)
    for subject in PINS["subjects"]:
        name = subject["name"]
        first = snapshot(corpus / "first" / name, subject, args.receipts / (name + "-first"))
        relocated = snapshot(corpus / "relocated" / name, subject, args.receipts / (name + "-relocated"))
        if first != relocated:
            raise SystemExit("checkout relocation changed tracked bytes")
        print(name, json.dumps(first, sort_keys=True))


if __name__ == "__main__":
    main()
