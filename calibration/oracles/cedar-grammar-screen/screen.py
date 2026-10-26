"""Native policy/schema gate over every CLI-accepted file of the pinned subjects."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import sys

PINS = {
    "cedar-examples": ("c251f7d1ad171bd12dee5d1d7a1cceaec994518f", 20, 14),
    "cedar-integration-tests": ("1999ea249229e26cabb398a279fea721854a471d", 22, 3),
}
GRAMMARS = {"112e886c976951095b43ad91929269f9c1cca17d", "2e9f10824728b13d4c019774b32ca0a883e97f96"}


def digest(data):
    return hashlib.sha256(data).hexdigest()


def run(command, timeout=30):
    return subprocess.run(command, capture_output=True, timeout=timeout)


def population(name, root, cedar):
    pin, policies, schemas = PINS[name]
    if run(["git", "-C", str(root), "rev-parse", "HEAD"]).stdout.decode().strip() != pin:
        raise RuntimeError(name + ": wrong subject HEAD")
    status = run(["git", "-C", str(root), "status", "--porcelain", "--untracked-files=all", "--ignored"])
    if status.returncode or status.stdout:
        raise RuntimeError(name + ": subject must be clean, without untracked/ignored inputs")
    files = sorted(p for p in root.rglob("*") if p.suffix in (".cedar", ".cedarschema") and p.is_file())
    accepted, rejected = [], []
    for path in files:
        kind = "schema" if path.suffix == ".cedarschema" else "policies"
        result = run([cedar, "check-parse", "--" + kind, str(path)])
        if result.returncode not in (0, 1):
            raise RuntimeError(name + ": Cedar reference command failed")
        (accepted if result.returncode == 0 else rejected).append(path)
    actual = (sum(p.suffix == ".cedar" for p in accepted), sum(p.suffix == ".cedarschema" for p in accepted))
    if actual != (policies, schemas):
        raise RuntimeError(name + ": reference population differs from recorded pins")
    return accepted, len(rejected)


def main():
    if sys.version_info < (3, 12):
        raise RuntimeError("requires Python 3.12+ for safe archive extraction")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-examples", default="workspace/calibration/cedar-examples")
    parser.add_argument("-integration", default="workspace/calibration/cedar-integration-tests")
    parser.add_argument("-archive", default="workspace/tools/cedar-sworn.tar.gz")
    parser.add_argument("-commit", default="112e886c976951095b43ad91929269f9c1cca17d", choices=sorted(GRAMMARS))
    parser.add_argument("-cedar", required=True)
    parser.add_argument("-tree-sitter", required=True)
    parser.add_argument("-timeout", type=float, default=10)
    parser.add_argument("-out", default="workspace/artifacts/cedar-grammar-screen.json")
    args = parser.parse_args()
    cedar, cli = str(Path(args.cedar).resolve()), str(Path(args.tree_sitter).resolve())
    reference_version = run([cedar, "--version"]).stdout.decode().strip()
    cli_version = run([cli, "--version"]).stdout.decode().strip()
    if "4.13.0" not in reference_version:
        raise RuntimeError("requires cedar-policy-cli 4.13.0")
    if not cli_version.startswith("tree-sitter 0.27.0"):
        raise RuntimeError("requires tree-sitter CLI 0.27.0, as in the recorded native screen")
    workspace = Path("workspace").resolve()
    (workspace / "tmp").mkdir(parents=True, exist_ok=True)
    output = Path(args.out).resolve()
    if not output.is_relative_to(workspace):
        raise RuntimeError("output must be in this checkout's workspace")
    archive = Path(args.archive).resolve()
    populations = []
    for name, path in (("cedar-examples", args.examples), ("cedar-integration-tests", args.integration)):
        root = Path(path).resolve()
        accepted, rejected = population(name, root, cedar)
        populations.append((name, root, accepted, rejected))
    rows, grammar_hashes = [], {}
    with tempfile.TemporaryDirectory(prefix="cedar-grammar-", dir=workspace / "tmp") as tmp:
        scratch = Path(tmp)
        with tarfile.open(archive) as tar:
            if tar.pax_headers.get("comment") != args.commit:
                raise RuntimeError("grammar archive embedded commit differs from candidate pin")
            tar.extractall(scratch, filter="data")
        source = next(p for p in scratch.iterdir() if p.is_dir())
        libraries = {}
        for name in ("cedar", "cedarschema"):
            src = source / name / "src"
            files = [src / "parser.c"]
            if (src / "scanner.c").exists():
                files.append(src / "scanner.c")
            lib = scratch / (name + ".dylib")
            command = ["cc", "-O0", "-dynamiclib", "-I", str(src), *map(str, files), "-o", str(lib)]
            subprocess.run(command, check=True, capture_output=True)
            grammar_hashes[name] = {p.name: digest(p.read_bytes()) for p in files}
            libraries[name] = lib
        for name, root, files, rejected in populations:
            for suffix, grammar in ((".cedar", "cedar"), (".cedarschema", "cedarschema")):
                selected = [p for p in files if p.suffix == suffix]
                errors, faults, content = [], [], hashlib.sha256()
                for path in selected:
                    rel = path.relative_to(root).as_posix()
                    content.update(rel.encode() + b"\0" + hashlib.sha256(path.read_bytes()).digest() + b"\0")
                    command = [cli, "parse", "-q", "--lib-path", str(libraries[grammar]), "--lang-name", grammar, str(path)]
                    try:
                        result = run(command, args.timeout)
                        if result.returncode not in (0, 1):
                            faults.append(rel)
                        elif result.returncode:
                            # A CLI/library failure may also exit 1. Require a
                            # parse diagnostic before calling it a syntax miss.
                            detail = run([x for x in command if x != "-q"], args.timeout)
                            if b"ERROR" in detail.stdout or b"MISSING" in detail.stdout:
                                errors.append(rel)
                            else:
                                faults.append(rel)
                    except subprocess.TimeoutExpired:
                        faults.append(rel)
                rows.append(dict(subject=name, subject_commit=PINS[name][0], grammar=grammar,
                    files=len(selected), bytes=sum(p.stat().st_size for p in selected),
                    content_sha256=content.hexdigest(), error_or_missing_files=len(errors),
                    error_set_sha256=digest(b"".join(x.encode() + b"\0" for x in errors)),
                    faults=len(faults), fault_set_sha256=digest(b"".join(x.encode() + b"\0" for x in faults)),
                    reference_rejected_files=rejected))
    receipt = dict(command="python3 calibration/oracles/cedar-grammar-screen/screen.py (see README)",
        candidate="SwornSystems/tree-sitter-cedar", commit=args.commit,
        archive_sha256=digest(archive.read_bytes()), licence="MIT OR Apache-2.0",
        reference=reference_version, tree_sitter=cli_version, native_compile="cc -O0 -dynamiclib",
        parser_sources=grammar_hashes, timeout_seconds=args.timeout, subjects=rows,
        files=sum(x["files"] for x in rows), error_or_missing_files=sum(x["error_or_missing_files"] for x in rows),
        faults=sum(x["faults"] for x in rows))
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
