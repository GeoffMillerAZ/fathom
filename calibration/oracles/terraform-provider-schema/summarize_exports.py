"""Measure native export size and an authored JSON serialization cost proxy.

This selector is custom calibration code, not a binding oracle, store or product
adapter. It reports only hashes, counts and byte sizes; schema text stays local.
"""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path

SPEC = importlib.util.spec_from_file_location("provider_schema_probe", Path(__file__).with_name("probe.py"))
probe = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe)


def bound_captures(capture, receipt_path):
    """Validate the complete native capture set before selecting any attributes."""
    receipt_bytes = receipt_path.read_bytes()
    receipt = json.loads(receipt_bytes)
    if (not isinstance(receipt, dict) or receipt.get("status") != "complete" or
            not isinstance(receipt.get("providers"), dict) or
            set(receipt["providers"]) != set(probe.NAMES) or
            not isinstance(receipt.get("launches"), list)):
        raise ValueError("receipt_incomplete")
    launches = {}
    for launch in receipt["launches"]:
        if not isinstance(launch, dict) or not isinstance(launch.get("command"), str):
            raise ValueError("receipt_shape")
        command = launch["command"]
        if command in launches:
            raise ValueError("receipt_duplicate_launch")
        launches[command] = launch
    raw = {}
    for name in probe.NAMES:
        provider = receipt["providers"][name]
        if not isinstance(provider, dict) or provider.get("status") != "complete":
            raise ValueError("receipt_incomplete")
        raw[name] = {}
        for kind, cap in (("schema", probe.MAX_EXPORT), ("show", probe.MAX_OTHER)):
            path = capture / (name + "." + kind + ".stdout")
            if path.stat().st_size > cap:
                raise ValueError("export byte cap")
            contents = path.read_bytes()
            digest = hashlib.sha256(contents).hexdigest()
            declared = provider.get(kind)
            launch = launches.get(name + "." + kind)
            if (not isinstance(declared, dict) or not isinstance(launch, dict) or
                    launch.get("status") != "exited" or launch.get("exit_code") != 0 or
                    declared.get("raw_sha256") != digest or
                    launch.get("stdout_sha256") != digest or
                    launch.get("stdout_bytes") != len(contents)):
                raise ValueError("capture_receipt_mismatch")
            raw[name][kind] = contents
    return raw, hashlib.sha256(receipt_bytes).hexdigest()


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture", type=Path, required=True)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args(argv)
    if os.path.lexists(args.out):
        raise FileExistsError("stale_output")
    captures, receipt_sha256 = bound_captures(args.capture, args.receipt)
    result = {"scope": "authored attribute-object serialization proxy; not store bytes or binding accuracy",
              "actual_store_bytes": "not measured; SG-37 absent",
              "native_receipt_sha256": receipt_sha256, "providers": {}}
    for name in probe.NAMES:
        raw = captures[name]["schema"]
        data = json.loads(raw)
        show = json.loads(captures[name]["show"])
        resources = {r["address"]: r for r in show["root_module"]["resources"]}
        selected = {}
        for expr in probe.FIXTURES[name]["expressions"]:
            parts = expr.split(".")
            is_data = parts[0] == "data"
            stop = 3 if is_data else 2
            address = ".".join(parts[:stop])
            field = parts[stop].split("[")[0]
            declaration = resources[address]
            source = show["provider_config"][declaration["provider_config_key"]]["full_name"]
            namespace = "data_source_schemas" if is_data else "resource_schemas"
            schema = data["provider_schemas"][source][namespace][declaration["type"]]
            key = (source, namespace, declaration["type"], field)
            selected[key] = {"source": source, "namespace": namespace,
                             "type": declaration["type"], "field": field,
                             "schema_version": schema.get("version"),
                             "attribute": schema["block"]["attributes"][field]}
        serialized = probe.json_bytes([selected[k] for k in sorted(selected)])
        result["providers"][name] = {
            "export_sha256": hashlib.sha256(raw).hexdigest(),
            "export_bytes": len(raw), "format_version": data.get("format_version"),
            "show_sha256": hashlib.sha256(captures[name]["show"]).hexdigest(),
            "show_bytes": len(captures[name]["show"]),
            "authored_references": len(probe.FIXTURES[name]["expressions"]),
            "selected_unique_attribute_objects": len(selected),
            "selected_resource_or_data_types": len({k[:3] for k in selected}),
            "selected_attribute_json_bytes": len(serialized),
            "selected_attribute_json_sha256": hashlib.sha256(serialized).hexdigest(),
            "exceeds_planned_2MiB_file_cap": len(raw) > 2 * 1024 * 1024,
            "exceeds_planned_8MiB_package_cap": len(raw) > 8 * 1024 * 1024,
            "locator": "single-line export; usable declaration anchors not measured or admitted",
        }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("xb") as stream:
        stream.write(probe.json_bytes(result) + b"\n")


if __name__ == "__main__":
    main()
