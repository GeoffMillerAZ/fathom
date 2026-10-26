#!/usr/bin/env python3
"""Build and run the fixed HLSL scope control without regenerating oracles."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


SOURCE_COMMIT = "6db920e72898abb20bf86c22652b401902e34598"
CONTROL_FILE = Path("cmd/indexbench/langoracle.go")
SUBJECTS = ("directx-graphics-samples", "kajiya", "directxtk12")
RUNS = ("hlsl-run1.json", "hlsl-run2.json", "hlsl-run3.json")
SCOPED_RUN = "hlsl-scoped.json"

_GUARD = b"\tif len(tf.Calls) > 0 || len(tf.Scope) > 0 {"
_CONTROL_GUARD = b"\tif len(tf.Calls) > 0 {"
_EXPANSION = (
    b"\t\tif len(tf.Scope) > 0 {\n"
    b"\t\t\t// Explicit scope covers files with zero oracle calls. Legacy\n"
    b"\t\t\t// nonempty calls without scope cover only their named files.\n"
    b"\t\t\tfor f := range inScope {\n"
    b"\t\t\t\tfiles[f] = true\n"
    b"\t\t\t}\n"
    b"\t\t}\n"
)


def _digest(data):
    return hashlib.sha256(data).hexdigest()


def expected_control_source(base):
    """Return the sole permitted source transformation, refusing drift."""
    if base.count(_GUARD) != 1 or base.count(_EXPANSION) != 1:
        raise RuntimeError("langoracle.go does not contain the expected scope source bytes")
    changed = base.replace(_GUARD, _CONTROL_GUARD, 1).replace(_EXPANSION, b"", 1)
    if changed == base:
        raise RuntimeError("control patch made no source change")
    return changed


def input_paths(source_root):
    oracle = Path(source_root).resolve() / "workspace/tmp/oracle"
    result = []
    for subject in SUBJECTS:
        result.extend((
            oracle / f"hlsl-{subject}-dxc.json",
            oracle / f"hlsl-{subject}-imports.json",
            oracle / f"hlsl-{subject}.scope",
        ))
    return result


def snapshot_inputs(source_root):
    entries = []
    for path in input_paths(source_root):
        try:
            data = path.read_bytes()
        except OSError as error:
            raise RuntimeError(f"missing oracle input: {path}") from error
        entries.append({"path": str(path), "bytes": len(data), "sha256": _digest(data)})
    return {"inputs": entries}


def verify_inputs(snapshot):
    entries = snapshot.get("inputs")
    if not isinstance(entries, list) or len(entries) != 9:
        raise RuntimeError("oracle snapshot must contain exactly nine inputs")
    total = 0
    for entry in entries:
        path = Path(entry["path"])
        try:
            data = path.read_bytes()
        except OSError as error:
            raise RuntimeError(f"oracle input changed or disappeared: {path}") from error
        if len(data) != entry.get("bytes") or _digest(data) != entry.get("sha256"):
            raise RuntimeError(f"oracle input changed: {path}")
        total += len(data)
    return {"input_files": len(entries), "input_bytes": total, "unchanged": True}


def _scope_argument(path):
    rows = [row for row in path.read_text().splitlines() if row]
    if not rows:
        raise RuntimeError(f"empty scope input: {path}")
    return ",".join(rows)


def grade_commands(source_root, binary, control_workspace):
    source_root = Path(source_root).resolve()
    binary = Path(binary).resolve()
    control_workspace = Path(control_workspace).resolve()
    oracle = source_root / "workspace/tmp/oracle"
    clones = source_root / "workspace/calibration/hlsl"
    plans = [(name, False) for name in RUNS] + [(SCOPED_RUN, True)]
    commands = []
    for output_name, scoped in plans:
        output = control_workspace / output_name
        for subject in SUBJECTS:
            command = [
                str(binary), "lang-oracle", "-dialect", "hlsl",
                "-subject", str((clones / subject).resolve()),
                "-tool-oracle", str((oracle / f"hlsl-{subject}-dxc.json").resolve()),
                "-tool-oracle", str((oracle / f"hlsl-{subject}-imports.json").resolve()),
                "-unmatched", str((control_workspace / f"{output.stem}-{subject}-unmatched.json").resolve()),
                "-commit", SOURCE_COMMIT[:12] + "-dirty-control",
            ]
            if scoped:
                command.extend(("-only", _scope_argument(oracle / f"hlsl-{subject}.scope")))
            command.extend(("-out", str(output)))
            commands.append(command)
    return commands


def _default_runner(command, **kwargs):
    subprocess.run(command, check=True, **kwargs)


def run_grades(source_root, control_root, reading_out, runner=_default_runner):
    source_root = Path(source_root).resolve()
    control_root = Path(control_root).resolve()
    reading_out = Path(reading_out).resolve()
    binary = control_root / "workspace/tools/indexbench"
    control_workspace = control_root / "workspace/tmp/oracle"
    control_workspace.mkdir(parents=True, exist_ok=True)
    reading_out.parent.mkdir(parents=True, exist_ok=True)
    for command in grade_commands(source_root, binary, control_workspace):
        runner(command, cwd=control_root)
    driver = control_root / "calibration/oracles/dxc-ast/reading.sh"
    env = os.environ.copy()
    env.update({
        "NOTES_ONLY": "1",
        "HLSL_CLONES": str((source_root / "workspace/calibration/hlsl").resolve()),
    })
    runner(["bash", str(driver), str(reading_out)], cwd=control_root, env=env)


def _git_env():
    env = os.environ.copy()
    env.update({
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_TERMINAL_PROMPT": "0",
    })
    return env


def _git_output(*command, cwd=None):
    return subprocess.run(command, cwd=cwd, check=True, stdout=subprocess.PIPE,
                          env=_git_env()).stdout


def _verify_control_paths(control_root, expected_tracked):
    staged = subprocess.run(["git", "diff", "--cached", "--quiet"],
                            cwd=control_root, env=_git_env()).returncode
    if staged != 0:
        raise RuntimeError("control checkout has staged changes")
    changed = _git_output("git", "diff", "--name-only", cwd=control_root).decode().splitlines()
    if changed != expected_tracked:
        raise RuntimeError(f"control checkout changed unexpected tracked files: {changed}")
    extras = set()
    for args in (("--others", "--exclude-standard"),
                 ("--others", "--ignored", "--exclude-standard")):
        rows = _git_output("git", "ls-files", *args, cwd=control_root).decode().splitlines()
        extras.update(row for row in rows if row and row != "workspace" and
                      not row.startswith("workspace/"))
    if extras:
        raise RuntimeError(f"control checkout has content outside workspace: {sorted(extras)}")


def verify_prepared_control(control_root, patch_path):
    control_root = Path(control_root).resolve()
    patch_path = Path(patch_path).resolve()
    head = _git_output("git", "rev-parse", "HEAD", cwd=control_root).decode().strip()
    if head != SOURCE_COMMIT:
        raise RuntimeError(f"control checkout is {head}, want {SOURCE_COMMIT}")
    base = _git_output("git", "show", f"{SOURCE_COMMIT}:{CONTROL_FILE.as_posix()}",
                       cwd=control_root)
    if (control_root / CONTROL_FILE).read_bytes() != expected_control_source(base):
        raise RuntimeError("control langoracle.go differs from the committed control patch")
    _verify_control_paths(control_root, [CONTROL_FILE.as_posix()])
    subprocess.run(["git", "diff", "--check"], cwd=control_root, check=True,
                   env=_git_env())
    subprocess.run(["git", "apply", "--reverse", "--check", "--unidiff-zero",
                    str(patch_path)], cwd=control_root, check=True, env=_git_env())
    driver = Path("calibration/oracles/dxc-ast/reading.sh")
    committed_driver = _git_output("git", "show", f"{SOURCE_COMMIT}:{driver.as_posix()}",
                                   cwd=control_root)
    if (control_root / driver).read_bytes() != committed_driver:
        raise RuntimeError("control reading.sh differs from the committed main driver")


def prepare_control(source_root, patch_path):
    source_root = Path(source_root).resolve()
    patch_path = Path(patch_path).resolve()
    source_head = _git_output("git", "rev-parse", "HEAD", cwd=source_root).decode().strip()
    if source_head != SOURCE_COMMIT:
        raise RuntimeError(f"source checkout is {source_head}, want {SOURCE_COMMIT}")
    control_root = source_root / "workspace/tmp/hlsl-scope-replay/control-source"
    if control_root.exists():
        raise RuntimeError(f"control checkout path already exists: {control_root}")
    subprocess.run(["git", "-c", "core.hooksPath=/dev/null", "clone", "--quiet",
                    "--shared", "--no-checkout", str(source_root), str(control_root)],
                   check=True, env=_git_env())
    subprocess.run(["git", "-c", "core.hooksPath=/dev/null", "checkout", "--quiet",
                    "--detach", SOURCE_COMMIT], cwd=control_root, check=True,
                   env=_git_env())
    base = _git_output("git", "show", f"{SOURCE_COMMIT}:{CONTROL_FILE.as_posix()}",
                       cwd=control_root)
    expected = expected_control_source(base)
    target = control_root / CONTROL_FILE
    actual = target.read_bytes()
    if actual != base:
        raise RuntimeError("fresh control checkout differs from the pinned source")
    _verify_control_paths(control_root, [])
    subprocess.run(["git", "apply", "--check", "--unidiff-zero", str(patch_path)],
                   cwd=control_root, check=True, env=_git_env())
    subprocess.run(["git", "apply", "--unidiff-zero", str(patch_path)],
                   cwd=control_root, check=True, env=_git_env())
    actual = target.read_bytes()
    if actual != expected:
        raise RuntimeError("control langoracle.go differs from the committed control patch")
    verify_prepared_control(control_root, patch_path)
    binary = control_root / "workspace/tools/indexbench"
    binary.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(["go", "build", "-o", str(binary), "./cmd/indexbench"],
                   cwd=control_root, check=True)
    verify_prepared_control(control_root, patch_path)
    return control_root


def _write_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    snap = sub.add_parser("snapshot")
    snap.add_argument("--source", required=True)
    snap.add_argument("--out", required=True)
    run = sub.add_parser("run")
    run.add_argument("--source", required=True)
    run.add_argument("--patch", required=True)
    run.add_argument("--snapshot", required=True)
    run.add_argument("--reading-out", required=True)
    run.add_argument("--receipt-out", required=True)
    args = parser.parse_args()
    try:
        if args.command == "snapshot":
            _write_json(args.out, snapshot_inputs(args.source))
        else:
            snapshot = json.loads(Path(args.snapshot).read_text())
            verify_inputs(snapshot)
            control_root = prepare_control(args.source, args.patch)
            run_grades(args.source, control_root, args.reading_out)
            verify_prepared_control(control_root, args.patch)
            receipt = verify_inputs(snapshot)
            _write_json(args.receipt_out, receipt)
    except (OSError, RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
