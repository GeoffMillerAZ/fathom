"""Measure native export size and an authored JSON serialization cost proxy.

This selector is custom calibration code, not a binding oracle, store or product
adapter. It reports only hashes, counts and byte sizes; schema text stays local.
"""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

SPEC = importlib.util.spec_from_file_location("provider_schema_probe", Path(__file__).with_name("probe.py"))
probe = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    args.out.unlink(missing_ok=True)
    result = {"scope": "authored attribute-object serialization proxy; not store bytes or binding accuracy",
              "actual_store_bytes": "not measured; SG-37 absent", "providers": {}}
    for name in probe.NAMES:
        export_path = args.capture / (name + ".schema.stdout")
        if export_path.stat().st_size > probe.MAX_EXPORT:
            raise ValueError("export byte cap")
        raw = export_path.read_bytes()
        data = json.loads(raw)
        show = json.loads((args.capture / (name + ".show.stdout")).read_bytes())
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
    args.out.write_bytes(probe.json_bytes(result) + b"\n")


if __name__ == "__main__":
    main()
