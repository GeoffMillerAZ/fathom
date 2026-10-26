#!/usr/bin/env python3
"""Provenance wrapper for the native Dart Sass resolver adapter."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import posixpath
import re
import signal
import subprocess
import sys
import tempfile
import time
import urllib.parse

TIMEOUT_SECONDS = 600
EXPECTED_TIER = "A"
PINNED_DART = "3.13.7"
PINNED_SASS_API = "18.1.0"
PINNED_SASS = "1.106.1"


class Refusal(RuntimeError):
    pass


def run(argv: list[str], *, cwd: Path | None = None, timeout: int = TIMEOUT_SECONDS,
        capture: bool = True) -> subprocess.CompletedProcess[str]:
    kwargs: dict[str, object] = {"cwd": cwd, "text": True, "start_new_session": True}
    if capture:
        kwargs["stdout"] = subprocess.PIPE
        kwargs["stderr"] = subprocess.PIPE
    proc = subprocess.Popen(argv, **kwargs)  # type: ignore[arg-type]
    try:
        out, err = proc.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        def signal_group(sig: int, retry_seconds: float) -> bool:
            deadline = time.monotonic() + retry_seconds
            while True:
                try:
                    os.killpg(proc.pid, sig)
                    return True
                except ProcessLookupError:
                    return True
                except PermissionError:
                    if time.monotonic() >= deadline:
                        return False
                    time.sleep(0.25)

        signal_group(signal.SIGTERM, 5)
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            try:
                os.killpg(proc.pid, 0)
            except ProcessLookupError:
                break
            except PermissionError:  # Darwin may transiently deny signaling during file-access checks.
                pass
            time.sleep(0.1)
        # Always target the group, even if its leader exited after SIGTERM.
        if not signal_group(signal.SIGKILL, 5):
            raise Refusal(f"timed out and could not kill process group for {argv[0]}")
        try:
            out, err = proc.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            out, err = "", ""
        raise Refusal(f"timed out after {timeout}s: {argv[0]}")
    if isinstance(out, str) and len(out.encode()) > 256 * 1024 * 1024:
        raise Refusal("command stdout exceeded 256 MiB limit")
    if isinstance(err, str) and len(err.encode()) > 16 * 1024 * 1024:
        raise Refusal("command stderr exceeded 16 MiB limit")
    return subprocess.CompletedProcess(argv, proc.returncode, out or "", err or "")


def checked(argv: list[str], *, cwd: Path | None = None) -> str:
    result = run(argv, cwd=cwd)
    if result.returncode:
        raise Refusal(f"command failed ({result.returncode}): {result.stderr.strip()}")
    return result.stdout.strip()


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def git(root: Path, *args: str) -> str:
    return checked(["git", "-C", str(root), *args])


def tracked_manifest(root: Path, scope: str | None) -> dict[str, str]:
    tracked = git(root, "ls-files", "-z").split("\0")
    paths = sorted(p for p in tracked if p and p.lower().endswith(".scss"))
    if scope:
        prefix = scope.rstrip("/") + "/"
        paths = [p for p in paths if p == scope or p.startswith(prefix)]
    if not paths:
        raise Refusal("scope contains no tracked .scss files")
    files: dict[str, str] = {}
    for rel in paths:
        path = root / rel
        if path.is_symlink():
            raise Refusal(f"tracked .scss symlink is forbidden: {rel}")
        if not path.is_file():
            raise Refusal(f"tracked .scss is not a regular file: {rel}")
        files[rel] = sha256(path)
    return files


def filesystem_manifest(root: Path, scope: str | None) -> set[str]:
    base = root / scope if scope else root
    found: set[str] = set()
    for current, dirs, names in os.walk(base, followlinks=False):
        dirs[:] = [name for name in dirs if not (Path(current) / name).is_symlink()]
        for name in names:
            path = Path(current) / name
            if path.suffix.lower() == ".scss":
                found.add(path.relative_to(root).as_posix())
    return found


def assert_inventory(root: Path, scope: str | None, expected: dict[str, str]) -> None:
    actual = filesystem_manifest(root, scope)
    if actual != set(expected):
        extra, missing = sorted(actual - set(expected)), sorted(set(expected) - actual)
        raise Refusal(f"SCSS inventory differs from tracked scope; extra={extra[:4]} missing={missing[:4]}")
    current = {p: sha256(root / p) for p in expected}
    if current != expected:
        raise Refusal("tracked SCSS content changed during collection")


def require_clean_subject(root: Path, expected_sha: str) -> None:
    if len(expected_sha) != 40 or any(c not in "0123456789abcdef" for c in expected_sha.lower()):
        raise Refusal("--expected-sha must be a full 40-character Git SHA")
    actual = git(root, "rev-parse", "HEAD")
    if actual.lower() != expected_sha.lower():
        raise Refusal(f"subject HEAD mismatch: expected {expected_sha}, got {actual}")
    if git(root, "status", "--porcelain", "--untracked-files=all"):
        raise Refusal("subject Git root is dirty")


def primary_hash(primary: Path) -> str:
    if not primary.is_file():
        raise Refusal("primary JSON is missing")
    return sha256(primary)


def adapter_hash_map(metadata: dict, key: str) -> dict[str, str]:
    try:
        result = metadata["metadata"][key]
    except (KeyError, TypeError) as exc:
        raise Refusal(f"adapter metadata missing {key}") from exc
    if not isinstance(result, dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in result.items()):
        raise Refusal(f"adapter {key} is not a path-to-hash map")
    return result


def package_pins(config_path: Path) -> tuple[dict[str, str], dict[str, str]]:
    try:
        config = json.loads(config_path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        raise Refusal(f"invalid Dart package config: {exc}") from exc
    packages = {item.get("name"): item for item in config.get("packages", []) if isinstance(item, dict)}
    expected = {"sass_api": PINNED_SASS_API, "sass": PINNED_SASS}
    versions: dict[str, str] = {}
    digests: dict[str, str] = {}
    for name, version in expected.items():
        item = packages.get(name)
        if not item or not isinstance(item.get("rootUri"), str):
            raise Refusal(f"Dart package config omits {name}")
        uri = urllib.parse.urljoin(config_path.as_uri(), item["rootUri"])
        parsed = urllib.parse.urlparse(uri)
        if parsed.scheme != "file":
            raise Refusal(f"{name} package root is not a local file URI")
        package_root = Path(urllib.parse.unquote(parsed.path)).resolve(strict=True)
        manifest = package_root / "pubspec.yaml"
        if not manifest.is_file():
            raise Refusal(f"{name} package has no pubspec.yaml")
        contents = manifest.read_text()
        found_name = re.search(r"(?m)^name:\s*['\"]?([^'\"\s]+)", contents)
        found_version = re.search(r"(?m)^version:\s*['\"]?([^'\"\s]+)", contents)
        actual_name = found_name.group(1) if found_name else ""
        actual_version = found_version.group(1) if found_version else ""
        if actual_name != name or actual_version != version:
            raise Refusal(f"package pin mismatch for {name}: {actual_name} {actual_version}")
        versions[name] = actual_version
        digests[name] = sha256(manifest)
    return versions, digests


def normalize_module(value: str) -> str:
    parts = urllib.parse.urlsplit(value)
    path = posixpath.normpath(parts.path) if parts.path else ""
    if parts.path.startswith("/") and not path.startswith("/"):
        path = "/" + path
    if parts.path.endswith("/") and not path.endswith("/"):
        path += "/"
    return urllib.parse.urlunsplit((parts.scheme, parts.netloc, path, parts.query, parts.fragment))


def validate_primary_enrichment(primary: dict, output: object, native_dependencies: object) -> dict:
    if not isinstance(output, dict) or set(output) != set(primary):
        raise Refusal("adapter output changed the primary top-level schema")
    if {k: v for k, v in output.items() if k != "imports"} != {k: v for k, v in primary.items() if k != "imports"}:
        raise Refusal("adapter altered primary fields outside imports")
    original_imports = primary["imports"]
    result_imports = output.get("imports")
    if not isinstance(result_imports, list) or len(result_imports) != len(original_imports):
        raise Refusal("adapter changed primary import count")
    if not isinstance(native_dependencies, list) or any(not isinstance(item, dict) for item in native_dependencies):
        raise Refusal("native dependency attestation is missing or invalid")
    for original, enriched in zip(original_imports, result_imports):
        if not isinstance(original, dict) or not isinstance(enriched, dict):
            raise Refusal("primary and adapter imports must be objects")
        if set(enriched) != set(original) | {"to_path"}:
            raise Refusal("adapter changed primary import schema")
        if not isinstance(enriched.get("to_path"), str):
            raise Refusal("adapter to_path must be a string")
        if any(enriched.get(key) != value for key, value in original.items() if key != "to_path"):
            raise Refusal("adapter altered a primary import field")
        if enriched["to_path"]:
            attested = any(
                item.get("resolution") == "resolved"
                and item.get("path") == original.get("path")
                and normalize_module(str(item.get("module", ""))) == normalize_module(str(original.get("module", "")))
                and item.get("to_path") == enriched["to_path"]
                for item in native_dependencies
            )
            if not attested:
                raise Refusal("adapter to_path lacks matching native resolver attestation")
    return output


def selftest() -> None:
    """Cheap synthetic controls; never launches Dart or reads a corpus."""
    with tempfile.TemporaryDirectory(prefix="sass-provenance-selftest-") as tmp:
        root = Path(tmp) / "subject"
        root.mkdir()
        checked(["git", "init", "-q", str(root)])
        (root / "src").mkdir()
        (root / "src/a.scss").write_text("a { color: red }\n")
        checked(["git", "-C", str(root), "add", "src/a.scss"])
        checked(["git", "-C", str(root), "-c", "user.name=Selftest", "-c", "user.email=selftest@example.invalid", "commit", "-qm", "fixture"])
        head = git(root, "rev-parse", "HEAD")
        require_clean_subject(root, head)
        manifest = tracked_manifest(root, "src")
        assert_inventory(root, "src", manifest)
        assert_inventory(root, None, manifest)
        (root / "src/a.scss").write_text("dirty\n")
        try:
            require_clean_subject(root, head)
            raise AssertionError("dirty subject accepted")
        except Refusal:
            pass
        checked(["git", "-C", str(root), "checkout", "--", "src/a.scss"])
        link = root / "src/untracked.scss"
        link.symlink_to("a.scss")
        try:
            assert_inventory(root, "src", manifest)
            raise AssertionError("untracked SCSS symlink accepted")
        except Refusal:
            pass
        link.unlink()
        tracked_link = root / "src/tracked-link.scss"
        tracked_link.symlink_to("a.scss")
        checked(["git", "-C", str(root), "add", "src/tracked-link.scss"])
        try:
            tracked_manifest(root, "src")
            raise AssertionError("tracked SCSS symlink accepted")
        except Refusal:
            pass
        tracked_link.unlink()
        checked(["git", "-C", str(root), "reset", "-q", "--", "src/tracked-link.scss"])
        primary = root / "primary.json"
        primary.write_text('{"imports":[]}\n')
        frozen = primary_hash(primary)
        primary.write_text('{"imports":[1]}\n')
        if primary_hash(primary) == frozen:
            raise AssertionError("primary hash failed to detect change")
        base = {"files": 4, "imports": [{"path": "a.scss", "line": 1, "module": "b", "to_path": ""}]}
        native_fixture = [{"path": "a.scss", "module": "b", "to_path": "b.scss", "resolution": "resolved"}]
        validate_primary_enrichment(base, {"files": 4, "imports": [{**base["imports"][0], "to_path": "b.scss"}]}, native_fixture)
        invalid = [
            {"files": 5, "imports": [{**base["imports"][0], "to_path": "b.scss"}]},
            {"files": 4, "invented": True, "imports": [{**base["imports"][0], "to_path": "b.scss"}]},
            {"files": 4, "imports": [{**base["imports"][0], "to_path": "b.scss", "metadata": "invented"}]},
            {"files": 4, "imports": [{**base["imports"][0], "path": "bogus.scss", "to_path": "b.scss"}]},
            {"files": 4, "imports": [{**base["imports"][0], "to_path": "bogus.scss"}]},
        ]
        for candidate in invalid:
            try:
                validate_primary_enrichment(base, candidate, native_fixture)
                raise AssertionError("invalid primary enrichment accepted")
            except Refusal:
                pass
    print("selftest: scope/inventory, dirty subject, symlinks, primary hash, preservation/schema controls: PASS")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--selftest", action="store_true")
    parser.add_argument("--subject", type=Path)
    parser.add_argument("--expected-sha")
    parser.add_argument("--primary", type=Path)
    parser.add_argument("--dart", type=Path)
    parser.add_argument("--package-config", type=Path)
    parser.add_argument("--dir", dest="scope")
    parser.add_argument("--load-path", action="append", default=[])
    parser.add_argument("--output", type=Path, required=False)
    parser.add_argument("--metadata", type=Path, required=False)
    args = parser.parse_args()
    try:
        if args.selftest:
            selftest()
            return 0
        if not all((args.subject, args.expected_sha, args.primary, args.dart, args.package_config, args.output, args.metadata)):
            parser.error("collection requires --subject --expected-sha --primary --dart --package-config --output --metadata")
        root = args.subject.resolve(strict=True)
        top = Path(git(root, "rev-parse", "--show-toplevel")).resolve(strict=True)
        if top != root:
            raise Refusal("--subject must be the Git root, not a subdirectory")
        if args.scope:
            scope_path = Path(args.scope)
            if scope_path.is_absolute() or ".." in scope_path.parts or not (root / scope_path).is_dir():
                raise Refusal("--dir must be an existing relative directory inside subject")
            resolved_scope = (root / scope_path).resolve(strict=True)
            if resolved_scope != root and root not in resolved_scope.parents:
                raise Refusal("--dir must resolve inside subject")
            scope = None if resolved_scope == root else resolved_scope.relative_to(root).as_posix()
        else:
            scope = None
        require_clean_subject(root, args.expected_sha)
        root_before = tracked_manifest(root, None)
        assert_inventory(root, None, root_before)
        before = tracked_manifest(root, scope)
        assert_inventory(root, scope, before)
        primary_path = args.primary.resolve(strict=True)
        frozen_primary_sha = primary_hash(primary_path)
        frozen_primary = json.loads(primary_path.read_text())
        if not isinstance(frozen_primary, dict) or not isinstance(frozen_primary.get("imports"), list):
            raise Refusal("primary JSON must be an object with an imports array")
        dart = args.dart.resolve(strict=True)
        package_config = args.package_config.resolve(strict=True)
        if not package_config.is_file():
            raise Refusal("--package-config must name an existing file")
        installed_pins, package_manifest_sha = package_pins(package_config)
        adapter = Path(__file__).with_name("bin") / "resolver.dart"
        output_path = args.output.resolve()
        metadata_path = args.metadata.resolve()
        for label, path in (("primary", primary_path), ("output", output_path), ("metadata", metadata_path)):
            if path == root or root in path.parents:
                raise Refusal(f"{label} must be outside the subject Git root")
        metadata_path.parent.mkdir(parents=True, exist_ok=True)
        metadata_path.touch(mode=0o600, exist_ok=True)
        metadata_path.chmod(0o600)
        version_result = run([str(dart), "--version"])
        dart_version_text = version_result.stderr + version_result.stdout
        if version_result.returncode or PINNED_DART not in dart_version_text:
            raise Refusal(f"Dart must be pinned at {PINNED_DART}; got {dart_version_text.strip()}")
        command = [str(dart), f"--packages={package_config}", str(adapter), str(root)]
        if scope:
            command += ["--dir", scope]
        for path in args.load_path:
            resolved = Path(path).resolve(strict=True)
            if not resolved.is_dir() or (resolved != root and root not in resolved.parents):
                raise Refusal("each --load-path must be a directory inside the pinned subject")
            command += ["--load-path", str(resolved)]
        command += ["--primary", str(primary_path), "--metadata", str(args.metadata.resolve())]
        load = os.getloadavg()[0]
        if load > 9:
            raise Refusal(f"load average {load:.2f} exceeds launch limit 9")
        proc = run(["nice", "-n", "10", *command], cwd=adapter.parent.parent)
        if proc.returncode:
            raise Refusal(f"Dart adapter failed ({proc.returncode}): {proc.stderr[-3000:]}")
        if primary_hash(primary_path) != frozen_primary_sha:
            raise Refusal("primary JSON changed during adapter run")
        require_clean_subject(root, args.expected_sha)
        assert_inventory(root, None, root_before)
        assert_inventory(root, scope, before)
        try:
            native = json.loads(args.metadata.read_text())
            output = json.loads(proc.stdout)
        except (OSError, json.JSONDecodeError) as exc:
            raise Refusal(f"adapter outputs are not readable JSON: {exc}") from exc
        expected = before
        for label in ("source_fingerprints_before", "source_fingerprints_after"):
            if adapter_hash_map(native, label) != expected:
                raise Refusal(f"adapter {label} does not equal Git-tracked scoped source hashes")
        output = validate_primary_enrichment(frozen_primary, output, native.get("native_dependencies"))
        inventory_hash = hashlib.sha256("\n".join(f"{p}\0{h}" for p, h in sorted(before.items())).encode()).hexdigest()
        load_path_roots = [Path(path).resolve(strict=True).relative_to(root).as_posix() or "." for path in args.load_path]
        load_path_files = {
            p: h for p, h in root_before.items()
            if any(scope_root == "." or p == scope_root or p.startswith(scope_root.rstrip("/") + "/") for scope_root in load_path_roots)
        }
        native_metadata = native.get("metadata", {})
        count_keys = (
            "source_files", "declared_dependencies", "resolved_dependencies",
            "unresolved_dependencies", "skipped", "parse_failures",
            "unmapped_native_dependencies", "ambiguous_native_dependencies",
        )
        native_counts = {key: native_metadata[key] for key in count_keys if key in native_metadata}
        for key in ("unmapped_native_dependencies", "ambiguous_native_dependencies"):
            if key in native:
                native_counts[key] = native[key]
        for key in (
            "unmapped_native_by_kind", "unpositioned_native_count",
            "unpositioned_native_by_kind",
            "unpositioned_native_resolved_count", "unpositioned_native_unresolved_count",
        ):
            if key in native:
                native_counts[key] = native[key]
        native_counts["failure_records"] = len(native_metadata.get("failures", []))
        native_counts["unresolved_native_dependencies"] = len(native.get("unresolved_native_dependencies", []))
        report = {
            "tier": EXPECTED_TIER,
            "subject_sha": args.expected_sha.lower(),
            "scope": scope or ".",
            "source_pin": inventory_hash,
            "source_count": len(before),
            "load_path_source_pin": hashlib.sha256("\n".join(f"{p}\0{h}" for p, h in sorted(load_path_files.items())).encode()).hexdigest(),
            "load_path_source_count": len(load_path_files),
            "primary_sha256": frozen_primary_sha,
            "adapter_sha256": sha256(adapter),
            "dart_binary_sha256": sha256(dart),
            "dart_version": dart_version_text.strip(),
            "sass_api": PINNED_SASS_API,
            "sass": PINNED_SASS,
            "installed_packages": installed_pins,
            "package_manifest_sha256": package_manifest_sha,
            "package_config_sha256": sha256(package_config),
            "load_paths": load_path_roots,
            "native_counts": native_counts,
            "primary_counts": {
                "imports": len(output.get("imports", [])),
                "resolved_to_path": sum(bool(item.get("to_path")) for item in output.get("imports", [])),
            },
            "metadata_sha256": sha256(metadata_path),
            "result_sha256": hashlib.sha256(proc.stdout.encode()).hexdigest(),
        }
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(proc.stdout)
        output_path.with_suffix(output_path.suffix + ".provenance.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
        print(json.dumps(report, indent=2, sort_keys=True))
        return 0
    except (Refusal, OSError, subprocess.SubprocessError) as exc:
        print(f"collect: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
