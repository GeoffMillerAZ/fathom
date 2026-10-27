#!/usr/bin/env python3
"""Bounded owner-tool candidates; raw evidence stays in ignored scratch."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import select
import selectors
import signal
import subprocess
import sys
import time

import probe


MAX_OUTPUT = 2 * 1024 * 1024
MAX_LSP_OUTPUT = 8 * 1024 * 1024
MAX_PACKAGE = 64 * 1024 * 1024
MAX_LAUNCH_LOAD = 9.0
LEGACY = {
    "upbound/platform-ref-aws-network-resources": "package/cluster/network",
    "upbound/platform-ref-azure-network-resources": "package/cluster/network",
    "upbound/platform-ref-gcp-network-resources": "cluster/network",
}


def private_env(scratch):
    scratch = scratch.resolve()
    return {
        "PATH": "/usr/bin:/bin", "HOME": str(scratch), "TMPDIR": str(scratch),
        "KUBECONFIG": "/dev/null", "DOCKER_CONFIG": str(scratch / "docker-empty"),
        "DOCKER_HOST": "tcp://127.0.0.1:9", "CONTAINER_HOST": "tcp://127.0.0.1:9",
        "HTTP_PROXY": "http://127.0.0.1:9", "HTTPS_PROXY": "http://127.0.0.1:9",
        "ALL_PROXY": "http://127.0.0.1:9", "NO_PROXY": "localhost,127.0.0.1",
        "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
        "CLOUDSDK_CONFIG": str(scratch / "cloud-empty"),
        "AWS_SHARED_CREDENTIALS_FILE": "/dev/null", "AWS_CONFIG_FILE": "/dev/null",
        "AZURE_CONFIG_DIR": str(scratch / "azure-empty"),
        "GOOGLE_APPLICATION_CREDENTIALS": "/dev/null",
    }


def sha(data):
    return hashlib.sha256(data).hexdigest()


def launch_load():
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise probe.Refusal("native launch requires nice 10")
    while True:
        load = os.getloadavg()[0]
        if load <= MAX_LAUNCH_LOAD:
            return round(load, 2)
        time.sleep(20)


def run_tool(argv, label, scratch, timeout=20, cap=MAX_OUTPUT, watch_file=None, cwd=None):
    """Capture exact stdout/stderr privately; a failure is a result, never a pass."""
    raw = scratch / "raw"
    raw.mkdir(parents=True, exist_ok=True)
    safe = re.sub(r"[^A-Za-z0-9_.-]", "_", label)
    out_path, err_path = raw / (safe + ".out"), raw / (safe + ".err")
    status = "exit"
    load = launch_load()
    started = time.monotonic()
    with out_path.open("wb") as stdout, err_path.open("wb") as stderr:
        try:
            proc = subprocess.Popen([str(a) for a in argv], cwd=cwd, env=private_env(scratch),
                                    stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                    start_new_session=True)
        except OSError as exc:
            stderr.write((type(exc).__name__ + ": " + str(exc)).encode())
            stderr.flush()
            return {"status": "spawn-error", "exit": None,
                    "stdout_sha256": sha(b""), "stderr_sha256": probe.digest(err_path),
                    "stdout_bytes": 0, "stderr_bytes": err_path.stat().st_size,
                    "launch_load1": load}
        while proc.poll() is None:
            if os.fstat(stdout.fileno()).st_size > cap or os.fstat(stderr.fileno()).st_size > cap:
                status = "output-cap"
                break
            if watch_file is not None and watch_file.is_file() and watch_file.stat().st_size > MAX_PACKAGE:
                status = "package-cap"
                break
            if time.monotonic() - started > timeout:
                status = "timeout"
                break
            time.sleep(0.02)
        if status != "exit":
            os.killpg(proc.pid, signal.SIGKILL)
        proc.wait()
    sizes = (out_path.stat().st_size, err_path.stat().st_size)
    if max(sizes) > cap:
        status = "output-cap"
        # Limit persistent private output too.
        for path in (out_path, err_path):
            if path.stat().st_size > cap:
                with path.open("r+b") as f:
                    f.truncate(cap)
        sizes = (out_path.stat().st_size, err_path.stat().st_size)
    return {"status": status, "exit": proc.returncode, "seconds": round(time.monotonic() - started, 3),
            "stdout_sha256": probe.digest(out_path), "stderr_sha256": probe.digest(err_path),
            "stdout_bytes": sizes[0], "stderr_bytes": sizes[1], "launch_load1": load}


def check_subjects(manifest, scratch):
    subjects = manifest.get("subjects")
    if not isinstance(subjects, list) or len(subjects) != 9:
        raise probe.Refusal("expected nine pinned subjects")
    frozen = json.loads((Path(__file__).with_name("pins-r1.json")).read_text())["subjects"]
    expected = {p.get("id", p["repo"] + "-legacy" if p.get("prior_probe") else p["repo"]):
                (p["sha"], p.get("scope", ["*.yaml", "*.yml"])) for p in frozen}
    actual = {entry.get("id"): (entry.get("sha"), entry.get("scope")) for entry in subjects}
    if len(actual) != len(subjects) or actual != expected:
        raise probe.Refusal("subject identity or scope differs from frozen pins")
    result = []
    for entry in subjects:
        root = Path(entry["root"]).resolve(strict=True)
        probe.git_identity(root, entry["sha"], scratch)
        files = probe.safe_files(root, entry["scope"])
        if not files:
            raise probe.Refusal("empty subject scope")
        if sum(probe.checked_size(file) for file in files) > probe.MAX_TOTAL:
            raise probe.Refusal("subject byte cap exceeded")
        result.append((entry, root, files))
    return result


def xpkg_build(crossplane, subjects, scratch):
    results = []
    empty = scratch / "empty-examples"
    empty.mkdir(exist_ok=True)
    for entry, root, _ in subjects:
        for package_root in (root, root / "package"):
            if not (package_root / "crossplane.yaml").is_file():
                continue
            label = re.sub(r"[^A-Za-z0-9_.-]", "_", entry["id"])
            # The package root is the complete pinned source root, not a derived copy.
            output = scratch / "xpkg" / (label + ".xpkg")
            output.parent.mkdir(exist_ok=True)
            result = run_tool([crossplane, "xpkg", "build", "--package-root", package_root,
                               "--examples-root", empty, "--package-file", output],
                              "xpkg-" + label, scratch, timeout=90, watch_file=output)
            result.update(subject=entry["id"], package_root=str(package_root.relative_to(root)),
                          package_sha256=probe.digest(output) if output.is_file() else None)
            results.append(result)
            break
    return results


def xr_for_legacy(definition, output):
    """Read the fixed XRD's group/kind/version; no general YAML interpretation."""
    raw = definition.read_text()
    group = re.search(r"(?m)^  group: ([A-Za-z0-9.-]+)$", raw)
    kind = re.search(r"(?m)^    kind: ([A-Za-z0-9]+)$", raw)
    version = re.search(r"(?m)^    - name: ([A-Za-z0-9]+)$", raw)
    if not all((group, kind, version)):
        raise probe.Refusal("legacy XRD shape changed")
    output.write_text(f"apiVersion: {group.group(1)}/{version.group(1)}\n"
                      f"kind: {kind.group(1)}\nmetadata:\n  name: offline-probe\n"
                      "spec:\n  id: offline-probe\n")


def legacy_render(crossplane, subjects, scratch, controller=None):
    result = []
    for entry, root, _ in subjects:
        rel = LEGACY.get(entry["id"])
        if rel is None:
            continue
        base = root / rel
        xr = scratch / (entry["id"].split("/")[-1] + "-xr.yaml")
        xr_for_legacy(base / "definition.yaml", xr)
        argv = [crossplane, "composition", "render", xr, base / "composition.yaml"]
        if controller:
            argv += ["--crossplane-binary", controller]
        item = run_tool(argv, "legacy-" + entry["id"].split("/")[-1], scratch, timeout=20)
        item.update(subject=entry["id"], controller_supplied=bool(controller),
                    input_kind="exact-legacy-composition")
        result.append(item)
    return result


def modern_up(up, subjects, scratch):
    """Try exact existing inputs; dry-run discovers tests without provisioning."""
    results = {"help": {name: run_tool([up, *args], "up038-" + name, scratch)
                         for name, args in {"version": ["version", "--client"],
                                            "render": ["composition", "render", "--help"],
                                            "test": ["test", "run", "--help"]}.items()},
               "actions": []}
    for entry, root, _ in subjects:
        rel = LEGACY.get(entry["id"])
        if not rel:
            continue
        base = root / rel
        xr = scratch / (entry["id"].split("/")[-1] + "-up038-xr.yaml")
        xr_for_legacy(base / "definition.yaml", xr)
        row = run_tool([up, "composition", "render", base / "composition.yaml", xr,
                        "--xrd", base / "definition.yaml", "--timeout=10s"],
                       "up038-legacy-" + entry["id"].split("/")[-1], scratch, cwd=root)
        row.update(subject=entry["id"], kind="exact-legacy-up-render")
        results["actions"].append(row)
    for entry, root, _ in subjects:
        if entry["id"] != "upbound/configuration-aws-network":
            continue
        composition = root / "apis/networks/composition.yaml"
        xr = root / "examples/networks/configuration-aws-network.yaml"
        xrd = root / "apis/networks/definition.yaml"
        if not all(path.is_file() for path in (composition, xr, xrd)):
            raise probe.Refusal("pinned modern render fixture is incomplete")
        row = run_tool([up, "composition", "render", composition, xr, "--xrd", xrd,
                        "--timeout=10s"], "up038-modern-render", scratch, cwd=root)
        row.update(subject=entry["id"], kind="pinned-modern-up-render")
        results["actions"].append(row)
        row = run_tool([up, "test", "run", str(root / "tests/*"), "--dry-run",
                        "--max-concurrency=1"], "up038-test-dry-run", scratch, cwd=root)
        row.update(subject=entry["id"], kind="test-dry-run")
        results["actions"].append(row)
    return results


def encode_lsp(message):
    payload = json.dumps(message, separators=(",", ":")).encode()
    return f"Content-Length: {len(payload)}\r\n\r\n".encode() + payload


def parse_lsp(buffer):
    messages = []
    while b"\r\n\r\n" in buffer:
        header, rest = buffer.split(b"\r\n\r\n", 1)
        match = re.search(rb"(?im)^Content-Length: ([0-9]+)\r?$", header)
        if not match:
            raise probe.Refusal("bad LSP frame")
        size = int(match.group(1))
        if size > MAX_LSP_OUTPUT:
            raise probe.Refusal("oversized LSP frame")
        if len(rest) < size:
            break
        try:
            messages.append(json.loads(rest[:size]))
        except ValueError as exc:
            raise probe.Refusal("bad LSP JSON") from exc
        buffer = rest[size:]
    return messages, buffer


def lsp_subject(up, entry, root, files, scratch):
    """One stdio server per source tree; didOpen each scoped file exactly once."""
    raw = scratch / "raw"
    raw.mkdir(exist_ok=True)
    label = re.sub(r"[^A-Za-z0-9_.-]", "_", entry["id"])
    out_path, err_path = raw / ("xpls-" + label + ".out"), raw / ("xpls-" + label + ".err")
    diagnostics = []
    opened_files = 0
    responses = {}
    total = 0
    buffer = b""
    status = "exit"
    load = launch_load()
    deadline = time.monotonic() + 45
    with out_path.open("wb") as stdout, err_path.open("wb") as stderr:
        try:
            proc = subprocess.Popen([up, "xpls", "serve"], cwd=root, env=private_env(scratch),
                                    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=stderr,
                                    start_new_session=True, bufsize=0)
        except OSError as exc:
            stderr.write(str(exc).encode())
            stderr.flush()
            return {"subject": entry["id"], "status": "spawn-error", "launch_load1": load}
        selector = selectors.DefaultSelector()
        selector.register(proc.stdout, selectors.EVENT_READ)

        os.set_blocking(proc.stdin.fileno(), False)

        def send(msg):
            frame = encode_lsp(msg)
            written = 0
            while written < len(frame):
                if time.monotonic() >= deadline:
                    raise probe.Refusal("LSP send timeout")
                _, writable, _ = select.select([], [proc.stdin], [], 0.1)
                if writable:
                    written += os.write(proc.stdin.fileno(), frame[written:])
                else:
                    pump(min(deadline, time.monotonic() + 0.05))

        def pump(until):
            nonlocal buffer, total, status
            while time.monotonic() < until:
                if os.fstat(stderr.fileno()).st_size > MAX_LSP_OUTPUT:
                    status = "output-cap"
                    return
                for _, _ in selector.select(timeout=min(0.1, max(0, until - time.monotonic()))):
                    chunk = os.read(proc.stdout.fileno(), 65536)
                    if not chunk:
                        return
                    total += len(chunk)
                    if total > MAX_LSP_OUTPUT:
                        status = "output-cap"
                        return
                    stdout.write(chunk)
                    buffer += chunk
                    parsed, buffer = parse_lsp(buffer)
                    for msg in parsed:
                        if msg.get("method") == "textDocument/publishDiagnostics":
                            params = msg.get("params", {})
                            entries = params.get("diagnostics")
                            if entries is None:
                                entries = []  # The pinned owner server emits null for an empty set.
                            if not isinstance(entries, list):
                                raise probe.Refusal("malformed LSP diagnostics")
                            diagnostics.append({"uri": params.get("uri"), "count": len(entries)})
                        elif "id" in msg and "method" not in msg:
                            responses[msg["id"]] = msg
                        elif "id" in msg and msg.get("method") == "workspace/configuration":
                            send({"jsonrpc": "2.0", "id": msg["id"], "result": []})

        try:
            send({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
                "processId": None, "rootUri": root.as_uri(), "capabilities": {},
                "workspaceFolders": [{"uri": root.as_uri(), "name": "subject"}]}})
            pump(min(deadline, time.monotonic() + 5))
            if 1 not in responses:
                status = "initialize-no-response"
            elif "error" in responses[1]:
                status = "initialize-error"
            else:
                send({"jsonrpc": "2.0", "method": "initialized", "params": {}})
                for file in files:
                    content = file.read_bytes()
                    if len(content) > probe.MAX_FILE:
                        raise probe.Refusal("LSP input file cap")
                    send({"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": {
                        "textDocument": {"uri": file.as_uri(), "languageId": "yaml", "version": 1,
                                         "text": content.decode("utf-8")}}})
                    opened_files += 1
                    pump(min(deadline, time.monotonic() + 0.05))
                    if status == "output-cap" or time.monotonic() >= deadline:
                        break
                if status != "output-cap":
                    pump(min(deadline, time.monotonic() + 3))
                if proc.poll() is None:
                    send({"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": None})
                    pump(min(deadline, time.monotonic() + 1))
                    send({"jsonrpc": "2.0", "method": "exit", "params": {}})
        except (BrokenPipeError, OSError, UnicodeError, probe.Refusal) as exc:
            status = type(exc).__name__
        finally:
            selector.close()
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
            proc.wait()
            proc.stdin.close()
            proc.stdout.close()
    if time.monotonic() >= deadline:
        status = "timeout"
    for path in (out_path, err_path):
        if path.stat().st_size > MAX_LSP_OUTPUT:
            status = "output-cap"
            with path.open("r+b") as stream:
                stream.truncate(MAX_LSP_OUTPUT)
    return {"subject": entry["id"], "status": status, "exit": proc.returncode,
            "selected_files": len(files), "opened_files": opened_files, "diagnostic_notifications": len(diagnostics),
            "diagnostic_entries": sum(d["count"] for d in diagnostics),
            "files_with_diagnostics": len({d["uri"] for d in diagnostics if d["count"]}),
            "initialized": 1 in responses and "result" in responses[1],
            "shutdown_responded": 2 in responses, "stdout_sha256": probe.digest(out_path),
            "stderr_sha256": probe.digest(err_path), "stdout_bytes": out_path.stat().st_size,
            "stderr_bytes": err_path.stat().st_size, "launch_load1": load}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--crossplane", type=Path, required=True)
    parser.add_argument("--up", type=Path)
    parser.add_argument("--up-modern", type=Path)
    parser.add_argument("--crossplane-controller", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--scratch", type=Path, required=True)
    args = parser.parse_args()
    args.out = args.out.resolve()
    args.scratch = args.scratch.resolve()
    args.out.unlink(missing_ok=True)
    current_nice = os.getpriority(os.PRIO_PROCESS, 0)
    if current_nice < 10:
        os.nice(10 - current_nice)
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise probe.Refusal("nice 10 required for native probe")
    args.crossplane = args.crossplane.resolve()
    if args.up:
        args.up = args.up.resolve()
    if args.up_modern:
        args.up_modern = args.up_modern.resolve()
    if args.crossplane_controller:
        args.crossplane_controller = args.crossplane_controller.resolve()
    args.scratch.mkdir(parents=True, exist_ok=True)
    if args.scratch.resolve() == args.out.parent.resolve():
        parser.error("receipt and raw scratch must differ")
    manifest = json.loads(args.manifest.read_text())
    expected = manifest.get("tool_sha256", {}).get("crossplane")
    if not expected or probe.digest(args.crossplane) != expected:
        raise probe.Refusal("Crossplane CLI digest differs from manifest")
    if args.up:
        expected_up = manifest.get("tool_sha256", {}).get("up")
        if not expected_up or probe.digest(args.up) != expected_up:
            raise probe.Refusal("Upbound CLI digest differs from manifest")
    if args.up_modern:
        expected_modern = manifest.get("tool_sha256", {}).get("up_modern")
        if not expected_modern or probe.digest(args.up_modern) != expected_modern:
            raise probe.Refusal("modern Upbound CLI digest differs from manifest")
    subjects = check_subjects(manifest, args.scratch)
    receipt = {"schema": "crossplane-owner-probes-v1", "manifest_sha256": probe.digest(args.manifest),
               "tools": {"crossplane_sha256": expected},
               "subject_files": {entry["id"]: len(files) for entry, _, files in subjects}}
    if args.crossplane_controller:
        receipt["tools"]["crossplane_controller_sha256"] = probe.digest(args.crossplane_controller)
    receipt["help"] = {"crossplane-render": run_tool([args.crossplane, "composition", "render", "--help"],
                                                     "crossplane-render-help", args.scratch),
                       "crossplane-xpkg": run_tool([args.crossplane, "xpkg", "build", "--help"],
                                                   "crossplane-xpkg-help", args.scratch)}
    receipt["xpkg"] = xpkg_build(args.crossplane, subjects, args.scratch)
    receipt["legacy_render"] = legacy_render(args.crossplane, subjects, args.scratch,
                                               args.crossplane_controller)
    if args.up:
        receipt["tools"]["up_sha256"] = expected_up
        receipt["help"].update({name: run_tool([args.up, *cmd], name, args.scratch) for name, cmd in {
            "up-version": ["version", "--client"], "xpls": ["xpls", "serve", "--help"],
            "up-render": ["composition", "render", "--help"],
            "up-test": ["test", "run", "--help"]}.items()})
        receipt["xpls"] = [lsp_subject(str(args.up), entry, root, files, args.scratch)
                            for entry, root, files in subjects]
    if args.up_modern:
        receipt["tools"]["up_modern_sha256"] = probe.digest(args.up_modern)
        receipt["up_modern"] = modern_up(args.up_modern, subjects, args.scratch)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    temp = args.out.with_suffix(args.out.suffix + ".tmp")
    temp.write_text(json.dumps(receipt, sort_keys=True, indent=2) + "\n")
    temp.replace(args.out)


if __name__ == "__main__":
    main()
