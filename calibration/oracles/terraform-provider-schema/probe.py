#!/usr/bin/env python3
"""Offline, authored-control probe for native OpenTofu provider schemas.

The operator supplies local, pinned artifacts. All native output remains in the
private scratch directory; the public receipt contains only counts and hashes.
"""

import argparse
import hashlib
import json
import os
import re
import selectors
import shutil
import signal
import socket
import subprocess
import sys
import time
from pathlib import Path


NAMES = ("aws", "azurerm", "google")
PLATFORM = "darwin_arm64"
PROFILE = '(version 1)(allow default)(deny network-outbound (remote ip "*:*"))'
SENTINEL = "schema_probe_nonexistent_7d4cf63f"
MAX_EXPORT = 128 * 1024 * 1024
MAX_OTHER = 16 * 1024 * 1024
TIMEOUT = 180
SHA = re.compile(r"^[0-9a-f]{64}$")
VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
LAUNCHES = []

FIXTURES = {
    "aws": {
        "provider": 'region = "us-east-1"',
        "resource": 'resource "aws_vpc" "probe" {\n cidr_block = "10.0.0.0/16"\n tags = { probe = "x" }\n}',
        "alias": 'resource "aws_vpc" "alias_probe" {\n provider = aws.secondary\n cidr_block = "10.0.0.0/16"\n tags = { probe = "x" }\n}',
        "data": 'data "aws_region" "current" {}',
        "expressions": ("aws_vpc.probe.id", "aws_vpc.alias_probe.id", "data.aws_region.current.name", 'aws_vpc.probe.tags["probe"]'),
        "mutation": ("id", "id", "name", "tags"),
    },
    "azurerm": {
        "provider": "features {}",
        "resource": 'resource "azurerm_resource_group" "probe" {\n name = "schema-probe-fixture"\n location = "westus"\n tags = { probe = "x" }\n}',
        "alias": 'resource "azurerm_resource_group" "alias_probe" {\n provider = azurerm.secondary\n name = "schema-probe-fixture-alias"\n location = "westus"\n tags = { probe = "x" }\n}',
        "data": 'data "azurerm_client_config" "current" {}',
        "expressions": ("azurerm_resource_group.probe.id", "azurerm_resource_group.alias_probe.id", "data.azurerm_client_config.current.tenant_id", 'azurerm_resource_group.probe.tags["probe"]'),
        "mutation": ("id", "id", "tenant_id", "tags"),
    },
    "google": {
        "provider": 'project = "schema-probe"',
        "resource": 'resource "google_storage_bucket" "probe" {\n name = "schema-probe-fixture"\n location = "US"\n labels = { probe = "x" }\n}',
        "alias": 'resource "google_storage_bucket" "alias_probe" {\n provider = google.secondary\n name = "schema-probe-fixture-alias"\n location = "US"\n labels = { probe = "x" }\n}',
        "data": 'data "google_client_config" "current" {}',
        "expressions": ("google_storage_bucket.probe.id", "google_storage_bucket.alias_probe.id", "data.google_client_config.current.project", 'google_storage_bucket.probe.labels["probe"]'),
        "mutation": ("id", "id", "project", "labels"),
    },
}


class ProbeError(Exception):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


def digest(path):
    h = hashlib.sha256()
    with open(path, "rb") as stream:
        for part in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(part)
    return h.hexdigest()


def json_bytes(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def check_manifest(manifest, pins, baseline_pins):
    if manifest.get("platform") != PLATFORM:
        raise ProbeError("platform")
    tool = manifest.get("tool")
    providers = manifest.get("providers")
    if not isinstance(tool, dict) or not isinstance(providers, list) or len(providers) != 3:
        raise ProbeError("manifest_shape")
    if sorted(p.get("name") for p in providers if isinstance(p, dict)) != list(NAMES):
        raise ProbeError("provider_set")
    if tool.get("sha256") != pins["tool"]["sha256"]:
        raise ProbeError("tool_pin")
    if pins["tool"]["version"] != baseline_pins["opentofu"]["version"]:
        raise ProbeError("tool_version_pin")
    for item in (tool,) + tuple(providers):
        keys = ("sha256",) if item is tool else ("archive_sha256", "binary_sha256")
        if any(not isinstance(item.get(key), str) or not SHA.fullmatch(item[key]) for key in keys):
            raise ProbeError("hash_shape")
        path_key = "path" if item is tool else "archive"
        path = item.get(path_key)
        if not isinstance(path, str) or not Path(path).is_file():
            raise ProbeError("missing_artifact")
        if digest(path) != item[keys[0]]:
            raise ProbeError("artifact_hash")
    for item in providers:
        reference = pins["providers"][item["name"]]
        if any(item.get(key) != reference.get(key) for key in ("version", "archive_sha256", "binary_sha256")):
            raise ProbeError("provider_pin")
    aws = next(p for p in providers if p["name"] == "aws")
    if aws["version"] != baseline_pins["providers"]["aws"]["version"] or aws["archive_sha256"] != baseline_pins["providers"]["aws"]["sha256"]:
        raise ProbeError("aws_base_pin")
    if any(not isinstance(p.get("version"), str) or not VERSION.fullmatch(p["version"]) for p in providers):
        raise ProbeError("version_shape")
    return {p["name"]: p for p in providers}


def private_env(root, config):
    home = root / "home"
    tmp = root / "tmp"
    home.mkdir()
    tmp.mkdir()
    return {
        "HOME": str(home), "TMPDIR": str(tmp), "PATH": "/usr/bin:/bin",
        "TF_CLI_CONFIG_FILE": str(config), "TF_IN_AUTOMATION": "1",
        "TF_INPUT": "0", "CHECKPOINT_DISABLE": "1", "GOMAXPROCS": "2",
        "AWS_EC2_METADATA_DISABLED": "true", "KUBECONFIG": "/dev/null",
        "HTTP_PROXY": "http://127.0.0.1:9", "HTTPS_PROXY": "http://127.0.0.1:9",
        "ALL_PROXY": "socks5://127.0.0.1:9", "http_proxy": "http://127.0.0.1:9",
        "https_proxy": "http://127.0.0.1:9", "all_proxy": "socks5://127.0.0.1:9",
        "NO_PROXY": "", "no_proxy": "",
    }


def wait_for_load():
    load = os.getloadavg()[0]
    while load > 9:
        time.sleep(20)
        load = os.getloadavg()[0]
    return load


def launch(argv, cwd, env, out, err, cap, timeout=TIMEOUT, _sandbox_prefix=None):
    load = wait_for_load()
    priority = os.getpriority(os.PRIO_PROCESS, 0)
    if priority < 10:
        os.setpriority(os.PRIO_PROCESS, 0, 10)
    priority = os.getpriority(os.PRIO_PROCESS, 0)
    if priority < 10:
        raise ProbeError("nice_unproven")
    admission = {"command": out.name.removesuffix(".stdout"), "load_at_launch": load,
                 "nice_at_launch": priority, "status": "admitted"}
    LAUNCHES.append(admission)
    if _sandbox_prefix is None:
        _sandbox_prefix = ["/usr/bin/sandbox-exec", "-p", PROFILE]
    try:
        proc = subprocess.Popen(list(_sandbox_prefix) + argv,
                                cwd=str(cwd), env=env, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, start_new_session=True)
    except OSError:
        admission["status"] = "spawn_failed"
        raise
    selector = selectors.DefaultSelector()
    selector.register(proc.stdout, selectors.EVENT_READ, "stdout")
    selector.register(proc.stderr, selectors.EVENT_READ, "stderr")
    paths = {"stdout": out, "stderr": err}
    size = 0
    deadline = time.monotonic() + timeout
    failure = None
    with open(paths["stdout"], "wb") as stdout, open(paths["stderr"], "wb") as stderr:
        streams = {"stdout": stdout, "stderr": stderr}
        try:
            while selector.get_map():
                remain = deadline - time.monotonic()
                if remain <= 0:
                    failure = "timeout"
                    break
                for key, _ in selector.select(min(remain, 0.5)):
                    chunk = os.read(key.fileobj.fileno(), 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                        continue
                    size += len(chunk)
                    if size > cap:
                        failure = "output_cap"
                        break
                    streams[key.data].write(chunk)
                if failure:
                    break
        finally:
            selector.close()
            if not failure:
                try:
                    proc.wait(timeout=max(0, deadline - time.monotonic()))
                except subprocess.TimeoutExpired:
                    failure = "timeout"
            if failure:
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            proc.wait()
            proc.stdout.close()
            proc.stderr.close()
    admission.update({"stdout_sha256": digest(out), "stderr_sha256": digest(err),
                      "stdout_bytes": out.stat().st_size, "stderr_bytes": err.stat().st_size,
                      "exit_code": proc.returncode, "status": failure or "exited"})
    if failure:
        raise ProbeError(failure)
    return proc.returncode


def prove_network_deny(root, env):
    if not Path("/usr/bin/sandbox-exec").is_file():
        raise ProbeError("sandbox_unavailable")
    with socket.socket() as server:
        server.bind(("127.0.0.1", 0))
        server.listen(1)
        port = server.getsockname()[1]
        with socket.create_connection(("127.0.0.1", port), timeout=2):
            pass
        code = ('import socket; s=socket.socket(); s.settimeout(2); '
                's.connect(("127.0.0.1", %d))' % port)
        rc = launch([sys.executable, "-c", code], root, env,
                    root / "deny.stdout", root / "deny.stderr", 1024 * 1024, 10)
        if rc == 0:
            raise ProbeError("sandbox_network_allowed")
        stderr = (root / "deny.stderr").read_text(errors="replace").lower()
        if "operation not permitted" not in stderr and "permission denied" not in stderr:
            raise ProbeError("sandbox_deny_unproven")


def fixture(name, version):
    f = FIXTURES[name]
    lines = [
        'terraform {\n required_providers {\n %s = {\n source = "hashicorp/%s"\n version = "= %s"\n }\n }\n}' % (name, name, version),
        'provider "%s" {\n %s\n}' % (name, f["provider"]),
        'provider "%s" {\n alias = "secondary"\n %s\n}' % (name, f["provider"]),
        f["resource"], f["alias"], f["data"],
    ]
    lines += ['output "o%d" { value = %s }' % (i, expr) for i, expr in enumerate(f["expressions"])]
    return "\n".join(lines) + "\n"


def mutate(name, index, source):
    expr = FIXTURES[name]["expressions"][index]
    field = FIXTURES[name]["mutation"][index]
    changed = expr.replace("." + field, "." + SENTINEL, 1)
    old = 'output "o%d" { value = %s }' % (index, expr)
    new = 'output "o%d" { value = %s }' % (index, changed)
    if source.count(old) != 1 or changed == expr:
        raise ProbeError("mutation_source")
    return source.replace(old, new)


def depth(obj):
    if isinstance(obj, dict):
        return 1 + max((depth(v) for v in obj.values()), default=0)
    if isinstance(obj, list):
        return 1 + max((depth(v) for v in obj), default=0)
    return 0


def map_count(obj):
    if isinstance(obj, dict):
        return int(obj.get("nesting_mode") == "map") + sum(map_count(v) for v in obj.values())
    if isinstance(obj, list):
        return int(len(obj) > 0 and obj[0] == "map") + sum(map_count(v) for v in obj)
    return 0


def attribute_count(obj):
    if not isinstance(obj, dict):
        return 0
    block = obj.get("block", obj)
    return len(block.get("attributes", {})) + sum(attribute_count(v) for v in block.get("block_types", {}).values())


def summarize_schema(raw):
    data = json.loads(raw)
    schemas = data.get("provider_schemas", {})
    namespaces = {}
    for namespace, schema in schemas.items():
        sections = {key: len(value) for key, value in schema.items()
                    if isinstance(value, dict) and key != "provider"}
        resource = schema.get("resource_schemas", {})
        sources = schema.get("data_source_schemas", {})
        blocks = [schema.get("provider", {}).get("block", {})]
        blocks += [v.get("block", {}) for v in resource.values()]
        blocks += [v.get("block", {}) for v in sources.values()]
        namespaces[namespace] = {
            "provider_schema_count": int("provider" in schema),
            "section_counts": sections,
            "unscored_sections": sorted(key for key in sections
                                         if key not in ("resource_schemas", "data_source_schemas")),
            "explicit_attribute_declarations_in_examined_blocks": sum(attribute_count(b) for b in blocks),
            "map_declarations_in_examined_blocks": sum(map_count(b) for b in blocks),
        }
    return {"namespace_count": len(namespaces), "namespaces": namespaces,
            "compact_json_bytes": len(json_bytes(data)),
            "raw_export_line_count": raw.count(b"\n") + int(bool(raw) and not raw.endswith(b"\n")),
            "usable_anchor_line_count": "unmeasured", "referenced_closure_serialized_bytes": "unmeasured",
            "max_json_depth": depth(data)}


def summarize_show(raw, name):
    data = json.loads(raw)
    if (not isinstance(data, dict) or not isinstance(data.get("root_module"), dict) or
            not isinstance(data.get("provider_config"), dict)):
        raise ProbeError("show_shape")
    root = data["root_module"]
    outputs = root.get("outputs")
    if not isinstance(outputs, dict) or not isinstance(root.get("resources"), list):
        raise ProbeError("show_shape")
    refs = {key: value.get("expression", {}).get("references", []) for key, value in outputs.items()}
    expected = FIXTURES[name]["expressions"]
    observed = [ref for values in refs.values() for ref in values]
    resources = root.get("resources", [])
    mapping = data["provider_config"]
    return {"authored_outputs": len(outputs),
            "native_reference_count": sum(len(v) for v in refs.values()),
            "authored_full_path_count": sum(path in refs.get("o%d" % i, []) for i, path in enumerate(expected)),
            "native_extra_reference_count": len(observed) - sum(path in refs.get("o%d" % i, []) for i, path in enumerate(expected)),
            "provider_config_count": len(mapping),
            "aliased_provider_config_count": sum(bool(v.get("alias")) for v in mapping.values()),
            "resource_provider_mapping_count": sum(bool(v.get("provider_config_key")) for v in resources),
            "aliased_resource_mapping_count": sum("secondary" in str(v.get("provider_config_key", "")) for v in resources),
            "compact_json_bytes": len(json_bytes(data)),
            "raw_lines": raw.count(b"\n") + int(bool(raw) and not raw.endswith(b"\n"))}


def run_command(tool, args, cwd, env, scratch, label, cap):
    out = scratch / (label + ".stdout")
    err = scratch / (label + ".stderr")
    rc = launch([tool] + args, cwd, env, out, err, cap)
    if rc != 0:
        raise ProbeError("native_" + label)
    return out.read_bytes()


def validate(tool, cwd, env, scratch, label):
    out = scratch / (label + ".stdout")
    err = scratch / (label + ".stderr")
    rc = launch([tool, "validate", "-json"], cwd, env, out, err, MAX_OTHER)
    try:
        data = json.loads(out.read_bytes())
    except (ValueError, OSError):
        raise ProbeError("validate_json")
    return summarize_validation(data, rc, digest(out))


def canonical_validation_sha256(raw_or_data):
    """Hash validate JSON independently of diagnostic emission order.

    Accepts historical capture bytes as well as parsed JSON. The full diagnostic
    object is the final tie-break, so a changed detail cannot hide behind the
    same severity, range, and summary.
    """
    data = json.loads(raw_or_data) if isinstance(raw_or_data, (bytes, str)) else raw_or_data
    if not isinstance(data, dict) or not isinstance(data.get("diagnostics", []), list):
        raise ProbeError("validate_json")
    diagnostics = data.get("diagnostics", [])
    if any(not isinstance(diag, dict) for diag in diagnostics):
        raise ProbeError("validate_json")
    normalized = dict(data)
    normalized["diagnostics"] = sorted(diagnostics, key=lambda diag: (
        str(diag.get("severity", "")), json_bytes(diag.get("range")),
        str(diag.get("summary", "")), json_bytes(diag)))
    return hashlib.sha256(json_bytes(normalized)).hexdigest()


def summarize_validation(data, rc, raw_sha256):
    diagnostics = data.get("diagnostics", [])
    def attributed(diag):
        if diag.get("severity") != "error":
            return False
        message = (str(diag.get("summary", "")) + " " + str(diag.get("detail", ""))).lower()
        snippet = str((diag.get("snippet") or {}).get("code", ""))
        span = diag.get("range") or {}
        return ("unsupported attribute" in message or "does not have an attribute named" in message) and \
            SENTINEL in (message + snippet) and Path(str(span.get("filename", ""))).name == "main.tf" and \
            isinstance(span.get("start", {}).get("line"), int) and span["start"]["line"] > 0
    attributed_errors = sum(attributed(d) for d in diagnostics)
    error_diagnostics = sum(d.get("severity") == "error" for d in diagnostics)
    return {"exit_zero": rc == 0, "valid": data.get("valid"),
            "error_count": data.get("error_count"), "warning_count": data.get("warning_count"),
            "diagnostic_count": len(diagnostics),
            "attributed_error_count": attributed_errors,
            "unattributed_error_count": error_diagnostics - attributed_errors,
            "diagnostic_severities": {v: sum(d.get("severity") == v for d in diagnostics)
                                      for v in ("error", "warning")},
            "raw_sha256": raw_sha256,
            "canonical_sha256": canonical_validation_sha256(data)}


def mutation_is_attributed(result):
    return (not result["exit_zero"] and result["valid"] is False and
            isinstance(result["error_count"], int) and result["error_count"] >= 1 and
            result["attributed_error_count"] >= 1 and result["unattributed_error_count"] == 0 and
            result["error_count"] == result["attributed_error_count"])


def provider_run(name, item, tool, root, env, frozen_source_hash):
    cwd = root / name
    cwd.mkdir()
    source = fixture(name, item["version"])
    tf = cwd / "main.tf"
    tf.write_text(source)
    source_hash = digest(tf)
    if source_hash != frozen_source_hash:
        raise ProbeError("source_drift")
    record = {"status": "running", "version": item["version"],
              "archive_sha256": item["archive_sha256"], "binary_sha256": item["binary_sha256"],
              "source_sha256": source_hash, "mutations": []}
    try:
        run_command(tool, ["init", "-backend=false", "-get=false", "-input=false"], cwd, env, root, name + ".init", MAX_OTHER)
        installed = list((cwd / ".terraform" / "providers").rglob("terraform-provider-*"))
        binaries = [p for p in installed if p.is_file() and os.access(p, os.X_OK)]
        if len(binaries) != 1 or digest(binaries[0]) != item["binary_sha256"]:
            raise ProbeError("installed_binary_hash")
        record["installed_binary_sha256"] = digest(binaries[0])
        record["lockfile_sha256"] = digest(cwd / ".terraform.lock.hcl")
        if digest(tf) != source_hash:
            raise ProbeError("source_drift")
        schema = run_command(tool, ["providers", "schema", "-json"], cwd, env, root, name + ".schema", MAX_EXPORT)
        show = run_command(tool, ["show", "-json", "-config"], cwd, env, root, name + ".show", MAX_OTHER)
        record["schema"] = summarize_schema(schema)
        record["schema"]["raw_sha256"] = hashlib.sha256(schema).hexdigest()
        record["show"] = summarize_show(show, name)
        record["show"]["raw_sha256"] = hashlib.sha256(show).hexdigest()
        record["baseline_validation"] = validate(tool, cwd, env, root, name + ".baseline")
        for index in range(4):
            tf.write_text(mutate(name, index, source))
            row = {"output_index": index, "source_sha256": digest(tf)}
            record["mutations"].append(row)
            row["validation"] = validate(tool, cwd, env, root, name + ".mutation" + str(index))
            tf.write_text(source)
            if digest(tf) != source_hash:
                raise ProbeError("restore_drift")
        schema2 = run_command(tool, ["providers", "schema", "-json"], cwd, env, root, name + ".schema_repeat", MAX_EXPORT)
        show2 = run_command(tool, ["show", "-json", "-config"], cwd, env, root, name + ".show_repeat", MAX_OTHER)
        record["schema_repeat_sha256"] = hashlib.sha256(schema2).hexdigest()
        record["show_repeat_sha256"] = hashlib.sha256(show2).hexdigest()
        record["schema_conserved"] = schema == schema2
        record["show_conserved"] = show == show2
        if not record["baseline_validation"]["exit_zero"] or record["baseline_validation"]["valid"] is not True or record["baseline_validation"]["error_count"] != 0:
            raise ProbeError("baseline_invalid")
        if record["show"]["authored_outputs"] != 4 or record["show"]["authored_full_path_count"] != 4:
            raise ProbeError("native_references")
        if record["show"]["aliased_provider_config_count"] != 1 or record["show"]["aliased_resource_mapping_count"] != 1:
            raise ProbeError("alias_limit")
        for mutation in record["mutations"]:
            if not mutation_is_attributed(mutation["validation"]):
                raise ProbeError("mutation_unattributed")
        if not record["schema_conserved"] or not record["show_conserved"]:
            raise ProbeError("conservation")
        record["status"] = "complete"
    except (ProbeError, ValueError, OSError, KeyError, TypeError) as exc:
        record["status"] = "failed"
        record["failure"] = exc.code if isinstance(exc, ProbeError) else "native_or_parse"
    finally:
        tf.write_text(source)
    return record


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--inputs", type=Path, required=True)
    parser.add_argument("--scratch", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    LAUNCHES.clear()
    if os.path.lexists(args.out):
        print("stale_output", file=sys.stderr)
        return 1
    receipt = {"scope": "authored_provider_schema_feasibility_only", "status": "failed",
               "measured_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
               "probe_sha256": digest(__file__),
               "pins_sha256": digest(Path(__file__).with_name("pins.json")),
               "protocol_sha256": digest(Path(__file__).with_name("protocol.json")),
               "platform": PLATFORM, "network_denial_proven": False,
               "limits": {"command_seconds": TIMEOUT, "schema_bytes": MAX_EXPORT,
                          "other_bytes": MAX_OTHER, "nice_min": 10, "launch_load_max": 9},
               "store_bytes": "unknown", "providers": {}}
    try:
        root = args.scratch.resolve()
        if root.exists() and (not root.is_dir() or any(root.iterdir())):
            raise ProbeError("stale_scratch")
        root.mkdir(parents=True, exist_ok=True)
        manifest = json.loads(args.inputs.read_text())
        pins_path = Path(__file__).with_name("pins.json")
        baseline_path = Path(__file__).resolve().parents[1] / "terraform" / "pins.json"
        pins = json.loads(pins_path.read_text())
        baseline_pins = json.loads(baseline_path.read_text())
        providers = check_manifest(manifest, pins, baseline_pins)
        receipt["pins_sha256"] = digest(pins_path)
        frozen = {name: hashlib.sha256(fixture(name, providers[name]["version"]).encode()).hexdigest()
                  for name in NAMES}
        protocol = json.loads(Path(__file__).with_name("protocol.json").read_text())
        if frozen != protocol["fixture_sha256"]:
            raise ProbeError("fixture_pin")
        for name in NAMES:
            receipt["providers"][name] = {"status": "pending", "version": providers[name]["version"],
                                          "source_sha256": frozen[name]}
        mirror = root / "mirror" / "registry.opentofu.org" / "hashicorp"
        for name in NAMES:
            item = providers[name]
            dest = mirror / name
            dest.mkdir(parents=True)
            shutil.copyfile(item["archive"], dest / ("terraform-provider-%s_%s_%s.zip" % (name, item["version"], PLATFORM)))
        config = root / "tofurc"
        config.write_text('provider_installation {\n filesystem_mirror {\n path = %s\n include = ["*/*/*"]\n }\n}\n' % json.dumps(str(root / "mirror")))
        env = private_env(root, config)
        prove_network_deny(root, env)
        receipt["network_denial_proven"] = True
        tool_version = run_command(manifest["tool"]["path"], ["version", "-json"], root, env, root, "tool.version", MAX_OTHER)
        if json.loads(tool_version).get("terraform_version") != pins["tool"]["version"]:
            raise ProbeError("tool_version")
        receipt["tool"] = {"sha256": manifest["tool"]["sha256"], "version": pins["tool"]["version"]}
        for name in NAMES:
            receipt["providers"][name] = provider_run(name, providers[name], manifest["tool"]["path"], root, env, frozen[name])
        if all(p["status"] == "complete" for p in receipt["providers"].values()):
            receipt["status"] = "complete"
    except (ProbeError, ValueError, OSError, KeyError, TypeError) as exc:
        receipt["failure"] = exc.code if isinstance(exc, ProbeError) else "input_or_io"
    receipt["launches"] = LAUNCHES.copy()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_bytes(json_bytes(receipt) + b"\n")
    return 0 if receipt["status"] == "complete" else 1


if __name__ == "__main__":
    sys.exit(main())
