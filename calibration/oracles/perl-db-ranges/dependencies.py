#!/usr/bin/env python3
"""Fingerprint a private Perl installation and its saved distribution archives."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile


def regular_files(directory):
    """Yield only physical regular files; any symlink makes the fingerprint invalid."""
    if not directory.is_dir() or directory.is_symlink():
        raise ValueError("dependency directory must be a real directory")
    for current, dirs, files in os.walk(directory, followlinks=False):
        base = Path(current)
        for name in dirs + files:
            path = base / name
            if path.is_symlink():
                raise ValueError("dependency symlinks are not supported")
        dirs[:] = sorted(name for name in dirs if not (base / name).is_symlink())
        for name in sorted(files):
            path = base / name
            if not path.is_symlink() and stat.S_ISREG(path.stat().st_mode):
                yield path


def fingerprint(local_root, archive_root):
    local_root = Path(local_root)
    if (local_root / "lib").is_symlink() or (local_root / "lib" / "perl5").is_symlink():
        raise ValueError("lib/perl5 must be a real directory")
    local_root = local_root.resolve(strict=True)
    archive_root = Path(archive_root).resolve(strict=True)
    if not local_root.is_dir() or not archive_root.is_dir():
        raise ValueError("dependency roots must be directories")
    scope = local_root / "lib" / "perl5"
    if scope.is_symlink():
        raise ValueError("lib/perl5 must be a real directory")
    local_state = library_fingerprint(scope)
    archives = []
    seen_names = set()
    for path in regular_files(archive_root):
        if not path.name.endswith((".tar.gz", ".tgz", ".zip")):
            raise ValueError("saved distribution is not a supported archive")
        if path.name in seen_names:
            raise ValueError("duplicate saved distribution basename")
        seen_names.add(path.name)
        binary = hashlib.sha256()
        size = 0
        with path.open("rb") as source:
            while chunk := source.read(1024 * 1024):
                binary.update(chunk)
                size += len(chunk)
        archives.append({"basename": path.name, "sha256": binary.hexdigest(), "bytes": size})
    archives.sort(key=lambda row: row["basename"].encode())
    return {
        "local_lib_perl5": local_state,
        "saved_distributions": archives,
        "note": "Versionless local installation state; this fingerprint does not assert distribution versions or dependency resolution.",
    }


def library_fingerprint(perl_lib):
    """Fingerprint one explicitly selected lib/perl5 tree without following links."""
    scope_path = Path(perl_lib)
    if scope_path.is_symlink() or scope_path.parent.is_symlink():
        raise ValueError("local lib/perl5 path must not contain a symlink")
    scope = scope_path.resolve(strict=True)
    if not scope.is_dir() or scope.name != "perl5":
        raise ValueError("local Perl library must be an existing lib/perl5 directory")
    files = []
    for path in regular_files(scope):
        if path.name == ".packlist" or path.name == "perllocal.pod":
            continue
        files.append(path)
    files.sort(key=lambda path: path.relative_to(scope).as_posix().encode())
    digest = hashlib.sha256()
    total_bytes = 0
    for path in files:
        rel = path.relative_to(scope).as_posix().encode()
        size = path.stat().st_size
        total_bytes += size
        digest.update(len(rel).to_bytes(8, "big"))
        digest.update(rel)
        digest.update(size.to_bytes(8, "big"))
        with path.open("rb") as source:
            while chunk := source.read(1024 * 1024):
                digest.update(chunk)
    return {"files": len(files), "bytes": total_bytes, "sha256": digest.hexdigest()}


def self_test():
    with tempfile.TemporaryDirectory() as temporary:
        base = Path(temporary)
        local_a, local_b, archives = (base / name for name in ("a", "b", "dists"))
        archives.mkdir()
        (archives / "Example-1.0.tar.gz").write_bytes(b"archive")
        for local in (local_a, local_b):
            scope = local / "lib" / "perl5"
            scope.mkdir(parents=True)
            (scope / "Example.pm").write_bytes(b"package Example; 1;\n")
            (scope / ".packlist").write_bytes(b"host-specific")
            (scope / "perllocal.pod").write_bytes(b"host-specific")
        original = fingerprint(local_a, archives)
        assert original == fingerprint(local_b, archives)
        (local_b / "lib" / "perl5" / "perllocal.pod").write_bytes(b"different metadata")
        assert original == fingerprint(local_b, archives)
        (local_b / "lib" / "perl5" / "Example.pm").write_bytes(b"package Example; 2;\n")
        assert original["local_lib_perl5"]["sha256"] != fingerprint(local_b, archives)["local_lib_perl5"]["sha256"]
        assert str(base) not in json.dumps(original)
        (local_a / "lib" / "perl5" / "internal-link").symlink_to(local_a / "lib" / "perl5" / "Example.pm")
        try:
            fingerprint(local_a, archives)
        except ValueError as exc:
            assert "symlink" in str(exc)
        else:
            raise AssertionError("internal symlink accepted")
    print("dependency fingerprint controls passed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", help="private installation prefix containing lib/perl5")
    parser.add_argument("--archives", help="directory of saved distribution archives")
    parser.add_argument("--out", help="new JSON output file")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not all((args.root, args.archives, args.out)):
        parser.error("--root, --archives, and --out are required")
    output = Path(args.out)
    if output.exists():
        parser.error("output already exists")
    result = fingerprint(args.root, args.archives)
    with output.open("x", encoding="utf-8") as target:
        json.dump(result, target, indent=2, sort_keys=True)
        target.write("\n")


if __name__ == "__main__":
    main()
